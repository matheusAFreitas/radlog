package app

import (
	"math"
	"sort"
	"time"

	"radlog/internal/store"
)

// ---------- saúde do tubo do GMC-800 (2.57) ----------
//
// Quatro conferências com o que o app já gravou (spec: ux/tubo.html):
//   - conta sem falhar: nenhum trecho de 2+ min seguidos em 0 CPM em 30 dias;
//   - varia só pelo acaso: dispersão de cada dia (SD ÷ √média dos minutos em
//     casa, sem chuva nas 4 h antes, sem medição, fora de alerta) até 1,2;
//     "vale olhar" se passar de 1,2 em mais da metade dos últimos 7 dias;
//   - sem rajadas estranhas: até 3 minutos marcados com interferência em 30 d;
//   - sensibilidade estável: razão de contagens RC102 ÷ GMC-800 com os dois
//     em casa, primeira semana × última semana dos 90 dias, dentro de ±5%.
// Os estados não usam os níveis de radiação: "ok", "warn" e "na". Recalcula
// uma vez por dia (depois das 06:00) ou no botão "Conferir agora". Nada é
// escrito no aparelho.

const (
	tubeDays      = 30
	tubeRatioDays = 90
	tubeGapMin    = 2   // minutos seguidos em 0 CPM
	tubeDispMax   = 1.2 // dispersão máxima de um dia normal
	tubeDayMin    = 360 // minutos para um dia entrar na dispersão
	tubeBurstMax  = 3
	tubeRatioMax  = 0.05
	tubeRatioPair = 600 // minutos juntos em cada semana da razão
	tubeHour      = 6   // hora do recálculo diário
)

type TubeGap struct {
	At  int64 `json:"at"` // ms
	Min int   `json:"min"`
}

type TubeBin struct {
	K   int     `json:"k"`
	N   int     `json:"n"`
	Ref float64 `json:"ref"` // minutos esperados só pelo acaso
}

type TubeRatioPt struct {
	Day int64   `json:"day"`
	R   float64 `json:"r"`
}

type TubeHealth struct {
	At    int64 `json:"at"`
	First int64 `json:"first"` // primeiro minuto do GMC-800 (0 sem dado)

	Gaps    []TubeGap `json:"gaps"`
	GapsOK  string    `json:"gapsState"`
	GapDays int       `json:"gapDays"` // dias com leitura nos 30

	Disp      float64   `json:"disp"` // mediana das dispersões diárias
	DispDays  int       `json:"dispDays"`
	DispHigh  int       `json:"dispHigh"` // dias acima de 1,2 entre os últimos 7 com dado
	DispLast  int       `json:"dispLast"` // quantos dias com dado entre os últimos 7
	Hist      []TubeBin `json:"hist"`
	DispState string    `json:"dispState"`

	Bursts     []int64 `json:"bursts"`    // ms do minuto de cada rajada
	BurstHour  int     `json:"burstHour"` // hora que se repete (-1 se nenhuma)
	BurstState string  `json:"burstState"`

	Ratio      []TubeRatioPt `json:"ratio"`
	R0         float64       `json:"r0"`
	R1         float64       `json:"r1"`
	RatioChg   float64       `json:"ratioChg"`
	RatioWhy   string        `json:"ratioWhy"` // sem rc, poucos dias
	RatioState string        `json:"ratioState"`
}

// tubeDue diz se o resultado guardado (feito em at) já venceu às 06:00.
func tubeDue(at, now time.Time) bool {
	if at.IsZero() {
		return true
	}
	y, m, d := now.Date()
	six := time.Date(y, m, d, tubeHour, 0, 0, 0, now.Location())
	if now.Before(six) {
		six = six.AddDate(0, 0, -1)
	}
	return at.Before(six)
}

// GetTubeHealth devolve a saúde do tubo; force recalcula agora.
func (a *App) GetTubeHealth(force bool) (TubeHealth, error) {
	now := time.Now()
	a.tubeMu.Lock()
	defer a.tubeMu.Unlock()
	if !force && a.tubeCache != nil && !tubeDue(time.UnixMilli(a.tubeCache.At), now) {
		return *a.tubeCache, nil
	}
	h, err := a.tubeHealth(now)
	if err != nil {
		return h, err
	}
	a.tubeCache = &h
	return h, nil
}

