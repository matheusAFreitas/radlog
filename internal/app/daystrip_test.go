package app

import (
	"testing"
	"time"

	"radlog/internal/store"
)

func TestDayStrip(t *testing.T) {
	a := newTestApp(t)
	end := time.Now().Truncate(stripStep)
	from := end.Add(-time.Hour)
	// 1 h de GMC: 40 min no fundo, 10 min bem alto, 10 min sem dado
	var pts []store.Point
	for i := 0; i < 50; i++ {
		cpm := 20.0
		if i >= 40 {
			cpm = 5000
		}
		pts = append(pts, store.Point{T: from.Add(time.Duration(i) * time.Minute), CPM: cpm, Src: store.SrcLive})
	}
	if _, err := a.store.Import(pts); err != nil {
		t.Fatal(err)
	}
	ds, err := a.GetDayStrip(from.UnixMilli(), end.UnixMilli())
	if err != nil {
		t.Fatal(err)
	}
	if len(ds.Slots) != 6 || ds.StepMs != stripStep.Milliseconds() {
		t.Fatalf("%d pedaços, passo %d", len(ds.Slots), ds.StepMs)
	}
	for i, s := range ds.Slots[:4] {
		if s.Home != "seguro" || s.HomeDev != "gmc" || s.HomeUSv <= 0 {
			t.Errorf("pedaço %d: %+v", i, s)
		}
		// GMC em casa e Radiacode com você, sem Radiacode: pessoal sem dado
		if s.Pers != "" {
			t.Errorf("pedaço %d pessoal: %+v", i, s)
		}
	}
	if s := ds.Slots[4]; s.Home == "seguro" || s.Home == "" {
		t.Errorf("pedaço alto: %+v", s)
	}
	if s := ds.Slots[5]; s.Home != "" {
		t.Errorf("sem dado: %+v", s)
	}
	// fora de casa com o GMC: os dois em você, a casa fica sem medição
	if _, err := a.SavePeriod(Period{From: from.UnixMilli(), To: from.Add(20 * time.Minute).UnixMilli(), Name: "passeio", GMC: "comigo", RC: "comigo"}); err != nil {
		t.Fatal(err)
	}
	ds, _ = a.GetDayStrip(from.UnixMilli(), end.UnixMilli())
	if s := ds.Slots[0]; !s.Away || s.Home != "" || s.Pers != "seguro" || s.PersDev != "gmc" {
		t.Errorf("fora de casa: %+v", s)
	}
	if s := ds.Slots[2]; s.Away || s.Home != "seguro" {
		t.Errorf("de volta: %+v", s)
	}
}
