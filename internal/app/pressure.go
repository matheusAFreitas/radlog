package app

import (
	"fmt"
	"math"
	"sort"
	"strings"
	"time"

	"radlog/internal/core/stats"
	"radlog/internal/store"
)

// ---------- pressão × radônio (2.44) ----------
//
// O radônio sai do solo e das paredes com mais facilidade quando a pressão
// do ar cai, e os "filhos" dele (Bi-214, Pb-214) emitem gama: o fundo em casa
// sobe. A pressão vem do mesmo serviço da chuva (Open-Meteo, sem chave) e
// fica um ano em chuva.json. A conta: mediana de 1 h do fundo em casa (fora
// de alertas) e correlação com a pressão de 0 a 12 h antes. Abaixo de 28 dias
// é "pista"; dali para cima, "relação". O lugar nunca sai daqui: a resposta
// não traz cidade nem coordenada.

const (
	pressKeepDays   = 365
	pressShowDays   = 7  // o gráfico mostra a última semana
	pressCorrDays   = 90 // a conta usa até 90 dias
	pressMaxLag     = 12 // horas
	pressMinPairs   = 72 // 3 dias de horas com os dois
	pressTrustDays  = 28
	pressMinHourMin = 30 // minutos com dado para a hora valer
	pressWeakR      = 0.2
)

// PressHour é uma hora da última semana: pressão e fundo em casa.
type PressHour struct {
	T   int64    `json:"t"` // ms, início da hora
	HPa *float64 `json:"hPa"`
	USv *float64 `json:"usv"` // mediana de 1 h em casa (µSv/h)
}

// PressRadon é a seção "Pressão × radônio" da tela Clima.
type PressRadon struct {
	Enabled bool        `json:"enabled"`
	Device  string      `json:"device"`
	Hours   []PressHour `json:"hours"`
	Lag     int         `json:"lag"`    // horas (pressão antes do fundo)
	R       float64     `json:"r"`      // correlação no atraso escolhido
	Days    int         `json:"days"`   // dias que entraram na conta
	Pairs   int         `json:"pairs"`  // horas com os dois
	Status  string      `json:"status"` // sem, pouco, fraca, acaso, pista, relacao
	Chance  float64     `json:"chance"` // teste de acaso: fração dos deslocamentos com |r| desse tamanho
	Title   string      `json:"title"`
	Text    string      `json:"text"`
}

// pressNum escreve um número com vírgula (o i18n troca no inglês).
func pressNum(x float64, d int) string {
	s := fmt.Sprintf("%.*f", d, x)
	if strings.HasPrefix(s, "-") {
		s = "−" + s[1:]
	}
	return strings.Replace(s, ".", ",", 1)
}

// pressCorr faz a correlação de Pearson entre a pressão lag horas antes e o
// fundo, só nas horas em que existem os dois.
func pressCorr(press, bg map[int64]float64, lag int) (r float64, pairs int, days int) {
	var xs, ys []float64
	dayset := map[string]bool{}
	for t, y := range bg {
		x, ok := press[t-int64(lag)*3600]
		if !ok {
			continue
		}
		xs = append(xs, x)
		ys = append(ys, y)
		dayset[time.Unix(t, 0).Format("2006-01-02")] = true
	}
	n := len(xs)
	if n < 3 {
		return 0, n, len(dayset)
	}
	var mx, my float64
	for i := range xs {
		mx += xs[i]
		my += ys[i]
	}
	mx /= float64(n)
	my /= float64(n)
	var sxy, sxx, syy float64
	for i := range xs {
		sxy += (xs[i] - mx) * (ys[i] - my)
		sxx += (xs[i] - mx) * (xs[i] - mx)
		syy += (ys[i] - my) * (ys[i] - my)
	}
	if sxx == 0 || syy == 0 {
		return 0, n, len(dayset)
	}
	return sxy / math.Sqrt(sxx*syy), n, len(dayset)
}

// deDaily tira de uma série horária a média de cada hora do dia (hora
// local). Pressão e fundo têm ciclo diário próprio (a maré da pressão, duas
// vezes por dia; a casa fechada à noite), e dois ciclos diários se parecem
// sem um causar o outro. Sem o ciclo, sobra a variação de um dia para outro,
// que é a que mexe com o radônio (2.46).
func deDaily(m map[int64]float64) map[int64]float64 {
	var sum [24]float64
	var n [24]int
	for t, v := range m {
		h := time.Unix(t, 0).Hour()
		sum[h] += v
		n[h]++
	}
	out := make(map[int64]float64, len(m))
	for t, v := range m {
		h := time.Unix(t, 0).Hour()
		out[t] = v - sum[h]/float64(n[h])
	}
	return out
}

