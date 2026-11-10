package app

import (
	"math"
	"sort"
	"time"

	"radlog/internal/core/stats"
	"radlog/internal/store"
)

// ---------- estatística do relatório técnico ----------
//
// Tudo calculado sobre as leituras por minuto guardadas (o CPM que o
// contador mediu em cada minuto, já com o tempo morto se estiver ligado).
// Minuto sem leitura é lacuna: nunca entra como zero.

// minuteStats é a estatística das leituras por minuto de um período.
type minuteStats struct {
	N                 int
	Mean, Median, SD  float64 // SD: desvio-padrão amostral (n−1)
	PoissonSD         float64 // √média: o que o acaso sozinho daria
	Dispersion        float64 // SD / PoissonSD (1 = só acaso)
	P5, P25, P75, P95 float64
	Min, Max          float64
	MinAt, MaxAt      time.Time
}

// percentile com interpolação linear entre as posições (método "tipo 7",
// o padrão do Excel e do NumPy). sorted já ordenado.
func percentile(sorted []float64, p float64) float64 {
	if len(sorted) == 0 {
		return 0
	}
	if len(sorted) == 1 {
		return sorted[0]
	}
	h := (float64(len(sorted)) - 1) * p
	lo := math.Floor(h)
	i := int(lo)
	if i+1 >= len(sorted) {
		return sorted[len(sorted)-1]
	}
	return sorted[i] + (h-lo)*(sorted[i+1]-sorted[i])
}

// usvMinuteStats é a mesma estatística com cada minuto em µSv/h, pela
// calibração que valia nele (2.36). Os campos de Poisson não valem em µSv/h.
func usvMinuteStats(pts []store.Point, tl calTL) minuteStats {
	u := make([]store.Point, len(pts))
	for i, p := range pts {
		u[i] = store.Point{T: p.T, CPM: tl.USv(p)}
	}
	s := calcMinuteStats(u)
	s.PoissonSD, s.Dispersion = 0, 0
	return s
}

func calcMinuteStats(pts []store.Point) minuteStats {
	var s minuteStats
	s.N = len(pts)
	if s.N == 0 {
		return s
	}
	v := make([]float64, 0, s.N)
	sum := 0.0
	s.Min, s.Max = math.Inf(1), math.Inf(-1)
	for _, p := range pts {
		v = append(v, p.CPM)
		sum += p.CPM
		if p.CPM < s.Min {
			s.Min, s.MinAt = p.CPM, p.T
		}
		if p.CPM > s.Max {
			s.Max, s.MaxAt = p.CPM, p.T
		}
	}
	s.Mean = sum / float64(s.N)
	if s.N > 1 {
		ss := 0.0
		for _, x := range v {
			ss += (x - s.Mean) * (x - s.Mean)
		}
		s.SD = math.Sqrt(ss / float64(s.N-1))
	}
	if s.Mean > 0 {
		s.PoissonSD = math.Sqrt(s.Mean)
		s.Dispersion = s.SD / s.PoissonSD
	}
	sort.Float64s(v)
	s.Median = percentile(v, .5)
	s.P5, s.P25, s.P75, s.P95 = percentile(v, .05), percentile(v, .25), percentile(v, .75), percentile(v, .95)
	return s
}

// gap é um trecho sem leitura.
type gap struct {
	From, To time.Time
}

func (g gap) Minutes() int { return int(math.Round(g.To.Sub(g.From).Minutes())) }

// findGaps acha os trechos sem leitura dentro de [from, to): antes da
// primeira, entre duas leituras com mais de 1 minuto de distância e depois
// da última. pts em ordem de tempo.
func findGaps(pts []store.Point, from, to time.Time) []gap {
	var out []gap
	if !to.After(from) {
		return nil
	}
	if len(pts) == 0 {
		return []gap{{from, to}}
	}
	if d := pts[0].T.Sub(from); d >= 90*time.Second {
		out = append(out, gap{from, pts[0].T})
	}
	for i := 1; i < len(pts); i++ {
		if d := pts[i].T.Sub(pts[i-1].T); d >= 90*time.Second {
			out = append(out, gap{pts[i-1].T.Add(time.Minute), pts[i].T})
		}
	}
	last := pts[len(pts)-1].T.Add(time.Minute)
	if d := to.Sub(last); d >= 90*time.Second {
		out = append(out, gap{last, to})
	}
	return out
}

