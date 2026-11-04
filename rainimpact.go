package main

import (
	"math"
	"sort"
	"time"

	"radlog/internal/store"
)

// ---------- chuva × radiação ----------
//
// A chuva traz para o chão os produtos do radônio do ar, e o CPM costuma
// subir um pouco por algumas horas. Aqui o app mede isso com os dados do
// próprio usuário: acha os episódios de chuva (radar quando há, senão o
// modelo), compara o CPM das 2 h antes com as 4 h depois e faz a mesma conta
// em horários sem chuva, para separar o efeito da chuva da variação normal
// (o máximo de 16 médias de 15 min sempre fica um pouco acima da média só
// por acaso).

const (
	rainSlot       = 10 * time.Minute
	rainWetModel   = 0.2 // mm na hora (modelo)
	rainWetRadar   = 0.3 // mm/h (radar; abaixo disso é garoa ou ruído)
	rainMergeGap   = time.Hour
	rainDryBefore  = 3 * time.Hour
	rainBaseWin    = 2 * time.Hour
	rainAfterWin   = 4 * time.Hour
	rainBin        = 15 * time.Minute
	rainKeepDays   = 60
	rainMinBaseMin = 60  // minutos com dado nas 2 h antes
	rainMinAftMin  = 120 // minutos com dado nas 4 h depois
)

// RainEpisode é uma chuva e o que o CPM fez depois dela.
type RainEpisode struct {
	Start    int64   `json:"start"` // ms
	End      int64   `json:"end"`
	MM       float64 `json:"mm"`     // chuva total estimada
	MaxMMh   float64 `json:"maxMMh"` // intensidade máxima
	Source   string  `json:"source"` // radar, modelo ou radar+modelo
	Baseline float64 `json:"baseline"`
	Peak     float64 `json:"peak"` // maior média de 15 min nas 4 h depois
	PeakAt   int64   `json:"peakAt"`
	Rise     float64 `json:"rise"`    // pico - base (CPM)
	BackAt   int64   `json:"backAt"`  // quando voltou perto da base (0 = não voltou em 4 h)
	Partial  bool    `json:"partial"` // as 4 h depois ainda não passaram
}

// RainNow é a chuva de agora (ou recente), que ainda não tem resultado.
type RainNow struct {
	Start    int64   `json:"start"`
	End      int64   `json:"end"` // último bloco com chuva
	Raining  bool    `json:"raining"`
	MM       float64 `json:"mm"`
	MaxMMh   float64 `json:"maxMMh"`
	Source   string  `json:"source"`
	Status   string  `json:"status"`   // medindo, contando ou ignorada
	ResultAt int64   `json:"resultAt"` // medindo: quando entra; contando: quando fecha
	Reason   string  `json:"reason"`   // ignorada: por quê
	PrevEnd  int64   `json:"prevEnd"`  // fim da chuva anterior (se atrapalhou)
	Baseline float64 `json:"baseline"`
	HasBase  bool    `json:"hasBase"`
	NowCPM   float64 `json:"nowCPM"` // média dos últimos 15 min
	HasNow   bool    `json:"hasNow"`
}

// RainCurvePt é um ponto da curva média, em minutos a partir do começo da
// chuva, do CPM menos a base (com chuva e sem chuva).
type RainCurvePt struct {
	Min  int      `json:"min"`
	Rain *float64 `json:"rain"`
	Dry  *float64 `json:"dry"`
}

// RainImpact é o resumo da tela "Chuva e radiação".
type RainImpact struct {
	Enabled     bool          `json:"enabled"`
	Episodes    []RainEpisode `json:"episodes"` // mais recente primeiro
	N           int           `json:"n"`
	AvgRise     float64       `json:"avgRise"` // subida média depois da chuva (CPM)
	AvgBase     float64       `json:"avgBase"`
	DryN        int           `json:"dryN"`
	DryRise     float64       `json:"dryRise"` // a mesma conta sem chuva
	Effect      float64       `json:"effect"`  // AvgRise - DryRise
	PeakMin     int           `json:"peakMin"` // mediana do tempo até o pico
	BackMin     int           `json:"backMin"` // mediana do tempo até voltar (das que voltaram)
	BackN       int           `json:"backN"`
	Curve       []RainCurvePt `json:"curve"`
	SkippedNoBg int           `json:"skippedNoData"` // chuvas sem dados suficientes
	From        int64         `json:"from"`
	Now         *RainNow      `json:"now"` // chuva em andamento ou recente
}

type rainRate struct {
	mmh   float64
	radar bool
}

