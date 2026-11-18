package app

// QA (item 130): os nomes que o próprio app dá traduzem nos Eventos e no
// nome da medição; o nome que a pessoa escreveu fica como está. Rodar com
// RADLOG_PT_OK=1 (QA); no CI roda sem.

import (
	"strings"
	"testing"
	"time"

	"radlog/internal/i18n"
	"radlog/internal/radiacode"
)

func qa12FakeRC(a *App) {
	var sec float64
	cnt := make([]uint32, 1024)
	a.rc.mu.Lock()
	a.rc.specTest = func(bool) *radiacode.Spectrum {
		sec += 600
		for i := range cnt {
			cnt[i] += 3
		}
		return &radiacode.Spectrum{Duration: time.Duration(sec * float64(time.Second)), A0: 0, A1: 3, A2: 0, Counts: append([]uint32(nil), cnt...)}
	}
	a.rc.mu.Unlock()
}

func TestQA21012ActEventNames(t *testing.T) {
	a := newTestApp(t)
	qa12FakeRC(a)
	cases := []struct {
		o       ActOpts
		pt, en  string // o nome no evento, em pt e como sai em inglês
		started string // trecho do "começou" em inglês
	}{
		{ActOpts{Kind: "amostra", Name: "areia monazítica", SampleID: "p:areia", MassG: 500, Nuclides: []string{"Th-232"}, NewBg: true, BgSec: 3600, SmpSec: 3600}, "areia monazítica", "monazite sand", "monazite sand, 500 g"},
		{ActOpts{Kind: "amostra", Name: "Uraninita", SampleID: "s:uraninita", Loose: true, Nuclides: []string{"U-238"}, SmpSec: 3600}, "Uraninita", "Uraninite", "Uraninite, loose stone"},
		{ActOpts{Kind: "amostra", Name: "", SampleID: "p:outro", MassG: 100, Nuclides: []string{"K-40"}, SmpSec: 3600, NameOwn: true}, "amostra", "sample", "sample, 100 g"},
		{ActOpts{Kind: "torio", Pct: 2, MassG: 40, NewBg: true, BgSec: 3600}, "eletrodo de tório (2% ThO₂)", "thorium electrode (2% ThO₂)", "thorium electrode (2% ThO₂), 40 g"},
		{ActOpts{Kind: "kcl", Pct: 51, MassG: 250, NewBg: true, BgSec: 3600}, "KCl (51%)", "KCl (51%)", "KCl (51%), 250 g"},
		{ActOpts{Kind: "amostra", Name: "Relógio", SampleID: "p:outro", MassG: 50, Nuclides: []string{"K-40"}, SmpSec: 3600, NameOwn: true}, "Relógio", "Relógio", "Relógio, 50 g"},
		{ActOpts{Kind: "amostra", Name: "Pedra do quintal", SampleID: "p:rocha", MassG: 300, Nuclides: []string{"K-40"}, SmpSec: 3600, NameOwn: true}, "Pedra do quintal", "Pedra do quintal", "Pedra do quintal, 300 g"},
	}
	for _, c := range cases {
		if err := a.StartActivity(c.o); err != nil {
			t.Fatalf("%s: %v", c.pt, err)
		}
		for i := 0; i < 4 && a.GetActivity().Phase != "pronto"; i++ {
			if err := a.ActivityNext(); err != nil {
				t.Fatalf("%s: %v", c.pt, err)
			}
		}
		a.CloseActivity()
	}
	ev := a.GetEvents(kindEvent)
	for _, c := range cases {
		var got []string
		for _, e := range ev {
			if strings.HasPrefix(e.Text, "Atividade: ") && strings.Contains(e.Text, c.pt) {
				got = append(got, e.Text)
			}
		}
		if len(got) != 2 {
			t.Fatalf("%s: eventos %q", c.pt, got)
		}
		for _, s := range got {
			en := i18n.T("en", s)
			t.Logf("%q → %q", s, en)
			if !strings.Contains(en, c.en) || strings.Contains(en, "Atividade") {
				t.Errorf("%s: em inglês %q", c.pt, en)
			}
			if strings.Contains(s, "começou") && !strings.Contains(strings.NewReplacer("\u2068", "", "\u2069", "").Replace(en), c.started) {
				t.Errorf("%s: começou em inglês %q, queria %q", c.pt, en, c.started)
			}
		}
	}
}

func TestQA21012MeasureDefaultName(t *testing.T) {
	for _, c := range []struct{ name, test, en string }{
		{"objeto", "", "Measure object · object"},
		{"Relógio", "", "Measure object · \u2068Relógio\u2069"},
		{"Teste de distância", "distancia", "Measure object · Distance test"},
	} {
		m := SavedMeasure{Name: c.name, Test: c.test}
		if en := i18n.T("en", "Medir objeto · "+measNameTr(m)); en != c.en {
			t.Errorf("%s: %q, queria %q", c.name, en, c.en)
		}
	}
	a := newTestApp(t)
	fakeGMC(t, a, 25)
	if _, err := a.StartMeasure(MeasureOpts{Name: "", Distance: "encostado", Auto: true, HomeBg: true, MaxSec: 1800, Pct: 20}); err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 200 && a.GetMeasure().Phase == "objeto"; i++ {
		a.feedMeasure(0, 4)
	}
	ms := a.GetMeasurements()
	if len(ms) == 0 || ms[len(ms)-1].Name != measNameDefault {
		t.Fatalf("medição sem nome: %+v", ms)
	}
	if en := i18n.T("en", "Medição: "+measNameTr(ms[len(ms)-1])); en != "Measurement: object" {
		t.Errorf("título do PDF em inglês: %q", en)
	}
}
