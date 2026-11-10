package app

import (
	"math"
	"strings"
	"time"

	"radlog/internal/store"
)

// ---------- resumo de um período (aba Resumo e dados novos) ----------

// SumPt é um ponto da curva do resumo (média do passo).
type SumPt struct {
	T   int64    `json:"t"` // ms
	CPM *float64 `json:"cpm"`
	USv *float64 `json:"usv"` // o mesmo ponto em µSv/h (calibração de cada minuto)
}

// Summary é o resumo de [From, To).
type Summary struct {
	From        int64    `json:"from"`
	To          int64    `json:"to"`
	Minutes     int      `json:"minutes"`     // minutos com leitura
	Coverage    float64  `json:"coverage"`    // 0..1, desde o primeiro dado e só até agora
	CoverageAll float64  `json:"coverageAll"` // 0..1 do período inteiro (até agora)
	AwayMinutes int      `json:"awayMinutes"`
	Avg         *float64 `json:"avg"`
	Min         *float64 `json:"min"`
	Max         *float64 `json:"max"`
	MinAt       int64    `json:"minAt"`
	MaxAt       int64    `json:"maxAt"`
	AvgUSv      float64  `json:"avgUSv"`
	Dose        float64  `json:"dose"` // µSv no período (só o que foi medido)
	BgCPM       float64  `json:"bgCPM"`
	BgOK        bool     `json:"bgOK"`
	VsBgPct     float64  `json:"vsBgPct"` // média em relação ao fundo, em %
	Attention   int      `json:"attention"`
	Alarm       int      `json:"alarm"`
	Danger      int      `json:"danger"`
	PeakAlert   float64  `json:"peakAlert"` // maior pico de alerta (CPM)
	// os mesmos em µSv/h (2.29: a tela do GMC mostra µSv/h; CPM só na
	// tabela técnica), cada um pela calibração da sua hora
	MinUSv       *float64 `json:"minUSv"`
	MaxUSv       *float64 `json:"maxUSv"`
	BgUSv        float64  `json:"bgUSv"`
	PeakAlertUSv float64  `json:"peakAlertUSv"`
	RainMM       float64  `json:"rainMM"`
	RainHours    int      `json:"rainHours"`
	RainKnown    bool     `json:"rainKnown"`
	Measurements int      `json:"measurements"`
	StepMs       int64    `json:"stepMs"`
	Curve        []SumPt  `json:"curve"`
	// cobertura e lacunas
	SpanMinutes int      `json:"spanMinutes"` // minutos do período até agora
	HistMinutes int      `json:"histMinutes"` // minutos desde o primeiro dado do GMC (até agora)
	PreMinutes  int      `json:"preMinutes"`  // minutos do período antes do primeiro dado
	HistFrom    int64    `json:"histFrom"`    // ms: começo do histórico dentro do período
	Ongoing     bool     `json:"ongoing"`     // o período ainda não terminou
	DaysData    int      `json:"daysData"`    // dias com pelo menos uma leitura
	Gaps        []SumGap `json:"gaps"`        // lacunas de 10 min ou mais (as mais recentes primeiro, até 30)
	GapCount    int      `json:"gapCount"`    // quantas lacunas de 10 min ou mais
	GapMinutes  int      `json:"gapMinutes"`  // minutos sem leitura desde o primeiro dado
	// estatística do fundo (em casa, sem medição de objeto)
	SD         *float64 `json:"sd"`         // desvio-padrão real dos minutos
	Dispersion *float64 `json:"dispersion"` // SD ÷ √média (1 = só acaso)
	// o RadiaCode no mesmo período (nil se não tem dado dele)
	RC *RcSum `json:"rc"`
}

