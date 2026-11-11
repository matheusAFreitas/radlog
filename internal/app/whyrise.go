package app

import (
	"fmt"
	"math"
	"sort"
	"strings"
	"time"

	"radlog/internal/core/stats"
)

// ---------- "Por que subiu?" (ideia 8, 2.55) ----------
//
// Quando a leitura sobe, o painel confere as causas comuns com os dados do
// próprio app: o acaso, o outro aparelho, chuva, radônio da noite, pressão,
// objeto ou anotação perto, saída de casa e, se nada apoia e só um aparelho
// subiu, o próprio aparelho. Não é diagnóstico. Calcula sob demanda, para um
// instante (agora ou a hora de um alerta). Spec: ux/porque.html.
//
// A conta é feita em contagens por minuto (GMC: CPM; RC102: cps × 60), que
// seguem Poisson: o "normal" é a mediana dos minutos em casa dos 30 dias
// antes; a subida vai do começo da subida (blocos de 10 min acima do normal,
// andando para trás a partir de t) até t, e o σ é √(contagens esperadas).

const (
	whyWin      = 12 * time.Hour
	whyBin      = 10 * time.Minute
	whyBgDays   = 30
	whyRainWin  = 4 * time.Hour
	whyAcasoZ   = 2.0
	whyPressHPa = 3.0
)

// WhyCause é uma linha do painel.
type WhyCause struct {
	Key   string `json:"key"`   // acaso, ambos, chuva, radonio, pressao, objeto, fora, aparelho
	State string `json:"state"` // yes, no, na
	Title string `json:"title"`
	Ev    string `json:"ev"`   // a evidência
	More  string `json:"more"` // por que isso faz subir
	Link  string `json:"link"` // tela: chuva, padroes, medir, fora, gmc, radiacode, graficos
	LinkT string `json:"linkText"`
}

// WhyBin é um bloco de 10 min do gráfico (µSv/h; nil = sem dado).
type WhyBin struct {
	T int64    `json:"t"`
	G *float64 `json:"g"`
	R *float64 `json:"r"`
}

// WhyMark é uma marca no gráfico.
type WhyMark struct {
	T    int64  `json:"t"`
	End  int64  `json:"end,omitempty"`
	Kind string `json:"kind"` // nota, objeto, fora
	Text string `json:"text"`
}

// WhyRise é a resposta de GetWhyRise.
type WhyRise struct {
	Device  string     `json:"device"`
	At      int64      `json:"at"`
	Now     bool       `json:"now"`
	From    int64      `json:"from"` // começo da subida
	Pct     float64    `json:"pct"`
	Sigma   float64    `json:"sigma"`
	Acaso   bool       `json:"acaso"`
	NoData  bool       `json:"noData"`
	Head    string     `json:"head"`
	Verdict string     `json:"verdict"`
	Causes  []WhyCause `json:"causes"`
	Bins    []WhyBin   `json:"bins"`
	GNorm   float64    `json:"gNorm"` // µSv/h normal de cada aparelho (0 = sem)
	RNorm   float64    `json:"rNorm"`
	Rain    [][2]int64 `json:"rain"`
	Marks   []WhyMark  `json:"marks"`
}

// whyDev é a série de um aparelho: contagens por minuto e µSv/h.
type whyDev struct {
	cpm, usv map[int64]float64 // minuto (ms) -> valor
	norm     float64           // contagens/min normais (mediana em casa, 30 dias)
	normUSv  float64
	hourMed  [24]float64 // mediana de cada hora do dia (contagens/min)
	hourN    [24]int
}

