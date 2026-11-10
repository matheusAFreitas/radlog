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
	Status  string      `json:"status"` // sem, pouco, pista, relacao
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

// pressVerdict escolhe o atraso de maior |r| e escreve a frase.
func pressVerdict(press, bg map[int64]float64) PressRadon {
	out := PressRadon{}
	if len(press) == 0 {
		out.Status = "sem"
		out.Title = "Ainda não há pressão guardada."
		out.Text = "Ela vem do mesmo serviço da chuva e chega na próxima busca."
		return out
	}
	best := 0
	bestR, bestN, bestD := pressCorr(press, bg, 0)
	for lag := 1; lag <= pressMaxLag; lag++ {
		r, n, d := pressCorr(press, bg, lag)
		if n >= pressMinPairs && (bestN < pressMinPairs || math.Abs(r) > math.Abs(bestR)) {
			best, bestR, bestN, bestD = lag, r, n, d
		}
	}
	out.Lag, out.R, out.Pairs, out.Days = best, bestR, bestN, bestD
	if bestN < pressMinPairs {
		out.Status = "pouco"
		out.Title = "Ainda não dá para comparar."
		out.Text = fmt.Sprintf("Precisa de pelo menos 3 dias com a pressão e o fundo em casa juntos. Até agora são %d horas.", bestN)
		return out
	}
	trust := bestD >= pressTrustDays
	out.Status = "pista"
	if trust {
		out.Status = "relacao"
	}
	rTxt := fmt.Sprintf("r = %s em %d dias.", pressNum(bestR, 2), bestD)
	tail := "Com menos de 28 dias isso ainda é uma pista."
	if trust {
		tail = "Com 28 dias ou mais, a relação já é confiável."
	}
	switch {
	case math.Abs(bestR) < pressWeakR:
		out.Title = "Não deu para ver relação com a pressão."
		out.Text = rTxt + " A pressão variou pouco, ou o radônio da sua casa depende mais de outra coisa (ventilação, horário)."
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
		out.Text = rTxt + " O sinal negativo quer dizer que um sobe quando o outro desce. " + tail
	default:
		pre := "Pista: "
		if trust {
			pre = "Relação: "
		}
		out.Title = pre + "aqui o fundo sobe junto com a pressão, ao contrário do comum."
		out.Text = rTxt + " Pode ser outra coisa que muda junto, como a casa fechada à noite. " + tail
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