// daysWithData conta os dias (do calendário local) com pelo menos uma leitura.
func daysWithData(pts []store.Point) int {
	seen := map[string]bool{}
	for _, p := range pts {
		seen[p.T.Format("2006-01-02")] = true
	}
	return len(seen)
}

// minutesAtOrAbove conta os minutos com CPM ≥ limite.
func minutesAtOrAbove(pts []store.Point, limit float64) int {
	n := 0
	for _, p := range pts {
		if p.CPM >= limit {
			n++
		}
	}
	return n
}

// histBin é uma barra do histograma, com o que o Poisson esperaria nela.
type histBin struct {
	Lo, Hi   float64 // [Lo, Hi)
	Count    int
	Expected float64
}

func poissonPMF(k int, lambda float64) float64 {
	if lambda <= 0 || k < 0 {
		return 0
	}
	lg, _ := math.Lgamma(float64(k) + 1)
	return math.Exp(float64(k)*math.Log(lambda) - lambda - lg)
}

// histogram das leituras por minuto (até ~40 barras) e a curva de Poisson
// esperada para a mesma média e o mesmo número de minutos. Os valores muito
// altos (acima do P99,5) vão para a última barra, para um pico não esmagar
// o resto.
func histogram(pts []store.Point, mean float64) []histBin {
	if len(pts) == 0 {
		return nil
	}
	v := make([]float64, len(pts))
	for i, p := range pts {
		v[i] = p.CPM
	}
	sort.Float64s(v)
	lo := math.Floor(v[0])
	hi := math.Ceil(percentile(v, .995)) + 1
	if hi <= lo {
		hi = lo + 1
	}
	w := math.Max(1, math.Ceil((hi-lo)/40))
	n := int(math.Ceil((hi - lo) / w))
	bins := make([]histBin, n)
	for i := range bins {
		bins[i].Lo = lo + float64(i)*w
		bins[i].Hi = bins[i].Lo + w
		for k := int(bins[i].Lo); k < int(bins[i].Hi); k++ {
			bins[i].Expected += poissonPMF(k, mean) * float64(len(pts))
		}
	}
	for _, x := range v {
		i := int((math.Round(x) - lo) / w)
		if i >= n {
			i = n - 1
		}
		if i < 0 {
			i = 0
		}
		bins[i].Count++
	}
	return bins
}

// pointsIn devolve os pontos com T em [from, to].
func pointsIn(pts []store.Point, from, to time.Time) []store.Point {
	i := sort.Search(len(pts), func(i int) bool { return !pts[i].T.Before(from) })
	j := sort.Search(len(pts), func(i int) bool { return pts[i].T.After(to) })
	if i >= j {
		return nil
	}
	return pts[i:j]
}

// maxEpisodeCharts é quantos episódios ganham o gráfico de antes e depois.
const maxEpisodeCharts = 12

// homeMedian devolve a mediana do CPM gravado em casa em [from, to) e quantos
// minutos entraram (o demo não entra, como no fundo de referência).
func (a *App) homeMedian(from, to time.Time) (float64, int) {
	if a.store == nil {
		return 0, 0
	}
	pts, err := a.store.Range(from, to)
	if err != nil {
		return 0, 0
	}
	pts = a.homeOnly(pts)
	if len(pts) == 0 {
		return 0, 0
	}
	v := make([]float64, len(pts))
	for i, p := range pts {
		v[i] = p.CPM
	}
	return stats.Median(v), len(v)
}

// episodeSeconds junta os segundos gravados dos alertas que tocam
// [start, end], com 2 min de folga, sem repetir (atenção e alarme do mesmo
// episódio gravam os mesmos segundos).
func (a *App) episodeSeconds(evs []Event, start, end time.Time) []AlertPoint {
	f, t := start.Add(-alertPre).UnixMilli(), end.Add(alertPre).UnixMilli()
	seen := map[int64]bool{}
	var out []AlertPoint
	for _, e := range evs {
		stop := e.End
		if stop == 0 {
			stop = e.Time
		}
		if e.Retro || stop < start.UnixMilli() || e.Time > end.UnixMilli() {
			continue
		}
		for _, p := range a.readAlertSeconds(e) {
			if p.T < f || p.T > t || seen[p.T] {
				continue
			}
			seen[p.T] = true
			out = append(out, p)
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].T < out[j].T })
	return thinMax(out, 400)
}