func (a *App) whySeries(dev string, from, t time.Time) whyDev {
	d := whyDev{cpm: map[int64]float64{}, usv: map[int64]float64{}}
	bgFrom := t.Add(-whyBgDays * 24 * time.Hour)
	var homeCPM, homeUSv []float64
	var byHour [24][]float64
	inAlert := a.alertChecker()
	add := func(tm time.Time, cpm, usv float64, home bool) {
		if !tm.Before(from) && tm.Before(t) {
			k := tm.Truncate(time.Minute).UnixMilli()
			d.cpm[k], d.usv[k] = cpm, usv
		}
		if home && tm.Before(from) && !inAlert(tm) {
			homeCPM = append(homeCPM, cpm)
			homeUSv = append(homeUSv, usv)
			h := tm.Hour()
			byHour[h] = append(byHour[h], cpm)
		}
	}
	if dev == devRC {
		away := a.rcAwayChecker()
		onObj := a.onObjectChecker(devRC)
		for _, p := range a.rcRange(bgFrom, t) {
			add(p.T, p.CPS*60, p.USvh, !away(p.T) && !onObj(p.T))
		}
	} else if a.store != nil {
		raw, err := a.points(bgFrom, t)
		if err == nil {
			a.mu.Lock()
			tl := a.calTLLocked()
			a.mu.Unlock()
			home := map[int64]bool{}
			for _, p := range bgClean(a.homeOnly(raw)) {
				home[p.T.UnixMilli()] = true
			}
			for _, p := range raw {
				add(p.T, p.CPM, tl.USv(p), home[p.T.UnixMilli()])
			}
		}
	}
	// GMC-800: contagens inteiras, mediana interpolada (a comum anda em
	// degraus de ~5% com ~20 CPM); RC102: taxa contínua, mediana comum
	med, medU := stats.Median, func(c, u []float64) float64 { return stats.Median(u) }
	if dev != devRC {
		med, medU = stats.MedianCounts, stats.MedianCountsScaled
	}
	if len(homeCPM) >= 60 {
		d.norm, d.normUSv = med(homeCPM), medU(homeCPM, homeUSv)
	}
	for h := range byHour {
		d.hourN[h] = len(byHour[h])
		if len(byHour[h]) > 0 {
			d.hourMed[h] = med(byHour[h])
		}
	}
	return d
}

// whyWindow soma as contagens e os minutos em [f, t).
func (d whyDev) window(f, t time.Time) (sum float64, n int) {
	for k, v := range d.cpm {
		if k >= f.UnixMilli() && k < t.UnixMilli() {
			sum += v
			n++
		}
	}
	return
}

// riseFrom anda para trás, de 10 em 10 min, enquanto o bloco está acima do
// normal (mais que 2 σ de um bloco). Sem bloco alto no fim: os últimos 30 min.
func (d whyDev) riseFrom(from, t time.Time) time.Time {
	if d.norm <= 0 {
		return t.Add(-30 * time.Minute)
	}
	thr := d.norm * (1 + math.Max(0.02, 2/math.Sqrt(d.norm*whyBin.Minutes())))
	start := t
	for b := t.Add(-whyBin); !b.Before(from); b = b.Add(-whyBin) {
		s, n := d.window(b, b.Add(whyBin))
		if n < 3 || s/float64(n) <= thr {
			break
		}
		start = b
	}
	if start.Equal(t) {
		return t.Add(-30 * time.Minute)
	}
	return start
}

// pct e z da janela contra o normal
func (d whyDev) rise(f, t time.Time) (pct, z float64, n int) {
	s, n := d.window(f, t)
	if n == 0 || d.norm <= 0 {
		return 0, 0, n
	}
	exp := d.norm * float64(n)
	return (s/exp - 1) * 100, (s - exp) / math.Sqrt(exp), n
}

func whyDevName(dev string) string {
	if dev == devRC {
		return "RC102"
	}
	return "GMC-800"
}

func whyNum(x float64, d int) string { return pressNum(x, d) }

func whyPct(x float64) string {
	x = math.Abs(x)
	if x >= 10 {
		return whyNum(x, 0)
	}
	return whyNum(x, 1)
}