// rainSlots monta a chuva em blocos de 10 min (radar onde houver, senão o
// modelo por hora espalhado nos 6 blocos).
func (a *App) rainSlots(from, to time.Time) map[int64]rainRate {
	a.mu.Lock()
	defer a.mu.Unlock()
	set := a.settings
	out := map[int64]rainRate{}
	radarHour := map[int64]bool{}
	if r := a.radar; r != nil && r.Lat == set.RainLat && r.Lon == set.RainLon {
		for k, p := range r.Points {
			t := time.Unix(k, 0)
			if t.Before(from) || !t.Before(to) {
				continue
			}
			s := t.Truncate(rainSlot).Unix()
			out[s] = rainRate{mmh: math.Max(out[s].mmh, p.MMh), radar: true}
			radarHour[t.Truncate(time.Hour).Unix()] = true
		}
	}
	if c := a.rain; c != nil && c.Lat == set.RainLat && c.Lon == set.RainLon {
		for h, mm := range c.Hours {
			if radarHour[h] {
				continue
			}
			t := time.Unix(h, 0)
			if t.Before(from.Add(-time.Hour)) || !t.Before(to) {
				continue
			}
			for i := 0; i < 6; i++ {
				out[t.Add(time.Duration(i)*rainSlot).Unix()] = rainRate{mmh: mm}
			}
		}
	}
	return out
}

func (r rainRate) wet() bool {
	if r.radar {
		return r.mmh >= rainWetRadar
	}
	return r.mmh >= rainWetModel
}

type rainSpan struct {
	start, end   time.Time
	mm, max      float64
	radar, model bool
}

// rainSpans junta os blocos com chuva em episódios (buracos de até 1 h
// continuam o mesmo episódio).
func rainSpans(slots map[int64]rainRate) []rainSpan {
	var keys []int64
	for k, r := range slots {
		if r.wet() {
			keys = append(keys, k)
		}
	}
	sort.Slice(keys, func(i, j int) bool { return keys[i] < keys[j] })
	var out []rainSpan
	for _, k := range keys {
		t := time.Unix(k, 0)
		r := slots[k]
		if n := len(out); n > 0 && t.Sub(out[n-1].end) <= rainMergeGap {
			s := &out[n-1]
			s.end = t.Add(rainSlot)
			s.mm += r.mmh / 6
			s.max = math.Max(s.max, r.mmh)
			s.radar = s.radar || r.radar
			s.model = s.model || !r.radar
			continue
		}
		out = append(out, rainSpan{start: t, end: t.Add(rainSlot), mm: r.mmh / 6, max: r.mmh, radar: r.radar, model: !r.radar})
	}
	return out
}

// minuteMap indexa os minutos (só em casa) por unix.
func minuteMap(pts []store.Point) map[int64]float64 {
	m := make(map[int64]float64, len(pts))
	for _, p := range pts {
		m[p.T.Truncate(time.Minute).Unix()] = p.CPM
	}
	return m
}

// meanIn é a média dos minutos em [from, to) e quantos havia.
func meanIn(m map[int64]float64, from, to time.Time) (float64, int) {
	sum, n := 0.0, 0
	for t := from.Truncate(time.Minute); t.Before(to); t = t.Add(time.Minute) {
		if v, ok := m[t.Unix()]; ok {
			sum += v
			n++
		}
	}
	if n == 0 {
		return 0, 0
	}
	return sum / float64(n), n
}

// window mede base, pico e volta a partir de t0. ok=false se faltar dado.
type winStat struct {
	base, peak float64
	peakAt     time.Time
	backAt     time.Time
	bins       []*float64 // média - base, de -2 h a +4 h, a cada 15 min
}

