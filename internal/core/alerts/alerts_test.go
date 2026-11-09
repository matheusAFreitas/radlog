package alerts

import (
	"testing"
	"time"

	"radlog/internal/core/levels"
)

func series(t0 time.Time, cpms ...float64) []Sample {
	out := make([]Sample, len(cpms))
	for i, c := range cpms {
		out[i] = Sample{T: t0.Add(time.Duration(i) * time.Minute), CPM: c}
	}
	return out
}

func TestFind(t *testing.T) {
	lv := levels.Default()
	t0 := time.Date(2026, 1, 1, 10, 0, 0, 0, time.UTC)
	// entra em 50, fica acima de 45 (saída), pico 80, sai em 40
	eps := Find(series(t0, 20, 50, 46, 80, 60, 40, 20, 55, 20), 100, lv)
	if len(eps) != 2 {
		t.Fatalf("quero 2 episódios, veio %d", len(eps))
	}
	e := eps[0]
	if !e.Start.Equal(t0.Add(time.Minute)) || e.First != 50 || e.Peak != 80 || !e.PeakAt.Equal(t0.Add(3*time.Minute)) {
		t.Fatalf("episódio errado: %+v", e)
	}
	if !e.End().Equal(t0.Add(5 * time.Minute)) {
		t.Fatalf("fim %v, quero %v", e.End(), t0.Add(5*time.Minute))
	}
	// alarme do usuário abaixo da atenção vira a entrada
	if n := len(Find(series(t0, 30, 42, 30), 40, lv)); n != 1 {
		t.Fatalf("alarme em 40: quero 1 episódio, veio %d", n)
	}
}

func TestFindGap(t *testing.T) {
	lv := levels.Default()
	t0 := time.Date(2026, 1, 1, 10, 0, 0, 0, time.UTC)
	pts := []Sample{{t0, 60}, {t0.Add(Gap + time.Minute), 60}}
	if n := len(Find(pts, 100, lv)); n != 2 {
		t.Fatalf("falta de dado maior que Gap tem que separar: veio %d", n)
	}
}

func TestLevel(t *testing.T) {
	lv := levels.Default()
	if Level(60, 100, lv) != "atencao" || Level(150, 100, lv) != "alarme" || Level(1000, 100, lv) != "perigo" {
		t.Fatal("níveis errados")
	}
}
