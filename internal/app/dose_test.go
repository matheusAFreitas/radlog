package app

import (
	"math"
	"testing"
	"time"

	"radlog/internal/gmc"
	"radlog/internal/store"
)

func TestGetDose(t *testing.T) {
	dir := t.TempDir()
	st, err := store.Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	a := NewApp()
	a.dataDir, a.store, a.cal = dir, st, gmc.DefaultCalibration()

	// 10 horas a 20 CPM, com 10 minutos a 220 CPM no meio, terminando agora.
	end := time.Now().Truncate(time.Minute)
	var pts []store.Point
	for i := 600; i >= 1; i-- {
		cpm := 20.0
		if i > 300 && i <= 310 {
			cpm = 220
		}
		pts = append(pts, store.Point{T: end.Add(-time.Duration(i) * time.Minute), CPM: cpm, Src: store.SrcLive})
	}
	if _, err := st.Import(pts); err != nil {
		t.Fatal(err)
	}

	c, err := a.GetDose("24h", "publico")
	if err != nil {
		t.Fatal(err)
	}
	usv := func(cpm float64) float64 { return gmc.DefaultCalibration().USv(cpm) } // calibração padrão
	wantTotal := (590*usv(20) + 10*usv(220)) / 60
	wantBg := 600 * usv(20) / 60 // mediana = 20 CPM
	wantAllow := 600 * 1000 / minutesPerYear
	near := func(a, b float64) bool { return math.Abs(a-b) < 1e-9 }
	if !near(c.Total, wantTotal) || !near(c.BackgroundTotal, wantBg) || !near(c.Allowance, wantAllow) ||
		!near(c.CeilingTotal, wantBg+wantAllow) || c.MeasuredMinutes != 600 || len(c.Times) != 288 {
		t.Fatalf("total %v fundo %v permitido %v teto %v min %d n %d", c.Total, c.BackgroundTotal, c.Allowance, c.CeilingTotal, c.MeasuredMinutes, len(c.Times))
	}
	if c.BackgroundSource != "mediana dos últimos 30 dias" || !near(c.BackgroundRate, usv(20)) {
		t.Fatalf("fundo: %v %q", c.BackgroundRate, c.BackgroundSource)
	}
	// as séries só crescem e terminam nos totais
	last := len(c.Dose) - 1
	if !near(c.Dose[last], c.Total) || !near(c.Ceiling[last], c.CeilingTotal) {
		t.Fatal("séries não batem com os totais")
	}
	for i := 1; i <= last; i++ {
		if c.Dose[i] < c.Dose[i-1] || c.Ceiling[i] < c.Ceiling[i-1] {
			t.Fatal("série acumulada diminuiu")
		}
	}
	// o pico de 10 min a 220 CPM excede o fundo em ~0,22 µSv, abaixo do
	// limite proporcional de 10 h (~1,14 µSv)
	if c.Excess <= 0 || c.Excess >= c.Allowance {
		t.Fatalf("excesso %v, permitido %v", c.Excess, c.Allowance)
	}
	if cov := c.Coverage; math.Abs(cov-600.0/1440) > 1e-9 {
		t.Fatalf("cobertura %v", cov)
	}

	w, _ := a.GetDose("7d", "trabalhador")
	if !near(w.Allowance, 600*20000/minutesPerYear) || len(w.Times) != 168 {
		t.Fatalf("trabalhador: %+v", w.Allowance)
	}
}

func TestDoseRange(t *testing.T) {
	a := newTestApp(t)
	base := time.Now().Truncate(time.Minute).Add(-5 * time.Hour)
	var pts []store.Point
	for i := 0; i < 240; i++ {
		pts = append(pts, store.Point{T: base.Add(time.Duration(i) * time.Minute), CPM: 30, Src: store.SrcLive})
	}
	if _, err := a.store.Import(pts); err != nil {
		t.Fatal(err)
	}
	// zoom de 1 h no meio: começa do zero, um ponto por minuto, 60 minutos medidos
	c, err := a.GetDoseRange(base.Add(2*time.Hour).UnixMilli(), base.Add(time.Hour).UnixMilli(), "publico")
	if err != nil {
		t.Fatal(err)
	}
	if c.StepMs != 60000 || c.MeasuredMinutes != 60 || len(c.Dose) != 60 || c.Dose[0] <= 0 {
		t.Fatalf("passo %d, minutos %d, pontos %d", c.StepMs, c.MeasuredMinutes, len(c.Dose))
	}
	want := 60 * a.cal.USv(30) / 60
	if d := c.Total - want; d > 1e-9 || d < -1e-9 {
		t.Fatalf("total %v, queria %v", c.Total, want)
	}
	// trecho longo usa passo maior
	if c, _ := a.GetDoseRange(time.Now().Add(-30*24*time.Hour).UnixMilli(), time.Now().UnixMilli(), "publico"); c.StepMs != time.Hour.Milliseconds() {
		t.Fatalf("passo do trecho longo: %d", c.StepMs)
	}
}