// RcSum é o resumo do RadiaCode (seu dosímetro) num período, em µSv/h e µSv,
// separando o tempo em casa e fora de casa.
type RcSum struct {
	Minutes     int       `json:"minutes"`
	Coverage    float64   `json:"coverage"`    // desde o primeiro dado do RadiaCode, até agora
	CoverageAll float64   `json:"coverageAll"` // no período inteiro, até agora
	SpanMinutes int       `json:"spanMinutes"`
	HistMinutes int       `json:"histMinutes"`
	HistFrom    int64     `json:"histFrom"`
	Dose        float64   `json:"dose"`
	Avg         float64   `json:"avg"`
	Min         float64   `json:"min"`
	MinAt       int64     `json:"minAt"`
	Max         float64   `json:"max"`
	MaxAt       int64     `json:"maxAt"`
	BgUSv       float64   `json:"bgUSv"` // fundo em casa (mediana de 7 dias)
	VsBgPct     float64   `json:"vsBgPct"`
	HomeMinutes int       `json:"homeMinutes"`
	HomeDose    float64   `json:"homeDose"`
	AwayMinutes int       `json:"awayMinutes"`
	AwayDose    float64   `json:"awayDose"`
	AwayAvg     float64   `json:"awayAvg"`
	AwayMax     float64   `json:"awayMax"`
	AwayMaxAt   int64     `json:"awayMaxAt"`
	Curve       []RcSumPt `json:"curve"`
	// dose que o próprio aparelho somou no período (inclui o tempo longe do
	// PC); DevOK = o registro dos totais cobre o começo do período
	DevDose float64 `json:"devDose"`
	DevOK   bool    `json:"devOK"`
	DevUpTo int64   `json:"devUpTo"`
	// alertas do RadiaCode no período (por episódio) e o maior pico (µSv/h)
	Attention int     `json:"attention"`
	Alarm     int     `json:"alarm"`
	Danger    int     `json:"danger"`
	PeakAlert float64 `json:"peakAlert"`
}

// RcSumPt é um ponto da curva do RadiaCode no resumo (média do passo).
type RcSumPt struct {
	T   int64    `json:"t"`
	USv *float64 `json:"usv"`
}

// rcSummary resume o RadiaCode em [from, to), com os mesmos passos da curva
// do GMC.
func (a *App) rcSummary(from, to time.Time, step time.Duration, n int) *RcSum {
	pts := a.rcRange(from, to)
	if len(pts) == 0 {
		return nil
	}
	away := a.rcAwayChecker()
	r := &RcSum{Minutes: len(pts)}
	sums, cnt := make([]float64, n), make([]int, n)
	for i, p := range pts {
		d := p.USvh / 60
		r.Dose += d
		if i == 0 || p.USvh < r.Min {
			r.Min, r.MinAt = p.USvh, p.T.UnixMilli()
		}
		if i == 0 || p.USvh > r.Max {
			r.Max, r.MaxAt = p.USvh, p.T.UnixMilli()
		}
		if away(p.T) {
			r.AwayMinutes++
			r.AwayDose += d
			if p.USvh > r.AwayMax {
				r.AwayMax, r.AwayMaxAt = p.USvh, p.T.UnixMilli()
			}
		} else {
			r.HomeMinutes++
			r.HomeDose += d
		}
		if k := int(p.T.Sub(from) / step); k >= 0 && k < n {
			sums[k] += p.USvh
			cnt[k]++
		}
	}
	r.Avg = r.Dose * 60 / float64(r.Minutes)
	if d, up, ok := a.rcDeviceDose(from, to); ok {
		r.DevDose, r.DevOK, r.DevUpTo = d, true, up.UnixMilli()
	}
	if r.AwayMinutes > 0 {
		r.AwayAvg = r.AwayDose * 60 / float64(r.AwayMinutes)
	}
	first := a.rcFirst()
	if first.IsZero() || pts[0].T.Before(first) {
		first = pts[0].T
	}
	cs := coverageSpan(from, to, first, time.Now())
	r.SpanMinutes, r.HistMinutes, r.HistFrom = cs.Span, cs.Hist, cs.HistFrom.UnixMilli()
	r.Coverage, r.CoverageAll = ratio(r.Minutes, cs.Hist), ratio(r.Minutes, cs.Span)
	if st := a.rcStats(); st != nil && st.BgUSv > 0 {
		r.BgUSv = st.BgUSv
		r.VsBgPct = (r.Avg - st.BgUSv) / st.BgUSv * 100
	}
	for k := 0; k < n; k++ {
		pt := RcSumPt{T: from.Add(time.Duration(k) * step).UnixMilli()}
		if cnt[k] > 0 {
			v := sums[k] / float64(cnt[k])
			pt.USv = &v
		}
		r.Curve = append(r.Curve, pt)
	}
	a.mu.Lock()
	var evs []Event
	for _, e := range a.events {
		if e.Kind == kindAlert && e.Device == devRC && !e.Demo && e.Time >= from.UnixMilli() && e.Time < to.UnixMilli() {
			c := e
			c.Active = a.isActiveLocked(e.ID)
			evs = append(evs, c)
		}
	}
	a.mu.Unlock()
	for _, ep := range alertEpisodes(evs, time.Now()) {
		switch ep.Level {
		case "perigo":
			r.Danger++
		case "alarme":
			r.Alarm++
		default:
			r.Attention++
		}
		r.PeakAlert = math.Max(r.PeakAlert, ep.Peak)
	}
	return r
}