// bestLag acha o atraso (0 a pressMaxLag h) de maior |r| com pares bastantes.
func bestLag(press, bg map[int64]float64) (lag int, r float64, pairs, days int) {
	r, pairs, days = pressCorr(press, bg, 0)
	for l := 1; l <= pressMaxLag; l++ {
		rr, n, d := pressCorr(press, bg, l)
		if n >= pressMinPairs && (pairs < pressMinPairs || math.Abs(rr) > math.Abs(r)) {
			lag, r, pairs, days = l, rr, n, d
		}
	}
	return
}

const (
	pressShifts   = 99 // deslocamentos no teste de acaso
	pressMinShift = 48 // horas: deslocamento mínimo (mais que um ciclo do tempo)
	pressAlpha    = 0.05
)

// pressChance é o teste de acaso: desloca o fundo no tempo (dando a volta
// no fim), o que mantém o jeito de cada série mas desfaz qualquer ligação
// com a pressão, e conta em quantos deslocamentos o melhor |r| de 0 a 12 h
// ficou do tamanho do encontrado. Devolve a fração (0 a 1); 1 se não há
// dias bastantes para deslocar.
func pressChance(press, bg map[int64]float64, absR float64) float64 {
	if len(bg) == 0 {
		return 1
	}
	lo, hi := int64(math.MaxInt64), int64(math.MinInt64)
	for t := range bg {
		lo, hi = min(lo, t), max(hi, t)
	}
	span := (hi-lo)/3600 + 1 // horas
	if span < 2*pressMinShift+24 {
		return 1
	}
	hits := 0
	for k := 1; k <= pressShifts; k++ {
		s := pressMinShift + (span-2*pressMinShift)*int64(k)/(pressShifts+1)
		moved := make(map[int64]float64, len(bg))
		for t, v := range bg {
			h := ((t-lo)/3600 + s) % span
			moved[lo+h*3600] = v
		}
		if _, r, n, _ := bestLag(press, moved); n >= pressMinPairs && math.Abs(r) >= absR {
			hits++
		}
	}
	return float64(hits+1) / float64(pressShifts+1)
}

// pressVerdict tira o ciclo do dia, escolhe o atraso de maior |r|, faz o
// teste de acaso e escreve a frase.
func pressVerdict(press, bg map[int64]float64) PressRadon {
	out := PressRadon{}
	if len(press) == 0 {
		out.Status = "sem"
		out.Title = "Ainda não há pressão guardada."
		out.Text = "Ela vem do mesmo serviço da chuva e chega na próxima busca."
		return out
	}
	pd, bd := deDaily(press), deDaily(bg)
	best, bestR, bestN, bestD := bestLag(pd, bd)
	out.Lag, out.R, out.Pairs, out.Days = best, bestR, bestN, bestD
	if bestN < pressMinPairs {
		out.Status = "pouco"
		out.Title = "Ainda não dá para comparar."
		out.Text = fmt.Sprintf("Precisa de pelo menos 3 dias com a pressão e o fundo em casa juntos. Até agora são %d horas.", bestN)
		return out
	}
	out.Chance = 1.0
	if math.Abs(bestR) >= pressWeakR {
		out.Chance = pressChance(pd, bd, math.Abs(bestR))
	}
	trust := bestD >= pressTrustDays
	out.Status = "pista"
	if trust {
		out.Status = "relacao"
	}
	rTxt := fmt.Sprintf("r = %s em %d dias, sem o ciclo do dia.", pressNum(bestR, 2), bestD)
	chTxt := fmt.Sprintf("Deslocando o fundo no tempo, uma relação desse tamanho apareceu por acaso em %s%% das vezes.", pressNum(out.Chance*100, 0))
	tail := "Com menos de 28 dias isso ainda é uma pista."
	if trust {
		tail = "Com 28 dias ou mais e passando no teste de acaso, a relação é confiável."
	}
	switch {
	case math.Abs(bestR) < pressWeakR:
		out.Status = "fraca"
		out.Title = "Não deu para ver relação com a pressão."
		out.Text = rTxt + " A pressão variou pouco, ou o radônio da sua casa depende mais de outra coisa (ventilação, horário)."
	case out.Chance >= pressAlpha:
		out.Status = "acaso"
		out.Title = "Não deu para ver relação com a pressão."
		out.Text = rTxt + " " + chTxt + " Então pode ser só coincidência. Com mais dias a resposta fica mais clara."
	case bestR < 0:
		pre := "Pista: "
		if trust {
			pre = "Relação: "
		}
		if best == 0 {
			out.Title = pre + "quando a pressão cai, o fundo sobe quase ao mesmo tempo."
		} else {
			out.Title = pre + fmt.Sprintf("quando a pressão cai, o fundo sobe cerca de %d h depois.", best)
		}
		out.Text = rTxt + " O sinal negativo quer dizer que um sobe quando o outro desce. " + chTxt + " " + tail
	default:
		pre := "Pista: "
		if trust {
			pre = "Relação: "
		}
		out.Title = pre + "aqui o fundo sobe junto com a pressão, ao contrário do comum."
		out.Text = rTxt + " Pode ser outra coisa que muda junto, como a casa fechada à noite. " + chTxt + " " + tail
	}
	return out
}