func (a *App) tubeHealth(now time.Time) (TubeHealth, error) {
	h := TubeHealth{At: now.UnixMilli(), Gaps: []TubeGap{}, Hist: []TubeBin{}, Bursts: []int64{}, Ratio: []TubeRatioPt{}, BurstHour: -1,
		GapsOK: "na", DispState: "na", BurstState: "na", RatioState: "na"}
	if f := a.gmcFirst(); !f.IsZero() {
		h.First = f.UnixMilli()
	}
	if a.store == nil {
		return h, nil
	}
	to := now.Truncate(time.Minute).Add(time.Minute)
	from := to.AddDate(0, 0, -tubeDays)
	pts, err := a.store.Range(from, to)
	if err != nil {
		return h, err
	}
	tubeGaps(&h, pts)
	tubeBursts(&h, pts)
	a.tubeDisp(&h, pts, from, to)
	a.tubeRatio(&h, now)
	return h, nil
}

// trechos de 2+ minutos seguidos em 0 CPM (fora de saturação)
func tubeGaps(h *TubeHealth, pts []store.Point) {
	days := map[int64]bool{}
	var start time.Time
	n := 0
	var prev time.Time
	flush := func() {
		if n >= tubeGapMin {
			h.Gaps = append(h.Gaps, TubeGap{At: start.UnixMilli(), Min: n})
		}
		n = 0
	}
	for _, p := range pts {
		y, m, d := p.T.Date()
		days[time.Date(y, m, d, 0, 0, 0, 0, p.T.Location()).Unix()] = true
		zero := p.CPM == 0 && !p.HasObs(store.ObsSaturation)
		if !zero || (n > 0 && p.T.Sub(prev) > 90*time.Second) {
			flush()
		}
		if zero {
			if n == 0 {
				start = p.T
			}
			n++
		}
		prev = p.T
	}
	flush()
	h.GapDays = len(days)
	if len(pts) == 0 {
		return
	}
	h.GapsOK = "ok"
	if len(h.Gaps) > 0 {
		h.GapsOK = "warn"
	}
}

// minutos marcados com interferência; a hora que se repete em metade deles
func tubeBursts(h *TubeHealth, pts []store.Point) {
	hours := map[int]int{}
	for _, p := range pts {
		if p.HasObs(store.ObsInterference) {
			h.Bursts = append(h.Bursts, p.T.UnixMilli())
			hours[p.T.Hour()]++
		}
	}
	if len(pts) == 0 {
		return
	}
	h.BurstState = "ok"
	if len(h.Bursts) > tubeBurstMax {
		h.BurstState = "warn"
		// a hora com mais rajadas, se tiver metade delas; no empate, a menor
		best := 0
		for hr := 0; hr < 24; hr++ {
			if c := hours[hr]; c > best && c*2 >= len(h.Bursts) {
				best, h.BurstHour = c, hr
			}
		}
	}
}

// pois é a probabilidade de k contagens com média l.
func pois(l float64, k int) float64 {
	if l <= 0 {
		if k == 0 {
			return 1
		}
		return 0
	}
	lg, _ := math.Lgamma(float64(k) + 1)
	return math.Exp(float64(k)*math.Log(l) - l - lg)
}