// SumGap é um trecho sem leitura, com a causa provável.
type SumGap struct {
	From    int64  `json:"from"`
	To      int64  `json:"to"`
	Minutes int    `json:"minutes"`
	Cause   string `json:"cause"`
	Open    bool   `json:"open"` // ainda sem leitura até agora
}

const (
	sumGapMin  = 10 * time.Minute
	sumGapList = 30
)

// gapCause diz a causa provável de uma lacuna olhando os eventos perto do
// começo dela. Fora de casa e app fechado com o contador sem histórico desse
// trecho (desligado, memória apagada) ficam como "sem leitura".
func gapCause(g gap, evs []Event, now time.Time) (string, bool) {
	if now.Sub(g.To) < 2*time.Minute {
		return "sem leitura até agora", true
	}
	for _, e := range evs {
		t := time.UnixMilli(e.Time)
		if t.Before(g.From.Add(-3*time.Minute)) || t.After(g.From.Add(3*time.Minute)) {
			continue
		}
		low := strings.ToLower(e.Text)
		switch {
		case strings.Contains(low, "parou de responder"):
			return "porta travou", false
		case strings.Contains(low, "desconectado"), strings.Contains(low, "conexão com o contador perdida"):
			return "contador desconectado", false
		}
	}
	return "app fechado e sem histórico do contador", false
}

func summaryStep(span time.Duration) time.Duration {
	switch {
	case span <= 26*time.Hour:
		return 15 * time.Minute
	case span <= 8*24*time.Hour:
		return time.Hour
	}
	return 6 * time.Hour
}

// GetSummary resume o período [from, to) em ms.
func (a *App) GetSummary(fromMs, toMs int64) (Summary, error) {
	from, to := time.UnixMilli(fromMs), time.UnixMilli(toMs)
	if to.Before(from) {
		from, to = to, from
	}
	if now := time.Now(); to.After(now.Add(time.Minute)) {
		to = now.Add(time.Minute)
	}
	s := Summary{From: from.UnixMilli(), To: to.UnixMilli(), Curve: []SumPt{}}
	pts, err := a.points(from.Truncate(time.Minute), to)
	if err != nil {
		return s, err
	}
	a.mu.Lock()
	tl := a.calTLLocked()
	demo := a.demo.Active
	var meas int
	for _, m := range a.measurements {
		if m.Time >= s.From && m.Time < s.To {
			meas++
		}
	}
	a.mu.Unlock()
	s.Measurements = meas
	away := a.awayChecker()

	step := summaryStep(to.Sub(from))
	s.StepMs = step.Milliseconds()
	n := int(math.Ceil(float64(to.Sub(from)) / float64(step)))
	sums, usums := make([]float64, n), make([]float64, n)
	cnt := make([]int, n)
	total, utotal := 0.0, 0.0
	for _, p := range pts {
		if p.T.Before(from) {
			continue
		}
		v := p.CPM
		s.Minutes++
		total += v
		u := tl.USv(p) // pela calibração que valia nesse minuto
		utotal += u
		s.Dose += u / 60
		if away(p.T) {
			s.AwayMinutes++
		}
		if s.Max == nil || v > *s.Max {
			x, xu := v, u
			s.Max, s.MaxAt, s.MaxUSv = &x, p.T.UnixMilli(), &xu
		}
		if s.Min == nil || v < *s.Min {
			x, xu := v, u
			s.Min, s.MinAt, s.MinUSv = &x, p.T.UnixMilli(), &xu
		}
		if i := int(p.T.Sub(from) / step); i >= 0 && i < n {
			sums[i] += v
			usums[i] += u
			cnt[i]++
		}
	}
	if s.Minutes > 0 {
		avg := total / float64(s.Minutes)
		s.Avg = &avg
		s.AvgUSv = utotal / float64(s.Minutes)
	}
	a.fillCoverage(&s, pts, from, to)
	s.RC = a.rcSummary(from, to, step, n)
	for i := 0; i < n; i++ {
		pt := SumPt{T: from.Add(time.Duration(i) * step).UnixMilli()}
		if cnt[i] > 0 {
			v, u := sums[i]/float64(cnt[i]), usums[i]/float64(cnt[i])
			pt.CPM, pt.USv = &v, &u
		}
		s.Curve = append(s.Curve, pt)
	}
	if bg, ok := a.backgroundCPM(); ok && bg > 0 {
		s.BgCPM, s.BgOK = bg, true
		a.mu.Lock()
		s.BgUSv = a.convCalLocked().USv(bg) // o fundo é dos últimos 30 dias: calibração de agora
		a.mu.Unlock()
		if s.Avg != nil {
			s.VsBgPct = (*s.Avg - bg) / bg * 100
		}
	}

	// alertas (o pico do alerta pode passar do valor do minuto). Conta por
	// episódio: atenção que virou alarme conta como um alarme só.
	a.mu.Lock()
	var evs []Event
	for _, e := range a.events {
		if e.Kind != kindAlert || e.Time < s.From || e.Time >= s.To || (e.Demo && !demo) || e.Device == devRC {
			continue
		}
		evs = append(evs, e)
	}
	a.mu.Unlock()
	for _, ep := range alertEpisodes(evs, time.Now()) {
		switch ep.Level {
		case "perigo":
			s.Danger++
		case "alarme":
			s.Alarm++
		default:
			s.Attention++
		}
		if ep.Peak > s.PeakAlert {
			s.PeakAlert = ep.Peak
			s.PeakAlertUSv = tl.USvAt(time.UnixMilli(ep.Start), ep.Peak)
		}
	}
	if s.Max != nil && s.PeakAlert > *s.Max {
		for _, p := range a.alertPeaks(from, to) {
			if p.CPM == s.PeakAlert {
				x, xu := p.CPM, tl.USvAt(time.UnixMilli(p.T), p.CPM)
				s.Max, s.MaxAt, s.MaxUSv = &x, p.T, &xu
			}
		}
	}

	// chuva (radar onde houver, senão o modelo)
	a.mu.Lock()
	rainOn := a.settings.RainEnabled
	a.mu.Unlock()
	if rainOn {
		slots := a.rainSlots(from, to)
		wetHours := map[int64]bool{}
		for k, r := range slots {
			t := time.Unix(k, 0)
			if t.Before(from) || !t.Before(to) {
				continue
			}
			s.RainKnown = true
			s.RainMM += r.mmh / 6
			if r.wet() {
				wetHours[t.Truncate(time.Hour).Unix()] = true
			}
		}
		s.RainHours = len(wetHours)
	}
	return s, nil
}

