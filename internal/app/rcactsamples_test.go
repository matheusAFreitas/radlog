package app

import (
	"testing"
	"time"
)

func TestActSamples(t *testing.T) {
	a := newTestApp(t)
	// sem arquivo: as três sugestões
	if l := a.actSamples(); len(l) != 3 || l[0].ID != "uraninita" || !l[0].Loose {
		t.Fatalf("sugestões: %+v", l)
	}
	// a referência vem da medição de objeto com o mesmo nome (a do RC102 primeiro)
	now := time.Now().UnixMilli()
	a.mu.Lock()
	a.measurements = []SavedMeasure{
		{ID: 1, Name: "Uraninita encostada", Time: now - 2000, Obj: MeasurePhase{USv: 9.4}},
		{ID: 2, Name: "uraninita", Time: now - 3000, Device: devRC, Obj: MeasurePhase{USv: 0.83}, Distance: "encostado"},
		{ID: 3, Name: "uraninita", Time: now - 1000, Test: "distancia", Device: devRC, Obj: MeasurePhase{USv: 0.5}},
	}
	a.mu.Unlock()
	v := a.actSampleViews()
	if v[0].RefUSv != 0.83 || v[0].RefDev != devRC || v[0].RefDist != "encostado" {
		t.Fatalf("referência: %+v", v[0])
	}
	if v[1].RefUSv != 0 {
		t.Fatalf("uranocircita não tem medição: %+v", v[1])
	}
	// guardar uma nova: entra no topo e as sugestões continuam
	id, err := a.SaveActSample(ActSample{Name: " Monazita ", Nuclides: []string{"Th-232", "X-1"}, SmpSec: 3600, Loose: true})
	if err != nil || id == "" {
		t.Fatal(id, err)
	}
	l := a.actSamples()
	if len(l) != 4 || l[0].ID != id || l[0].Name != "Monazita" || len(l[0].Nuclides) != 1 || l[0].Seed {
		t.Fatalf("depois de guardar: %+v", l)
	}
	if _, err := a.SaveActSample(ActSample{Name: "x", Nuclides: []string{"X-1"}}); err == nil {
		t.Fatal("sem radionuclídeo válido devia falhar")
	}
	// apagar uma sugestão: não volta
	a.DeleteActSample("uraninita")
	for _, s := range a.actSamples() {
		if s.ID == "uraninita" {
			t.Fatal("a sugestão apagada voltou")
		}
	}
	if n := len(a.actSamples()); n != 3 {
		t.Fatalf("esperava 3, veio %d", n)
	}
}

func TestActValidateLoose(t *testing.T) {
	o := ActOpts{Kind: "amostra", Nuclides: []string{"U-238"}, Loose: true}
	if err := actValidate(&o); err != nil {
		t.Fatalf("pedra solta sem massa devia valer: %v", err)
	}
	o = ActOpts{Kind: "amostra", Nuclides: []string{"U-238"}}
	if err := actValidate(&o); err == nil {
		t.Fatal("amostra no pote sem massa devia falhar")
	}
	o = ActOpts{Kind: "kcl", Pct: 100, Loose: true}
	if err := actValidate(&o); err == nil || o.Loose {
		t.Fatalf("KCl sempre precisa da massa: %v %v", err, o.Loose)
	}
}
