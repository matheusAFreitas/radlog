package app

import (
	"testing"
	"time"

	"radlog/internal/gmc"
)

func TestSuggestCounterCal(t *testing.T) {
	fab := gmc.DefaultCalibration().Points
	got, err := suggestCounterCal(fab, 0.00564)
	if err != nil {
		t.Fatal(err)
	}
	want := []float64{1773, 15368, 29505, 68279, 136683, 251160}
	for i, p := range got {
		if p.CPM != want[i] || p.USv != fab[i].USv {
			t.Fatalf("ponto %d = %+v, queria %v CPM = %v µSv/h", i+1, p, want[i], fab[i].USv)
		}
	}
	if k := got[0].USv / got[0].CPM; !sameK(k, 0.00564) {
		t.Fatalf("fator do 1º ponto = %v", k)
	}
	if _, err := suggestCounterCal(nil, 0.00564); err == nil {
		t.Fatal("sem pontos deveria falhar")
	}
	if _, err := suggestCounterCal(fab, 0); err == nil {
		t.Fatal("sem fator deveria falhar")
	}
}

func TestRotateCounterCal(t *testing.T) {
	fab := gmc.DefaultCalibration().Points
	n1, _ := suggestCounterCal(fab, 0.00564)
	n2, _ := suggestCounterCal(fab, 0.0055)
	now := time.Unix(1_800_000_000, 0)

	// 1ª gravação: a de fábrica fica guardada, sem anterior
	d := rotateCounterCal(counterCalData{}, fab, now)
	if d.Factory == nil || !d.Factory.Factory || !samePoints(d.Factory.Points, fab) || d.Previous != nil {
		t.Fatalf("1ª = %+v", d)
	}
	// 2ª: fábrica continua, anterior = a nossa primeira
	d = rotateCounterCal(d, n1, now.Add(time.Hour))
	if !samePoints(d.Factory.Points, fab) || d.Previous == nil || !samePoints(d.Previous.Points, n1) || d.Previous.Factory {
		t.Fatalf("2ª = %+v", d)
	}
	// 3ª: a anterior passa a ser a segunda
	d = rotateCounterCal(d, n2, now.Add(2*time.Hour))
	if !samePoints(d.Factory.Points, fab) || !samePoints(d.Previous.Points, n2) {
		t.Fatalf("3ª = %+v", d)
	}
	// voltando da de fábrica para outra: a de fábrica não vira "anterior"
	d = rotateCounterCal(d, fab, now.Add(3*time.Hour))
	if !samePoints(d.Previous.Points, n2) {
		t.Fatalf("depois de sair da de fábrica = %+v", d)
	}

	// contador que já não estava na de fábrica: guarda como original
	d = rotateCounterCal(counterCalData{}, n1, now)
	if d.Factory == nil || d.Factory.Factory {
		t.Fatalf("original não de fábrica = %+v", d)
	}
}

func TestCounterCalFile(t *testing.T) {
	a := NewApp()
	a.dataDir = t.TempDir()
	a.settings = defaultSettings()
	a.cal = gmc.DefaultCalibration()
	a.loadCalHist()
	if v := a.GetCounterCal(nil); !v.Same || v.Factory != nil {
		t.Fatalf("sem fator manual o contador e o app são iguais: %+v", v)
	}
	if err := a.setGMCFactor(0.00564, srcIntercal, "teste"); err != nil {
		t.Fatal(err)
	}
	a.counterCalChanged(a.cal.Points)
	v := a.GetCounterCal(nil)
	if v.Same || len(v.Suggested) != 6 || v.Suggested[0].CPM != 1773 || v.Source != srcIntercal || v.Factory == nil || !v.Factory.Factory {
		t.Fatalf("view = %+v", v)
	}
	// depois de gravar os pontos sugeridos, o fator do app continua o mesmo
	a.saveLastCal(gmc.Calibration{Points: v.Suggested, Source: "contador"})
	a.mu.Lock()
	a.cal = gmc.Calibration{Points: v.Suggested, Source: "contador"}
	a.mu.Unlock()
	if !sameK(a.curK(), 0.00564) {
		t.Fatalf("o app mudou o fator: %v", a.curK())
	}
	if v := a.GetCounterCal(nil); !v.Same {
		t.Fatalf("contador e app deveriam estar iguais: %+v", v)
	}
}
