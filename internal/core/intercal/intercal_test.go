package intercal

import (
	"errors"
	"math"
	"math/rand"
	"testing"
	"time"
)

// poisson simples (Knuth) para médias pequenas
func poisson(r *rand.Rand, mean float64) float64 {
	l, k, p := math.Exp(-mean), 0.0, 1.0
	for {
		p *= r.Float64()
		if p <= l {
			return k
		}
		k++
	}
}

// pares sintéticos: GMC ~ Poisson(cpm), Radiacode com cps ~ 28× o GMC/60 e
// µSv/h = k × CPM esperado (com um pouco de ruído)
func synth(seed int64, start time.Time, minutes int, cpm, k, ratio float64) []Pair {
	r := rand.New(rand.NewSource(seed))
	ps := make([]Pair, minutes)
	for i := range ps {
		c := poisson(r, cpm)
		ps[i] = Pair{
			T:      start.Add(time.Duration(i) * time.Minute),
			Counts: c,
			USvh:   k * cpm * (1 + 0.02*r.NormFloat64()),
			CPS:    poisson(r, cpm*ratio) / 60,
		}
	}
	return ps
}

var t0 = time.Date(2026, 11, 9, 0, 5, 0, 0, time.UTC)

func TestComputeRecoversFactor(t *testing.T) {
	ps := synth(1, t0, 1440, 26.8, 0.00564, 28)
	r, err := Compute(ps)
	if err != nil {
		t.Fatal(err)
	}
	if math.Abs(r.K/0.00564-1) > 0.02 {
		t.Fatalf("K = %v, esperado ≈ 0,00564", r.K)
	}
	if r.Rel95 <= 0 || r.Rel95 > 0.05 {
		t.Fatalf("margem 95%% = %v", r.Rel95)
	}
	if !(r.Lo < 0.00564 && 0.00564 < r.Hi) {
		t.Fatalf("faixa %v–%v não contém o fator", r.Lo, r.Hi)
	}
	if r.Minutes != 1440 || len(r.Hours) != 24 {
		t.Fatalf("minutos %d, horas %d", r.Minutes, len(r.Hours))
	}
	if math.Abs(r.Dispersion-1) > 0.2 {
		t.Fatalf("tubo saudável deveria dar ≈ 1, deu %v", r.Dispersion)
	}
	if math.Abs(r.CountRatio/28-1) > 0.02 {
		t.Fatalf("razão de contagens %v", r.CountRatio)
	}
}

func TestComputeDropsOutliers(t *testing.T) {
	ps := synth(2, t0, 600, 26.8, 0.00564, 28)
	ps[100].Counts = 900 // fonte encostada um minuto
	ps[200].USvh = 5
	r, err := Compute(ps)
	if err != nil {
		t.Fatal(err)
	}
	if r.Outliers != 2 || r.Minutes != 598 {
		t.Fatalf("fora da curva %d, minutos %d", r.Outliers, r.Minutes)
	}
	if math.Abs(r.K/0.00564-1) > 0.03 {
		t.Fatalf("K = %v", r.K)
	}
}

func TestComputeFewPairs(t *testing.T) {
	if _, err := Compute(synth(3, t0, 30, 20, 0.006, 28)); !errors.Is(err, ErrFewPairs) {
		t.Fatalf("err = %v", err)
	}
}

func TestWatch(t *testing.T) {
	since := t0
	// 6 semanas, 16 h por dia juntos em casa
	var ps []Pair
	for d := 0; d < 42; d++ {
		ratio := 28.0
		if d >= 28 { // depois do aprendizado, o GMC passa a contar 8% a menos
			ratio = 28 * 1.08
		}
		ps = append(ps, synth(int64(10+d), since.Add(time.Duration(d)*24*time.Hour), 16*60, 25, 0.0056, ratio)...)
	}
	day := func(d float64) time.Time { return since.Add(time.Duration(d * 24 * float64(time.Hour))) }

	if w := WatchState(nil, since, day(3)); w.State != WatchNoData {
		t.Fatalf("sem minutos: %s", w.State)
	}
	if w := WatchState(ps, since, day(10)); w.State != WatchLearning || w.LearnedMin == 0 {
		t.Fatalf("aprendendo: %+v", w)
	}
	w := WatchState(ps[:28*16*60], since, day(28.5))
	if w.Normal <= 0 || math.Abs(w.Normal/28-1) > 0.02 {
		t.Fatalf("normal %v", w.Normal)
	}
	if w := WatchState(ps, since, day(35)); w.State != WatchOneWeek {
		t.Fatalf("uma semana fora: %s (%+v)", w.State, w.Last)
	}
	if w := WatchState(ps, since, day(42)); w.State != WatchDrift || w.Last.Pct < 5 {
		t.Fatalf("duas semanas fora: %s (%+v / %+v)", w.State, w.Last, w.Prev)
	}
	// sem a mudança: concordam
	var ok []Pair
	for d := 0; d < 42; d++ {
		ok = append(ok, synth(int64(100+d), since.Add(time.Duration(d)*24*time.Hour), 16*60, 25, 0.0056, 28)...)
	}
	if w := WatchState(ok, since, day(42)); w.State != WatchAgree || math.Abs(w.Last.Pct) > 2 {
		t.Fatalf("concordam: %s %+v", w.State, w.Last)
	}
}