func measureWindow(m map[int64]float64, t0 time.Time, now time.Time) (winStat, bool) {
	var w winStat
	base, nb := meanIn(m, t0.Add(-rainBaseWin), t0)
	if nb < rainMinBaseMin {
		return w, false
	}
	end := t0.Add(rainAfterWin)
	if end.After(now) {
		end = now
	}
	_, na := meanIn(m, t0, end)
	if na < rainMinAftMin {
		return w, false
	}
	w.base, w.peak = base, math.Inf(-1)
	noise := math.Sqrt(math.Max(base, 1) / 15) // desvio de uma média de 15 min
	for t := t0.Add(-rainBaseWin); t.Before(t0.Add(rainAfterWin)); t = t.Add(rainBin) {
		v, n := meanIn(m, t, t.Add(rainBin))
		if n < 8 || !t.Add(rainBin).Before(now.Add(time.Minute)) {
			w.bins = append(w.bins, nil)
			continue
		}
		d := v - base
		w.bins = append(w.bins, &d)
		if !t.Before(t0) && v > w.peak {
			w.peak, w.peakAt = v, t.Add(rainBin/2)
		}
	}
	if math.IsInf(w.peak, -1) {
		return w, false
	}
	// voltou: primeira média de 15 min depois do pico até base + 1 desvio
	for t := w.peakAt.Add(rainBin / 2); t.Add(rainBin).Before(t0.Add(rainAfterWin).Add(time.Second)) && !t.After(now); t = t.Add(rainBin) {
		v, n := meanIn(m, t, t.Add(rainBin))
		if n >= 8 && v <= base+noise {
			w.backAt = t.Add(rainBin / 2)
			break
		}
	}
	return w, true
}

// GetRainImpact calcula a tela "Chuva e radiação" (últimos 60 dias).
func (a *App) GetRainImpact() (RainImpact, error) {
	a.mu.Lock()
	set := a.settings
	a.mu.Unlock()
	res := RainImpact{Enabled: set.RainEnabled && !(set.RainLat == 0 && set.RainLon == 0), Episodes: []RainEpisode{}, Curve: []RainCurvePt{}}
	if !res.Enabled {
		return res, nil
	}
	now := time.Now()
	from := now.Add(-rainKeepDays * 24 * time.Hour)
	pts, err := a.points(from.Add(-rainBaseWin), now.Add(time.Minute))
	if err != nil {
		return res, err
	}
	pts = a.homeOnly(pts)
	if len(pts) > 0 {
		res.From = pts[0].T.UnixMilli()
	}
	m := minuteMap(pts)
	slots := a.rainSlots(from.Add(-rainDryBefore), now)
	spans := rainSpans(slots)

	wet := func(t time.Time) bool {
		r, ok := slots[t.Truncate(rainSlot).Unix()]
		return ok && r.wet()
	}
	// só conta como "sem chuva" onde há dado de chuva (radar ou modelo)
	covered := func(t0, t1 time.Time) bool {
		n, have := 0, 0
		for x := t0; x.Before(t1); x = x.Add(rainSlot) {
			n++
			if _, ok := slots[x.Truncate(rainSlot).Unix()]; ok {
				have++
			}
		}
		return n > 0 && have*10 >= n*8
	}
	nBins := int((rainBaseWin + rainAfterWin) / rainBin)
	rainSum := make([]float64, nBins)
	rainCnt := make([]int, nBins)
	var peaks, backs []float64
	var lastEnd time.Time
	for _, s := range spans {
		prevEnd := lastEnd
		lastEnd = s.end
		// precisa de 3 h secas antes (senão a base já tem efeito da chuva)
		if !prevEnd.IsZero() && s.start.Sub(prevEnd) < rainDryBefore {
			continue
		}
		if s.start.Before(from) || now.Sub(s.start) < 2*time.Hour {
			continue
		}
		if prevEnd.IsZero() && !covered(s.start.Add(-rainDryBefore), s.start) {
			continue // não dá para saber se choveu antes
		}
		w, ok := measureWindow(m, s.start, now)
		if !ok {
			res.SkippedNoBg++
			continue
		}
		src := "modelo"
		if s.radar && s.model {
			src = "radar+modelo"
		} else if s.radar {
			src = "radar"
		}
		ep := RainEpisode{Start: s.start.UnixMilli(), End: s.end.UnixMilli(), MM: s.mm, MaxMMh: s.max, Source: src,
			Baseline: w.base, Peak: w.peak, PeakAt: w.peakAt.UnixMilli(), Rise: w.peak - w.base}
		if !w.backAt.IsZero() {
			ep.BackAt = w.backAt.UnixMilli()
			backs = append(backs, w.backAt.Sub(s.start).Minutes())
		}
		peaks = append(peaks, w.peakAt.Sub(s.start).Minutes())
		ep.Partial = now.Before(s.start.Add(rainAfterWin))
		res.Episodes = append(res.Episodes, ep)
		res.AvgRise += ep.Rise
		res.AvgBase += ep.Baseline
		for i, b := range w.bins {
			if b != nil && i < nBins {
				rainSum[i] += *b
				rainCnt[i]++
			}
		}
	}
	// o bloco de chuva mais recente que já tem dado (radar a cada 10 min)
	var latest time.Time
	for k := range slots {
		if t := time.Unix(k, 0); !t.After(now) && t.After(latest) {
			latest = t
		}
	}
	res.Now = rainNow(spans, m, now, latest, from, covered)
	res.N = len(res.Episodes)
	if res.N > 0 {
		res.AvgRise /= float64(res.N)
		res.AvgBase /= float64(res.N)
		res.PeakMin = int(median(peaks))
		if len(backs) > 0 {
			res.BackMin = int(median(backs))
			res.BackN = len(backs)
		}
	}

	// controle: começos de hora sem chuva de 3 h antes até 4 h depois
	drySum := make([]float64, nBins)
	dryCnt := make([]int, nBins)
	for t := from.Truncate(time.Hour); t.Add(rainAfterWin).Before(now); t = t.Add(3 * time.Hour) {
		if !covered(t.Add(-rainDryBefore), t.Add(rainAfterWin)) {
			continue
		}
		dry := true
		for x := t.Add(-rainDryBefore); x.Before(t.Add(rainAfterWin)); x = x.Add(rainSlot) {
			if wet(x) {
				dry = false
				break
			}
		}
		if !dry {
			continue
		}
		w, ok := measureWindow(m, t, now)
		if !ok {
			continue
		}
		res.DryN++
		res.DryRise += w.peak - w.base
		for i, b := range w.bins {
			if b != nil && i < nBins {
				drySum[i] += *b
				dryCnt[i]++
			}
		}
	}
	if res.DryN > 0 {
		res.DryRise /= float64(res.DryN)
	}
	if res.N > 0 && res.DryN > 0 {
		res.Effect = res.AvgRise - res.DryRise
	}
	for i := 0; i < nBins; i++ {
		pt := RainCurvePt{Min: int((time.Duration(i)*rainBin - rainBaseWin + rainBin/2) / time.Minute)}
		if rainCnt[i] > 0 {
			v := rainSum[i] / float64(rainCnt[i])
			pt.Rain = &v
		}
		if dryCnt[i] > 0 {
			v := drySum[i] / float64(dryCnt[i])
			pt.Dry = &v
		}
		res.Curve = append(res.Curve, pt)
	}
	// mais recente primeiro
	sort.Slice(res.Episodes, func(i, j int) bool { return res.Episodes[i].Start > res.Episodes[j].Start })
	return res, nil
}

