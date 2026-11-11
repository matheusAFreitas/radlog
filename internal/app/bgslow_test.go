package app

import (
	"testing"
	"time"
)

// bsSeries monta 120 dias com GMC e RC dados por dia (i = 0 é o mais antigo).
func bsSeries(gm, rc func(i int) float64) BgSlow {
	o := BgSlow{State: "na"}
	t0 := time.Date(2026, 7, 1, 0, 0, 0, 0, time.Local)
	for i := 0; i < bsDays; i++ {
		d := BgSlowDay{Day: t0.AddDate(0, 0, i).UnixMilli(), GM: gm(i), RC: rc(i)}
		d.Valid = d.GM > 0
		o.Days = append(o.Days, d)
	}
	return o
}

func wobble(i int) float64 { return float64(i%5-2) * 0.1 }

func TestBgSlowStable(t *testing.T) {
	o := bsSeries(func(i int) float64 { return 20 + wobble(i) }, func(i int) float64 { return 560 + wobble(i) })
	bsAnalyze(&o)
	if o.State != "ok" || o.Rise > 0.02 || o.Rise < -0.02 {
		t.Fatalf("estável: %s %.3f", o.State, o.Rise)
	}
	if !bsSorted(o.Days) {
		t.Fatal("dias fora de ordem")
	}
}

func TestBgSlowAmbient(t *testing.T) {
	// sobe 15% devagar nos últimos 40 dias, os dois aparelhos
	f := func(i int) float64 {
		if i < 80 {
			return 1
		}
		return 1 + 0.15*float64(i-80)/30
	}
	o := bsSeries(func(i int) float64 { return 20 * min(f(i), 1.15) }, func(i int) float64 { return 560 * min(f(i), 1.15) })
	bsAnalyze(&o)
	if o.State != "warn" || !o.Env || o.Rise < 0.10 {
		t.Fatalf("ambiente: %s env=%v %.3f above=%d", o.State, o.Env, o.Rise, o.Above)
	}
	if o.Weeks < 2 {
		t.Fatalf("semanas %d", o.Weeks)
	}
}

func TestBgSlowDevice(t *testing.T) {
	f := func(i int) float64 {
		if i < 80 {
			return 1
		}
		return min(1+0.15*float64(i-80)/30, 1.15)
	}
	o := bsSeries(func(i int) float64 { return 20 * f(i) }, func(i int) float64 { return 560 })
	bsAnalyze(&o)
	if o.State != "warn" || o.Env || !o.RCOK {
		t.Fatalf("aparelho: %s env=%v rcok=%v", o.State, o.Env, o.RCOK)
	}
}

func TestBgSlowStep(t *testing.T) {
	o := bsSeries(func(i int) float64 {
		if i >= 100 {
			return 23 + wobble(i)
		}
		return 20 + wobble(i)
	}, func(i int) float64 { return 0 })
	bsAnalyze(&o)
	if o.State != "step" || o.StepDay != o.Days[100].Day || o.RCOK {
		t.Fatalf("degrau: %s step=%d want %d", o.State, o.StepDay, o.Days[100].Day)
	}
}

func TestBgSlowLittleData(t *testing.T) {
	o := bsSeries(func(i int) float64 {
		if i < 100 {
			return 0
		}
		return 20
	}, func(i int) float64 { return 0 })
	bsAnalyze(&o)
	if o.State != "na" || o.Left < 30 || o.RefGM != 20 {
		t.Fatalf("pouco dado: %s left=%d ref=%.1f", o.State, o.Left, o.RefGM)
	}
}

func TestBgSlowMoved(t *testing.T) {
	// mudou de lugar no dia 60: o fundo novo (24) vira a referência
	o := bsSeries(func(i int) float64 {
		if i >= 60 {
			return 24 + wobble(i)
		}
		return 20 + wobble(i)
	}, func(i int) float64 { return 0 })
	o.Moved = o.Days[60].Day
	bsAnalyze(&o)
	if o.State != "ok" || o.RefN < 30 {
		t.Fatalf("marca: %s refN=%d rise=%.3f", o.State, o.RefN, o.Rise)
	}
	// marca recente: falta dado para a referência nova
	o2 := bsSeries(func(i int) float64 { return 20 }, func(i int) float64 { return 0 })
	o2.Moved = o2.Days[95].Day
	bsAnalyze(&o2)
	if o2.State != "na" || o2.Left != 30-(106-95) {
		t.Fatalf("marca recente: %s left=%d refN=%d", o2.State, o2.Left, o2.RefN)
	}
}

func TestBgSlowFromStore(t *testing.T) {
	a := tubeFixture(t, 1, false, false)
	b, err := a.GetBgSlow(true)
	if err != nil {
		t.Fatal(err)
	}
	if len(b.Days) != bsDays || !bsSorted(b.Days) {
		t.Fatalf("%d dias", len(b.Days))
	}
	full, rc := 0, 0
	for _, d := range b.Days {
		if d.Valid {
			full++
			if d.GM < 18 || d.GM > 22 {
				t.Fatalf("mediana do dia %.1f", d.GM)
			}
		}
		if d.RC > 0 {
			rc++
		}
	}
	// 40 dias de dado: uns 38 inteiros; a referência (até 30 dias atrás) só tem uns 9
	if full < 37 || rc < 37 || b.State != "na" || b.RefGM < 18 || b.Left < 1 {
		t.Fatalf("válidos %d rc %d estado %s ref %.1f faltam %d", full, rc, b.State, b.RefGM, b.Left)
	}
	if b.RefUSv <= 0 {
		t.Fatal("sem µSv/h")
	}
	b2, err := a.MarkBgMoved(0)
	if err != nil || b2.Moved == 0 || a.settings.BgMoved != b2.Moved {
		t.Fatalf("marca: %v %d", err, b2.Moved)
	}
	if b3, _ := a.ClearBgMoved(); b3.Moved != 0 {
		t.Fatal("desfazer")
	}
	if _, err := a.MarkBgMoved(time.Now().Add(48 * time.Hour).UnixMilli()); err == nil {
		t.Fatal("dia no futuro")
	}
}
