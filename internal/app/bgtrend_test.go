package app

import (
	"math"
	"math/rand"
	"testing"
	"time"

	"radlog/internal/store"
)

func poissonR(r *rand.Rand, lam float64) float64 {
	l, k, p := math.Exp(-lam), 0, 1.0
	for {
		p *= r.Float64()
		if p <= l {
			return float64(k)
		}
		k++
	}
}

// synthDays gera minutos de days dias com Poisson em volta de level(dia),
// ciclo diário do radônio (±15%) e uma variação de dia para dia (clima).
func synthDays(seed int64, start time.Time, days int, level func(d int) float64) []store.Point {
	r := rand.New(rand.NewSource(seed))
	var pts []store.Point
	for d := 0; d < days; d++ {
		wx := 1 + r.NormFloat64()*0.02
		for m := 0; m < 1440; m++ {
			t := start.AddDate(0, 0, d).Add(time.Duration(m) * time.Minute)
			cyc := 1 + 0.15*math.Cos(2*math.Pi*float64(m)/1440)
			pts = append(pts, store.Point{T: t, CPM: poissonR(r, level(d)*cyc*wx), Src: store.SrcDevice})
		}
	}
	return pts
}

func TestBgFitSynthetic(t *testing.T) {
	start := time.Date(2026, 9, 1, 0, 0, 0, 0, time.Local)
	for seed := int64(1); seed <= 5; seed++ {
		// só ruído: estável
		f := bgFitDays(bgDaily(synthDays(seed, start, 30, func(int) float64 { return 20 })))
		if f.State != "estavel" || f.DaysUsed != 30 {
			t.Fatalf("ruído (semente %d): %s — %s", seed, f.State, f.Text)
		}
		// rampa de 20 a 26 em 30 dias: subindo, ~30%
		f = bgFitDays(bgDaily(synthDays(seed, start, 30, func(d int) float64 { return 20 + 6*float64(d)/29 })))
		if f.State != "subindo" || math.Abs(f.ChangePct-30) > 6 || math.Abs(f.Slope-6.0/29) > 0.05 {
			t.Fatalf("rampa (semente %d): %s %.1f%% %.3f — %s", seed, f.State, f.ChangePct, f.Slope, f.Text)
		}
		// descendo
		f = bgFitDays(bgDaily(synthDays(seed, start, 30, func(d int) float64 { return 26 - 6*float64(d)/29 })))
		if f.State != "descendo" {
			t.Fatalf("rampa para baixo (semente %d): %s — %s", seed, f.State, f.Text)
		}
		// degrau no dia 15: 20 → 24
		f = bgFitDays(bgDaily(synthDays(seed, start, 30, func(d int) float64 {
			if d < 15 {
				return 20
			}
			return 24
		})))
		if f.State != "degrau" || f.StepAt != start.AddDate(0, 0, 15).UnixMilli() || math.Abs(f.StepTo-f.StepFrom-4) > 0.6 {
			t.Fatalf("degrau (semente %d): %s em %v, %.1f → %.1f — %s", seed, f.State, time.UnixMilli(f.StepAt), f.StepFrom, f.StepTo, f.Text)
		}
	}
	// poucos dias
	if f := bgFitDays(bgDaily(synthDays(1, start, 5, func(int) float64 { return 20 }))); f.State != "insuficiente" {
		t.Fatalf("5 dias: %s", f.State)
	}
	// dia com menos de 12 h não entra
	pts := synthDays(1, start, 8, func(int) float64 { return 20 })
	pts = pts[:7*1440+600]
	if f := bgFitDays(bgDaily(pts)); f.DaysUsed != 7 {
		t.Fatalf("dia incompleto entrou: %d", f.DaysUsed)
	}
}

func TestGetBgTrend(t *testing.T) {
	a := newTestApp(t)
	today := time.Now().Truncate(24 * time.Hour)
	start := time.Date(today.Year(), today.Month(), today.Day(), 0, 0, 0, 0, time.Local).AddDate(0, 0, -10)
	pts := synthDays(7, start, 10, func(int) float64 { return 20 })
	pts[100].Obs, pts[100].Has, pts[100].CPM = store.ObsSaturation, true, 5000
	if _, err := a.store.Import(pts); err != nil {
		t.Fatal(err)
	}
	tr, err := a.GetBgTrend(30)
	if err != nil {
		t.Fatal(err)
	}
	if tr.Days != 30 || tr.StepMs != (3*time.Hour).Milliseconds() || len(tr.Pts) != 240 || len(tr.Daily) != 10 {
		t.Fatalf("trend: dias %d passo %d pts %d diários %d", tr.Days, tr.StepMs, len(tr.Pts), len(tr.Daily))
	}
	if tr.Daily[0].N != 1439 || tr.Daily[0].Mean > 25 {
		t.Fatalf("o minuto saturado entrou: %+v", tr.Daily[0])
	}
	var with int
	for _, p := range tr.Pts {
		if p.Mean != nil {
			with++
			if *p.Mean < 17 || *p.Mean > 23 || *p.Median < 17 || *p.Median > 23 {
				t.Fatalf("janela móvel: %v %v", *p.Mean, *p.Median)
			}
		}
	}
	if with < 70 || with > 90 {
		t.Fatalf("janelas com dado: %d", with)
	}
	if tr.Fit.State != "estavel" || tr.Fit.DaysUsed != 10 {
		t.Fatalf("tendência: %+v", tr.Fit)
	}
	if d := tr.Daily[3]; d.Dispersion < 1 || d.Dispersion > 2.5 || math.Abs(d.PoissonSD-math.Sqrt(d.Mean)) > 1e-9 {
		t.Fatalf("dia: %+v", d)
	}
}
