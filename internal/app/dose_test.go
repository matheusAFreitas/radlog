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
	// cobertura desde o primeiro dado (2.3): as 10 h medidas são tudo o que existe
	if cov := c.Coverage; cov < 0.99 || c.HistFrom == 0 || c.HistMinutes > 602 || c.SpanMinutes < 1430 {
		t.Fatalf("cobertura %v, desde %v, %d de %d min", cov, c.HistFrom, c.HistMinutes, c.SpanMinutes)
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

func TestDoseFromDevice(t *testing.T) {
	a := newTestApp(t)
	now := time.Now().Truncate(time.Minute)
	// totais do aparelho: 0,15 µSv/h, zerado no meio (às -3 h)
	var pts []rcDosePt
	for m := 6 * 60; m >= 10; m -= 5 {
		at := now.Add(-time.Duration(m) * time.Minute)
		el := float64(6*60-m) * 60 // s desde -6 h
		dose := 0.5 + el/3600*0.15
		sec := 10000 + el
		if m <= 180 { // zerado às -3 h
			e2 := float64(180-m) * 60
			dose, sec = e2/3600*0.15, e2
		}
		pts = append(pts, rcDosePt{at, dose, sec})
	}
	a.rcDoseMerge(pts)
	// só 1 h de minutos gravados pelo app
	for m := 60; m >= 1; m-- {
		a.rcAddMinute(now.Add(-time.Duration(m)*time.Minute+time.Second), 12, 0.15)
	}
	a.rcFlushMinute()
	c, err := a.GetDoseSrc("6h", "publico", "radiacode")
	if err != nil {
		t.Fatal(err)
	}
	// do primeiro total (-6 h) ao último (-10 min): 5 h 50 min a 0,15 µSv/h
	want := 0.15 * (6*60 - 10 - 5) / 60.0 // os 5 min antes de zerar se perdem
	want += 0.15 * 10 / 60                // 2.12.2: depois do último total, os minutos do app
	if c.DoseSource != "aparelho" || math.Abs(c.Total-want) > 0.01 {
		t.Fatalf("dose do aparelho: %v (%v) quer %v", c.Total, c.DoseSource, want)
	}
	if math.Abs(c.AppTotal-0.15) > 0.01 || c.AppMinutes != 60 {
		t.Fatalf("conta do app: %v em %d min", c.AppTotal, c.AppMinutes)
	}
	if c.MeasuredMinutes < 340 || c.Dose[len(c.Dose)-1] != c.Total {
		t.Fatalf("minutos %d", c.MeasuredMinutes)
	}
}

// Trocar o fator hoje (sem recálculo retroativo) não pode fazer a dose do
// período passar do teto: o fundo usa, minuto a minuto, a mesma calibração
// que a dose. Com o recálculo ligado, os dois passam para o fator novo.
func TestDoseBackgroundSameCalibration(t *testing.T) {
	a := newTestApp(t)
	a.loadCalHist()
	now := time.Now().Truncate(time.Minute)
	var pts []store.Point
	for i := 3 * 24 * 60; i >= 1; i-- {
		pts = append(pts, store.Point{T: now.Add(-time.Duration(i) * time.Minute), CPM: 27, Src: store.SrcLive})
	}
	if _, err := a.store.Import(pts); err != nil {
		t.Fatal(err)
	}
	if err := a.SetGMCFactor(0.00564, "intercalibração"); err != nil {
		t.Fatal(err)
	}
	from, to := now.Add(-3*24*time.Hour).UnixMilli(), now.UnixMilli()
	c, err := a.GetDoseRange(from, to, "publico")
	if err != nil {
		t.Fatal(err)
	}
	avg := c.Total / float64(c.MeasuredMinutes) * 60
	if math.Abs(c.BackgroundRate-avg) > 1e-6 || math.Abs(c.Excess) > 1e-3 {
		t.Fatalf("fundo %v µSv/h × dose média %v µSv/h, excesso %v", c.BackgroundRate, avg, c.Excess)
	}
	h := a.GetCalHistory()
	if err := a.SetCalRetro(h.Versions[0].ID); err != nil {
		t.Fatal(err)
	}
	c, _ = a.GetDoseRange(from, to, "publico")
	if math.Abs(c.BackgroundRate-27*0.00564) > 1e-6 || math.Abs(c.Excess) > 1e-3 {
		t.Fatalf("com recálculo: fundo %v, excesso %v", c.BackgroundRate, c.Excess)
	}
}

// o pico de uma medição de objeto (aparelho encostado nele) não entra na
// dose: aqueles minutos valem o fundo (2.46)
func TestDoseSkipsObjectMeasure(t *testing.T) {
	dir := t.TempDir()
	st, err := store.Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	a := NewApp()
	a.dataDir, a.store, a.cal = dir, st, gmc.DefaultCalibration()
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
	objFrom, objTo := end.Add(-310*time.Minute), end.Add(-300*time.Minute)
	a.measurements = []SavedMeasure{{ID: 1, Name: "pedra", Time: objFrom.Add(-5 * time.Minute).UnixMilli(),
		Obj: MeasurePhase{Start: objFrom.UnixMilli(), End: objTo.UnixMilli()}}}
	c, err := a.GetDose("24h", "publico")
	if err != nil {
		t.Fatal(err)
	}
	usv := gmc.DefaultCalibration().USv
	if want := 600 * usv(20) / 60; math.Abs(c.Total-want) > 1e-9 {
		t.Fatalf("total %v, queria só o fundo %v", c.Total, want)
	}
	// o RC102 "com você" não é tirado
	if a.onObjectChecker(devRC)(objFrom.Add(time.Minute)) {
		t.Fatal("RC102 não estava no objeto")
	}
	a.measurements[0].Other = &MeasureSide{Device: devRC, Role: "voce"}
	if a.onObjectChecker(devRC)(objFrom.Add(time.Minute)) {
		t.Fatal("RC102 com você não está no objeto")
	}
	a.measurements[0].Other.Role = "objeto"
	if !a.onObjectChecker(devRC)(objFrom.Add(time.Minute)) {
		t.Fatal("os dois no objeto")
	}
}