// hourMedians agrupa minutos (T, valor) em medianas de 1 h, com o mínimo de
// minutos por hora.
func hourMedians(pts []store.Point) map[int64]float64 {
	by := map[int64][]float64{}
	for _, p := range pts {
		h := p.T.Truncate(time.Hour).Unix()
		by[h] = append(by[h], p.CPM)
	}
	out := map[int64]float64{}
	for h, v := range by {
		if len(v) >= pressMinHourMin {
			out[h] = stats.Median(v)
		}
	}
	return out
}

// GetPressureRadon calcula a seção "Pressão × radônio" com o aparelho
// escolhido (gmc ou radiacode), sempre só em casa e fora de alertas.
func (a *App) GetPressureRadon(dev string) (PressRadon, error) {
	a.mu.Lock()
	set := a.settings
	press := map[int64]float64{}
	if c := a.rain; c != nil && c.Lat == set.RainLat && c.Lon == set.RainLon {
		for k, v := range c.Press {
			press[k] = v
		}
	}
	tl := a.calTLLocked()
	a.mu.Unlock()
	dev = devName(dev)
	enabled := set.RainEnabled && !(set.RainLat == 0 && set.RainLon == 0)
	if !enabled {
		return PressRadon{Device: dev, Hours: []PressHour{}}, nil
	}
	now := time.Now()
	from := now.Add(-pressCorrDays * 24 * time.Hour).Truncate(time.Hour)
	inAlert := a.alertChecker()
	var pts []store.Point
	if dev == devRC {
		for _, p := range a.rcRange(from, now) {
			if p.N < 20 || inAlert(p.T) {
				continue
			}
			pts = append(pts, store.Point{T: p.T, CPM: p.USvh})
		}
		pts = a.rcHomeOnly(pts)
	} else {
		raw, err := a.points(from, now)
		if err != nil {
			return PressRadon{Device: dev, Hours: []PressHour{}}, err
		}
		for _, p := range a.homeOnly(raw) {
			if !inAlert(p.T) {
				pts = append(pts, p)
			}
		}
	}
	bg := hourMedians(pts) // GMC: CPM (a calibração não muda a correlação)
	out := pressVerdict(press, bg)
	out.Enabled, out.Device = true, dev
	// a última semana, hora a hora, em µSv/h
	start := now.Add(-pressShowDays * 24 * time.Hour).Truncate(time.Hour)
	out.Hours = []PressHour{}
	for t := start; !t.After(now); t = t.Add(time.Hour) {
		h := PressHour{T: t.UnixMilli()}
		if v, ok := press[t.Unix()]; ok {
			v := v
			h.HPa = &v
		}
		if v, ok := bg[t.Unix()]; ok {
			u := v
			if dev != devRC {
				u = tl.USvAt(t, v) // a calibração que valia naquela hora, como nos Gráficos
			}
			h.USv = &u
		}
		out.Hours = append(out.Hours, h)
	}
	sort.Slice(out.Hours, func(i, j int) bool { return out.Hours[i].T < out.Hours[j].T })
	return out, nil
}