// fillCoverage completa o resumo com dias com dado, lacunas e a dispersão.
func (a *App) fillCoverage(s *Summary, pts []store.Point, from, to time.Time) {
	now := time.Now()
	in := pointsIn(pts, from, to)
	cs := coverageSpan(from, to, a.pointsFirst(pts), now)
	effTo := cs.EffTo
	in = pointsIn(in, from, effTo)
	s.SpanMinutes, s.HistMinutes, s.PreMinutes = cs.Span, cs.Hist, cs.Pre
	s.HistFrom, s.Ongoing = cs.HistFrom.UnixMilli(), cs.Ongoing
	s.Coverage, s.CoverageAll = ratio(len(in), cs.Hist), ratio(len(in), cs.Span)
	s.DaysData = daysWithData(in)
	s.GapMinutes = max(0, cs.Hist-len(in))
	s.Gaps = []SumGap{}

	a.mu.Lock()
	var evs []Event
	for _, e := range a.events {
		if e.Kind == kindEvent && e.Time >= from.Add(-5*time.Minute).UnixMilli() && e.Time < effTo.UnixMilli() {
			evs = append(evs, e)
		}
	}
	a.mu.Unlock()
	if cs.Hist == 0 {
		return
	}
	gs := findGaps(in, cs.HistFrom, effTo)
	for i := len(gs) - 1; i >= 0; i-- {
		g := gs[i]
		if g.To.Sub(g.From) < sumGapMin {
			continue
		}
		s.GapCount++
		if len(s.Gaps) < sumGapList {
			cause, open := gapCause(g, evs, now)
			s.Gaps = append(s.Gaps, SumGap{From: g.From.UnixMilli(), To: g.To.UnixMilli(), Minutes: g.Minutes(), Cause: cause, Open: open})
		}
	}

	// dispersão: só o fundo da casa (sem fora de casa nem objeto encostado)
	if st := calcMinuteStats(a.homeOnly(in)); st.N >= 30 && st.Mean > 0 {
		sd, disp := st.SD, st.Dispersion
		s.SD, s.Dispersion = &sd, &disp
	}
}