// GetWhyRise monta o painel para o aparelho dev ("gmc" ou "radiacode") no
// instante atMs (0 = agora).
func (a *App) GetWhyRise(dev string, atMs int64) (WhyRise, error) {
	dev = devName(dev)
	now := time.Now()
	t := now
	if atMs > 0 && atMs < now.UnixMilli() {
		t = time.UnixMilli(atMs)
	}
	t = t.Truncate(time.Minute).Add(time.Minute)
	from := t.Add(-whyWin)
	out := WhyRise{Device: dev, At: t.UnixMilli(), Now: now.Sub(t) < 2*time.Minute, Causes: []WhyCause{}, Bins: []WhyBin{}, Rain: [][2]int64{}, Marks: []WhyMark{}}
	other := "gmc"
	if dev == "gmc" {
		other = devRC
	}
	me, ot := a.whySeries(dev, from, t), a.whySeries(other, from, t)
	g, r := me, ot
	if dev == devRC {
		g, r = ot, me
	}
	out.GNorm, out.RNorm = g.normUSv, r.normUSv
	// gráfico: blocos de 10 min
	for b := from; b.Before(t); b = b.Add(whyBin) {
		bin := WhyBin{T: b.UnixMilli()}
		for i, s := range []whyDev{g, r} {
			var sum float64
			n := 0
			for k := b.UnixMilli(); k < b.Add(whyBin).UnixMilli(); k += 60_000 {
				if v, ok := s.usv[k]; ok {
					sum += v
					n++
				}
			}
			if n > 0 {
				v := sum / float64(n)
				if i == 0 {
					bin.G = &v
				} else {
					bin.R = &v
				}
			}
		}
		out.Bins = append(out.Bins, bin)
	}
	dn := whyDevName(dev)
	if len(me.cpm) == 0 || me.norm <= 0 {
		out.NoData = true
		out.Head = fmt.Sprintf("Sem dados do %s nas 12 horas antes", dn)
		if me.norm <= 0 && len(me.cpm) > 0 {
			out.Head = fmt.Sprintf("Ainda não dá para saber o normal do %s", dn)
		}
		out.Verdict = "O painel precisa das leituras do aparelho nas horas antes e de alguns dias de fundo em casa para comparar."
		a.whyMarks(&out, from, t)
		return out, nil
	}
	rf := me.riseFrom(from, t)
	pct, z, _ := me.rise(rf, t)
	out.From, out.Pct, out.Sigma = rf.UnixMilli(), pct, z
	out.Acaso = z < whyAcasoZ
	hh := rf.Format("15:04")
	switch {
	case pct <= 0:
		out.Head = fmt.Sprintf("O %s está %s%% abaixo do normal desde %s", map[bool]string{true: "fundo", false: dn}[dev == "gmc"], whyPct(pct), hh)
	case dev == "gmc":
		out.Head = fmt.Sprintf("O fundo subiu %s%% desde %s", whyPct(pct), hh)
	default:
		out.Head = fmt.Sprintf("O RC102 subiu %s%% desde %s", whyPct(pct), hh)
	}
	a.whyMarks(&out, from, t)

	var C []WhyCause
	// acaso
	acaso := WhyCause{Key: "acaso", Title: "Acaso", More: "O decaimento é aleatório, então a contagem varia sozinha. Até uns 2 σ isso é normal e acontece toda hora.", Link: "padroes", LinkT: "Abrir Padrões do fundo"}
	if out.Acaso {
		acaso.State = "yes"
		acaso.Ev = fmt.Sprintf("A subida é de %s σ. Variações assim aparecem várias vezes por dia.", whyNum(math.Max(z, 0), 1))
	} else {
		acaso.State = "no"
		acaso.Ev = fmt.Sprintf("A subida é de %s σ. O acaso não chega nisso.", whyNum(z, 0))
		acaso.Link, acaso.LinkT = "", ""
	}
	C = append(C, acaso)
	// os dois aparelhos
	on := whyDevName(other)
	both := WhyCause{Key: "ambos", Title: "Os dois aparelhos viram", More: "Se fosse o ambiente, os dois subiriam juntos. É o que separa radiação de verdade de coisa de um aparelho só."}
	opct, oz, on2 := ot.rise(rf, t)
	switch {
	case on2 < 5 || ot.norm <= 0:
		both.State, both.Ev = "na", fmt.Sprintf("Sem dado: o %s não tem leituras nessa hora.", on)
	case oz >= whyAcasoZ && pct > 0 && opct >= pct*0.4:
		both.State, both.Ev = "yes", fmt.Sprintf("O %s também subiu, %s%% no mesmo trecho.", on, whyPct(opct))
	default:
		both.State = "no"
		both.Ev = fmt.Sprintf("O %s ficou parado (%s%%, %s σ).", on, whyPct(opct), whyNum(oz, 1))
		if opct >= 0 {
			both.Ev = fmt.Sprintf("O %s ficou parado (+%s%%, %s σ).", on, whyPct(opct), whyNum(oz, 1))
		} else {
			both.Ev = fmt.Sprintf("O %s ficou parado (−%s%%, %s σ).", on, whyPct(opct), whyNum(math.Abs(oz), 1))
		}
	}
	// num alerta, vale a confirmação cruzada dele (a mesma da faixa e da lista)
	if f, to, ok := a.alertSpanAt(dev, t); ok {
		cc := a.GetCrossCheck(dev, f, to, false)
		switch cc.State {
		case "ok":
			both.State, both.Ev = "yes", cc.Text
		case "no":
			both.State, both.Ev = "no", cc.Text
		default:
			both.State, both.Ev = "na", cc.Text
		}
	}
	C = append(C, both)
	// fora de casa (o aparelho da subida)
	awayAt := a.awayCheckerFor(dev)
	fora := WhyCause{Key: "fora", Title: "Saída de casa", More: "Fora de casa o fundo muda: granito, prédios, solo, altitude e avião sobem a dose. Isso entra na Minha dose, não no fundo de casa.", Link: "fora", LinkT: "Abrir Fora de casa"}
	var awaySince time.Time
	for m := rf; m.Before(t); m = m.Add(5 * time.Minute) {
		if awayAt(m) {
			awaySince = m
			break
		}
	}
	if !awaySince.IsZero() {
		a.mu.Lock()
		for _, p := range a.periods {
			if p.From <= awaySince.UnixMilli() && awaySince.UnixMilli() < periodEnd(p) {
				awaySince = time.UnixMilli(p.From)
			}
		}
		a.mu.Unlock()
		fora.State, fora.Ev = "yes", fmt.Sprintf("O %s está fora de casa desde %s.", dn, whyClock(awaySince, t))
	} else {
		fora.State, fora.Ev = "no", fmt.Sprintf("O %s estava em casa.", dn)
	}
	// chuva
	chuva := WhyCause{Key: "chuva", Title: "Chuva", More: "A chuva traz para o chão os produtos do radônio que estavam no ar. Eles têm meia-vida curta (minutos), então a subida dura poucas horas e some sozinha.", Link: "chuva", LinkT: "Abrir a tela Clima"}
	a.mu.Lock()
	set := a.settings
	rainOn := set.RainEnabled && !(set.RainLat == 0 && set.RainLon == 0)
	var press map[int64]float64
	if c := a.rain; c != nil && c.Lat == set.RainLat && c.Lon == set.RainLon {
		press = map[int64]float64{}
		for k, v := range c.Press {
			press[k] = v
		}
	}
	a.mu.Unlock()
	if !rainOn {
		chuva.State, chuva.Ev = "na", "Sem dado: a chuva não está ligada (Configurações › Janela e tela)."
	} else {
		slots := a.rainSlots(t.Add(-whyRainWin), t)
		if len(slots) == 0 {
			chuva.State, chuva.Ev = "na", "Sem dado de chuva nessa hora."
		} else {
			mm := 0.0
			var first time.Time
			for k, s := range slots {
				if s.wet() {
					mm += s.mmh * rainSlot.Hours()
					if tk := time.Unix(k, 0); first.IsZero() || tk.Before(first) {
						first = tk
					}
				}
			}
			if mm >= 0.5 {
				chuva.State = "yes"
				chuva.Ev = fmt.Sprintf("Choveu %s mm nas 4 horas antes, a partir de %s.", whyNum(mm, 1), whyClock(first, t))
				if ri, err := a.GetRainImpactDev(dev); err == nil && ri.N >= 3 && ri.AvgBase > 0 && ri.Effect > 0 {
					chuva.Ev += fmt.Sprintf(" Na sua casa, a chuva costuma subir o fundo uns %s%%.", whyPct(ri.Effect/ri.AvgBase*100))
				}
			} else {
				chuva.State, chuva.Ev = "no", "Não choveu nas 4 horas antes."
			}
		}
	}
	// radônio da noite (hora do dia)
	rad := WhyCause{Key: "radonio", Title: "Radônio da noite", More: "Com a casa fechada e o ar parado, o radônio que sai do chão e das paredes se acumula. De dia, com janela aberta e vento, ele se espalha.", Link: "padroes", LinkT: "Abrir Padrões do fundo"}
	h := t.Add(-time.Minute).Hour()
	if me.hourN[h] < 300 || me.norm <= 0 {
		rad.State, rad.Ev = "na", "Sem dado: ainda são poucos dias para saber o padrão de cada hora."
	} else {
		hp := (me.hourMed[h]/me.norm - 1) * 100
		switch {
		case !awaySince.IsZero():
			rad.State, rad.Ev = "no", fmt.Sprintf("O %s estava fora de casa.", dn)
		case hp >= 3 && pct > 0 && hp >= pct*0.4:
			rad.State, rad.Ev = "yes", fmt.Sprintf("Às %dh o seu fundo costuma ficar %s%% acima da média do dia.", h, whyPct(hp))
		case hp >= 1:
			rad.State, rad.Ev = "no", fmt.Sprintf("Às %dh o seu fundo costuma ficar só %s%% acima da média do dia.", h, whyPct(hp))
		default:
			rad.State, rad.Ev = "no", fmt.Sprintf("Às %dh o seu fundo costuma ficar na média do dia.", h)
		}
	}
	// pressão caindo
	pr := WhyCause{Key: "pressao", Title: "Pressão caindo", More: "Quando a pressão do ar cai, sai mais radônio do solo e das paredes, e o fundo em casa sobe algumas horas depois.", Link: "chuva", LinkT: "Abrir a tela Clima"}
	if !rainOn || len(press) == 0 {
		pr.State, pr.Ev = "na", "Sem dado de pressão (vem com a chuva, em Configurações › Janela e tela)."
	} else {
		var last, first float64
		lastK, firstK := int64(0), int64(math.MaxInt64)
		hi := math.Inf(-1)
		for k, v := range press {
			tk := time.Unix(k, 0)
			if tk.After(t) || tk.Before(t.Add(-whyWin)) {
				continue
			}
			if k > lastK {
				lastK, last = k, v
			}
			if k < firstK {
				firstK, first = k, v
			}
			hi = math.Max(hi, v)
		}
		switch {
		case lastK == 0:
			pr.State, pr.Ev = "na", "Sem dado de pressão nessa hora."
		case hi-last >= whyPressHPa:
			pr.State, pr.Ev = "yes", fmt.Sprintf("A pressão caiu %s hPa nas 12 horas antes.", whyNum(hi-last, 1))
		default:
			// a variação de verdade na janela, com o sentido
			pr.State = "no"
			switch d := last - first; {
			case math.Abs(d) < 0.05:
				pr.Ev = "A pressão ficou estável nas 12 horas antes."
			case d > 0:
				pr.Ev = fmt.Sprintf("A pressão subiu %s hPa nas 12 horas antes.", whyNum(d, 1))
			default:
				pr.Ev = fmt.Sprintf("A pressão caiu %s hPa nas 12 horas antes.", whyNum(-d, 1))
			}
		}
	}
	// objeto ou anotação perto
	obj := WhyCause{Key: "objeto", Title: "Objeto ou fonte perto", More: "Um objeto perto do aparelho sobe muito a leitura dele, e quase nada a do outro aparelho. Para medir direito, use o Medir objeto.", Link: "medir", LinkT: "Abrir Medir objeto"}
	near := rf.Add(-time.Hour)
	var objTxt string
	for _, m := range out.Marks {
		if m.Kind == "fora" {
			continue
		}
		end := m.End
		if end == 0 {
			end = m.T
		}
		if end >= near.UnixMilli() && m.T <= t.UnixMilli() {
			if m.Kind == "nota" {
				objTxt = fmt.Sprintf("Anotação às %s: \"%s\".", time.UnixMilli(m.T).Format("15:04"), m.Text)
			} else if objTxt == "" {
				objTxt = fmt.Sprintf("Medição de objeto às %s.", time.UnixMilli(m.T).Format("15:04"))
			}
		}
	}
	if objTxt != "" {
		obj.State, obj.Ev = "yes", objTxt
	} else {
		obj.State, obj.Ev = "no", fmt.Sprintf("Nenhuma medição de objeto nem anotação desde %s.", near.Format("15:04"))
		obj.Link, obj.LinkT = "", ""
	}
	// coisa do aparelho
	apar := WhyCause{Key: "aparelho", Title: "Coisa do aparelho", More: fmt.Sprintf("Confira se o %s foi mexido ou ficou perto de alguma coisa nova, e o cabo. Ruído no tubo também faz a contagem pular.", dn), Link: dev, LinkT: fmt.Sprintf("Abrir a tela do %s", dn)}
	explained := chuva.State == "yes" || rad.State == "yes" || pr.State == "yes" || obj.State == "yes" || fora.State == "yes"
	switch {
	case out.Acaso:
		apar.State, apar.Ev = "no", "A subida é do tamanho do acaso."
	case both.State == "yes":
		apar.State, apar.Ev = "no", "Os dois aparelhos subiram juntos."
	case both.State == "na":
		apar.State, apar.Ev = "na", fmt.Sprintf("Sem o %s para comparar.", on)
	case explained:
		apar.State, apar.Ev = "no", "Outra causa explica a subida."
	default:
		apar.State, apar.Ev = "yes", fmt.Sprintf("Só o %s subiu, sem chuva, sem nota e fora da hora do radônio.", dn)
	}
	C = append(C, chuva, rad, pr, obj)
	if dev == devRC || fora.State == "yes" {
		C = append(C, fora)
	}
	C = append(C, apar)
	rank := map[string]int{"yes": 0, "no": 1, "na": 2}
	sort.SliceStable(C, func(i, j int) bool { return rank[C[i].State] < rank[C[j].State] })
	out.Causes = C

	// veredito em frases curtas
	var v []string
	if out.Acaso {
		v = append(v, "Provavelmente é só o acaso.", fmt.Sprintf("A subida é de %s σ, o tipo de variação que acontece várias vezes por dia.", whyNum(math.Max(z, 0), 1)))
	} else {
		if obj.State == "yes" {
			v = append(v, obj.Ev)
		}
		if fora.State == "yes" {
			v = append(v, fmt.Sprintf("O %s saiu de casa às %s e subiu fora.", dn, whyClock(awaySince, t)))
		}
		if chuva.State == "yes" {
			v = append(v, strings.SplitN(chuva.Ev, " Na sua casa", 2)[0])
		}
		if rad.State == "yes" {
			v = append(v, "Esta é a hora em que o radônio costuma subir na sua casa.")
		}
		if pr.State == "yes" {
			v = append(v, pr.Ev)
		}
		switch {
		case both.State == "yes":
			if len(v) == 0 {
				v = append(v, "Nenhuma das causas comuns apoia.")
			}
			v = append(v, "Os dois aparelhos subiram juntos, então é radiação de verdade.")
		case both.State == "no" && (obj.State == "yes" || fora.State == "yes"):
			v = append(v, fmt.Sprintf("Só o %s subiu: é o que estava perto dele, não a casa.", dn))
		case both.State == "no" && !explained:
			v = append(v, fmt.Sprintf("Só o %s subiu. Pode ser coisa do aparelho.", dn))
		case both.State == "no":
			v = append(v, fmt.Sprintf("O %s não subiu junto.", on))
		default:
			if len(v) == 0 {
				v = append(v, "Nenhuma das causas comuns apoia.")
			}
			v = append(v, fmt.Sprintf("Sem o %s para comparar.", on))
		}
	}
	out.Verdict = strings.Join(v, " ")
	return out, nil
}

