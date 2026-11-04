package main

import (
	"math"
	"time"
)

// ---------- resumo de um período (aba Resumo e dados novos) ----------

// SumPt é um ponto da curva do resumo (média do passo).
type SumPt struct {
	T   int64    `json:"t"` // ms
	CPM *float64 `json:"cpm"`
}

// Summary é o resumo de [From, To).
type Summary struct {
	From         int64    `json:"from"`
	To           int64    `json:"to"`
	Minutes      int      `json:"minutes"`  // minutos com leitura
	Coverage     float64  `json:"coverage"` // 0..1 do período
	AwayMinutes  int      `json:"awayMinutes"`
	Avg          *float64 `json:"avg"`
	Min          *float64 `json:"min"`
	Max          *float64 `json:"max"`
	MinAt        int64    `json:"minAt"`
	MaxAt        int64    `json:"maxAt"`
	AvgUSv       float64  `json:"avgUSv"`
	Dose         float64  `json:"dose"` // µSv no período (só o que foi medido)
	BgCPM        float64  `json:"bgCPM"`
	BgOK         bool     `json:"bgOK"`
	VsBgPct      float64  `json:"vsBgPct"` // média em relação ao fundo, em %
	Attention    int      `json:"attention"`
	Alarm        int      `json:"alarm"`
	Danger       int      `json:"danger"`
	PeakAlert    float64  `json:"peakAlert"` // maior pico de alerta (CPM)
	RainMM       float64  `json:"rainMM"`
	RainHours    int      `json:"rainHours"`
	RainKnown    bool     `json:"rainKnown"`
	Measurements int      `json:"measurements"`
	StepMs       int64    `json:"stepMs"`
	Curve        []SumPt  `json:"curve"`
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
	cal := a.cal
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
	sums := make([]float64, n)
	cnt := make([]int, n)
	total := 0.0
	for _, p := range pts {
		if p.T.Before(from) {
			continue
		}
		v := p.CPM
		s.Minutes++
		total += v
		s.Dose += cal.USv(v) / 60
		if away(p.T) {
			s.AwayMinutes++
		}
		if s.Max == nil || v > *s.Max {
			x := v
			s.Max, s.MaxAt = &x, p.T.UnixMilli()
		}
		if s.Min == nil || v < *s.Min {
			x := v
			s.Min, s.MinAt = &x, p.T.UnixMilli()
		}
		if i := int(p.T.Sub(from) / step); i >= 0 && i < n {
			sums[i] += v
			cnt[i]++
		}
	}
	if s.Minutes > 0 {
		avg := total / float64(s.Minutes)
		s.Avg = &avg
		s.AvgUSv = cal.USv(avg)
	}
	if span := to.Sub(from).Minutes(); span > 0 {
		s.Coverage = math.Min(1, float64(s.Minutes)/span)
	}
	for i := 0; i < n; i++ {
		pt := SumPt{T: from.Add(time.Duration(i) * step).UnixMilli()}
		if cnt[i] > 0 {
			v := sums[i] / float64(cnt[i])
			pt.CPM = &v
		}
		s.Curve = append(s.Curve, pt)
	}
	if bg, ok := a.backgroundCPM(); ok && bg > 0 {
		s.BgCPM, s.BgOK = bg, true
		if s.Avg != nil {
			s.VsBgPct = (*s.Avg - bg) / bg * 100
		}
	}

	// alertas (o pico do alerta pode passar do valor do minuto). Conta por
	// episódio: atenção que virou alarme conta como um alarme só.
	a.mu.Lock()
	var evs []Event
	for _, e := range a.events {
		if e.Kind != kindAlert || e.Time < s.From || e.Time >= s.To || (e.Demo && !demo) {
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
		s.PeakAlert = math.Max(s.PeakAlert, ep.Peak)
	}
	if s.Max != nil && s.PeakAlert > *s.Max {
		for _, p := range a.alertPeaks(from, to) {
			if p.CPM == s.PeakAlert {
				x := p.CPM
				s.Max, s.MaxAt = &x, p.T
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
