package app

import (
	"math"
	"testing"
	"time"

	"radlog/internal/gmc"
	"radlog/internal/store"
)

func TestPercentile(t *testing.T) {
	v := []float64{10, 20, 30, 40, 50}
	if percentile(v, .5) != 30 || percentile(v, .25) != 20 || percentile(v, 0.1) != 14 {
		t.Fatalf("p50=%v p25=%v p10=%v", percentile(v, .5), percentile(v, .25), percentile(v, .1))
	}
}

func TestMinuteStatsAndGaps(t *testing.T) {
	base := time.Date(2026, 11, 1, 0, 0, 0, 0, time.Local)
	var pts []store.Point
	for m := 0; m < 60; m++ {
		if m >= 20 && m < 30 { // 10 minutos sem dado
			continue
		}
		pts = append(pts, store.Point{T: base.Add(time.Duration(m) * time.Minute), CPM: float64(25 + m%5)})
	}
	s := calcMinuteStats(pts)
	if s.N != 50 || math.Abs(s.Mean-27) > 1e-9 || s.Min != 25 || s.Max != 29 {
		t.Fatalf("stats %+v", s)
	}
	if math.Abs(s.PoissonSD-math.Sqrt(27)) > 1e-9 || s.SD <= 0 {
		t.Fatalf("sd %v poisson %v", s.SD, s.PoissonSD)
	}
	g := findGaps(pts, base, base.Add(70*time.Minute))
	if len(g) != 2 || g[0].Minutes() != 10 || g[1].Minutes() != 10 {
		t.Fatalf("lacunas %+v", g)
	}
	if minutesAtOrAbove(pts, 29) != 10 {
		t.Fatal("tempo acima")
	}
	bins := histogram(pts, s.Mean)
	n := 0
	for _, b := range bins {
		n += b.Count
	}
	if n != 50 {
		t.Fatalf("histograma somou %d", n)
	}
}

func TestLastCal(t *testing.T) {
	a, _ := testApp(t)
	c := gmc.Calibration{Points: []gmc.CalPoint{{CPM: 1000, USv: 10.1}}, Source: "contador"}
	a.saveLastCal(c)
	a.cal = gmc.DefaultCalibration()
	a.loadLastCal()
	if a.cal.Source != calFromLast || math.Abs(a.cal.USv(100)-1.01) > 1e-9 {
		t.Fatalf("calibração %+v", a.cal)
	}
	// a de fábrica nunca é guardada
	a2, _ := testApp(t)
	a2.saveLastCal(gmc.DefaultCalibration())
	a2.loadLastCal()
	if a2.cal.Source == calFromLast {
		t.Fatal("guardou a de fábrica")
	}
}