func (a *App) tubeDisp(h *TubeHealth, pts []store.Point, from, to time.Time) {
	inAlert := a.alertChecker()
	slots := a.rainSlots(from.Add(-4*time.Hour), to)
	rainy := map[int64]bool{} // blocos de 10 min com chuva nas 4 h antes
	for k, r := range slots {
		if r.mmh <= 0 {
			continue
		}
		for t := time.Unix(k, 0); t.Before(time.Unix(k, 0).Add(4*time.Hour + rainSlot)); t = t.Add(rainSlot) {
			rainy[t.Unix()] = true
		}
	}
	type day struct {
		k  int64
		cs []float64
	}
	byDay := map[int64]*day{}
	var keys []int64
	for _, p := range a.homeOnly(pts) {
		c, ok := gmcCounts(p)
		if !ok || inAlert(p.T) || rainy[p.T.Truncate(rainSlot).Unix()] {
			continue
		}
		y, m, d := p.T.Date()
		k := time.Date(y, m, d, 0, 0, 0, 0, p.T.Location()).UnixMilli()
		if byDay[k] == nil {
			byDay[k] = &day{k: k}
			keys = append(keys, k)
		}
		byDay[k].cs = append(byDay[k].cs, math.Round(c))
	}
	sort.Slice(keys, func(i, j int) bool { return keys[i] < keys[j] })
	var disps []float64
	type use struct {
		mean float64
		cs   []float64
	}
	var used []use
	for _, k := range keys {
		cs := byDay[k].cs
		if len(cs) < tubeDayMin {
			continue
		}
		var s float64
		for _, c := range cs {
			s += c
		}
		mean := s / float64(len(cs))
		if mean <= 0 {
			continue
		}
		var ss float64
		for _, c := range cs {
			ss += (c - mean) * (c - mean)
		}
		disp := math.Sqrt(ss/float64(len(cs)-1)) / math.Sqrt(mean)
		disps = append(disps, disp)
		used = append(used, use{mean, cs})
	}
	h.DispDays = len(disps)
	if len(disps) == 0 {
		return
	}
	last := disps
	if len(last) > 7 {
		last = last[len(last)-7:]
	}
	h.DispLast = len(last)
	for _, d := range last {
		if d > tubeDispMax {
			h.DispHigh++
		}
	}
	s := append([]float64(nil), disps...)
	sort.Float64s(s)
	h.Disp = percentile(s, .5)
	h.DispState = "ok"
	if h.DispLast >= 3 && h.DispHigh*2 > h.DispLast {
		h.DispState = "warn"
	}
	// histograma dos minutos contra a soma das Poisson de cada dia
	cnt := map[int]int{}
	lo, hi, tot := math.MaxInt32, 0, 0
	for _, u := range used {
		for _, c := range u.cs {
			k := int(c)
			cnt[k]++
			tot++
		}
	}
	// faixa: do 0,2% ao 99,8% dos minutos, e pelo menos ±3√média
	acc := 0
	ks := make([]int, 0, len(cnt))
	for k := range cnt {
		ks = append(ks, k)
	}
	sort.Ints(ks)
	for _, k := range ks {
		acc += cnt[k]
		if lo == math.MaxInt32 && float64(acc) >= .002*float64(tot) {
			lo = k
		}
		if float64(acc) <= .998*float64(tot) || hi == 0 {
			hi = k
		}
	}
	var mAll float64
	for _, u := range used {
		mAll += u.mean * float64(len(u.cs))
	}
	mAll /= float64(tot)
	sd := math.Sqrt(mAll)
	lo = min(lo, max(0, int(math.Floor(mAll-3*sd))))
	hi = max(hi, int(math.Ceil(mAll+3*sd)))
	for k := lo; k <= hi; k++ {
		b := TubeBin{K: k, N: cnt[k]}
		for _, u := range used {
			b.Ref += float64(len(u.cs)) * pois(u.mean, k)
		}
		h.Hist = append(h.Hist, b)
	}
}

func (a *App) tubeRatio(h *TubeHealth, now time.Time) {
	from := now.AddDate(0, 0, -tubeRatioDays)
	ps := a.intercalPairs(from, now, true)
	if len(ps) == 0 {
		h.RatioWhy = "semrc"
		return
	}
	type acc struct {
		g, c float64
		n    int
	}
	byDay := map[int64]*acc{}
	var keys []int64
	lastPair := ps[0].T
	for _, p := range ps {
		y, m, d := p.T.Date()
		k := time.Date(y, m, d, 0, 0, 0, 0, p.T.Location()).UnixMilli()
		if byDay[k] == nil {
			byDay[k] = &acc{}
			keys = append(keys, k)
		}
		byDay[k].g += p.Counts
		byDay[k].c += p.CPS * 60
		byDay[k].n++
		if p.T.After(lastPair) {
			lastPair = p.T
		}
	}
	if now.Sub(lastPair) > tubeDays*24*time.Hour {
		h.RatioWhy = "semrc"
		return
	}
	sort.Slice(keys, func(i, j int) bool { return keys[i] < keys[j] })
	for _, k := range keys {
		if d := byDay[k]; d.n >= 60 && d.g > 0 {
			h.Ratio = append(h.Ratio, TubeRatioPt{Day: k, R: d.c / d.g})
		}
	}
	if len(h.Ratio) < 2 || h.Ratio[len(h.Ratio)-1].Day-h.Ratio[0].Day < 14*86400000 {
		h.RatioWhy = "poucos"
		return
	}
	week := func(fromMs, toMs int64) (float64, int) {
		var g, c float64
		n := 0
		for _, k := range keys {
			if k >= fromMs && k < toMs {
				g += byDay[k].g
				c += byDay[k].c
				n += byDay[k].n
			}
		}
		if g <= 0 {
			return 0, n
		}
		return c / g, n
	}
	f0 := h.Ratio[0].Day
	l1 := h.Ratio[len(h.Ratio)-1].Day + 86400000
	var n0, n1 int
	h.R0, n0 = week(f0, f0+7*86400000)
	h.R1, n1 = week(l1-7*86400000, l1)
	if n0 < tubeRatioPair || n1 < tubeRatioPair || h.R0 <= 0 {
		h.RatioWhy = "poucos"
		return
	}
	h.RatioChg = h.R1/h.R0 - 1
	h.RatioState = "ok"
	if math.Abs(h.RatioChg) >= tubeRatioMax {
		h.RatioState = "warn"
	}
}