// rainNow descreve a última chuva, se ela ainda não tem resultado fechado:
// chovendo agora, parou há pouco ou as 4 h depois ainda não passaram.
func rainNow(spans []rainSpan, m map[int64]float64, now, latest, from time.Time, covered func(t0, t1 time.Time) bool) *RainNow {
	if len(spans) == 0 {
		return nil
	}
	s := spans[len(spans)-1]
	if s.start.Before(from) || (now.Sub(s.end) > 30*time.Minute && !now.Before(s.start.Add(rainAfterWin))) {
		return nil
	}
	src := "modelo"
	if s.radar && s.model {
		src = "radar+modelo"
	} else if s.radar {
		src = "radar"
	}
	// chovendo = o último bloco com dado ainda tem chuva
	r := &RainNow{Start: s.start.UnixMilli(), End: s.end.UnixMilli(), Raining: !latest.IsZero() && s.end.After(latest) && now.Sub(s.end) <= 20*time.Minute,
		MM: s.mm, MaxMMh: s.max, Source: src}
	if v, n := meanIn(m, s.start.Add(-rainBaseWin), s.start); n >= rainMinBaseMin {
		r.Baseline, r.HasBase = v, true
	}
	if v, n := meanIn(m, now.Add(-15*time.Minute), now.Add(time.Minute)); n >= 8 {
		r.NowCPM, r.HasNow = v, true
	}
	switch {
	case len(spans) > 1 && s.start.Sub(spans[len(spans)-2].end) < rainDryBefore:
		r.Status = "ignorada"
		r.PrevEnd = spans[len(spans)-2].end.UnixMilli()
		r.Reason = "choveu menos de 3 h antes"
	case len(spans) == 1 && !covered(s.start.Add(-rainDryBefore), s.start):
		r.Status, r.Reason = "ignorada", "sem dado de chuva das 3 h antes"
	case !r.HasBase:
		r.Status, r.Reason = "ignorada", "sem leituras do contador nas 2 h antes"
	case now.Sub(s.start) < 2*time.Hour:
		r.Status, r.ResultAt = "medindo", s.start.Add(2*time.Hour).UnixMilli()
	default:
		r.Status, r.ResultAt = "contando", s.start.Add(rainAfterWin).UnixMilli()
	}
	return r
}