// whyClock é "HH:MM", com a data quando não é o mesmo dia de t.
func whyClock(x, t time.Time) string {
	if x.Format("2006-01-02") == t.Format("2006-01-02") {
		return x.Format("15:04")
	}
	return x.Format("02/01 15:04")
}

// whyMarks põe no gráfico a chuva, as anotações, as medições de objeto e os
// trechos fora de casa das 12 h.
func (a *App) whyMarks(out *WhyRise, from, t time.Time) {
	f, tt := from.UnixMilli(), t.UnixMilli()
	for _, n := range a.GetNotes(f-3600_000, tt) {
		if n.Time >= f && n.Time < tt {
			out.Marks = append(out.Marks, WhyMark{T: n.Time, End: n.End, Kind: "nota", Text: n.Text})
		}
	}
	a.mu.Lock()
	for _, m := range a.measurements {
		s, e := m.Obj.Start, m.Obj.End
		if m.Time > 0 && (s == 0 || m.Time < s) {
			s = m.Time
		}
		if s == 0 {
			continue
		}
		if e == 0 {
			e = s
		}
		if e >= f && s < tt {
			name := m.Name
			if name == "" {
				name = "medição de objeto"
			}
			out.Marks = append(out.Marks, WhyMark{T: max(s, f), End: min(e, tt), Kind: "objeto", Text: name})
		}
	}
	for _, p := range a.periods {
		if e := periodEnd(p); p.From < tt && e > f {
			out.Marks = append(out.Marks, WhyMark{T: max(p.From, f), End: min(e, tt), Kind: "fora", Text: p.Name})
		}
	}
	a.mu.Unlock()
	sort.Slice(out.Marks, func(i, j int) bool { return out.Marks[i].T < out.Marks[j].T })
	for _, s := range rainSpans(a.rainSlots(from, t)) {
		out.Rain = append(out.Rain, [2]int64{s.start.UnixMilli(), s.end.UnixMilli()})
	}
}
