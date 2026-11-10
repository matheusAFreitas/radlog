package app

import (
	"math"
	"testing"
	"time"

	"radlog/internal/store"
)

func TestIntercalFlow(t *testing.T) {
	a := newTestApp(t)
	a.loadCalHist()
	if v := a.GetIntercal(); v.State != "parado" {
		t.Fatalf("estado inicial %q", v.State)
	}
	start := time.Now().Add(-25 * time.Hour).Truncate(time.Minute)
	var pts []store.Point
	for i := 0; i < 24*60; i++ {
		obs := ""
		if i == 10 {
			obs = store.ObsSaturation // saturado: fica de fora
		}
		pts = append(pts, store.Point{T: start.Add(time.Duration(i) * time.Minute), CPM: 20, Src: store.SrcLive, Has: true, Raw: 20, Corr: 20, Obs: obs})
	}
	if _, err := a.store.Import(pts); err != nil {
		t.Fatal(err)
	}
	writeRc(t, a, start, 24*60, 0.12)

	k0 := a.curK()
	if err := a.saveIntercal(intercalData{Start: start.UnixMilli(), End: start.Add(intercalDur).UnixMilli(), K0: k0}); err != nil {
		t.Fatal(err)
	}
	v := a.GetIntercal()
	if v.State != "pronta" || v.Result == nil {
		t.Fatalf("depois das 24 h: %+v", v)
	}
	if math.Abs(v.Result.K-0.006) > 1e-9 || v.Result.Minutes != 24*60-1 {
		t.Fatalf("fator %v, minutos %d", v.Result.K, v.Result.Minutes)
	}
	if want := (k0/0.006 - 1) * 100; math.Abs(v.Pct-want) > 1e-6 {
		t.Fatalf("pct %v, quero %v", v.Pct, want)
	}
	if v, err := a.ApplyIntercal(false); err != nil || !v.Applied {
		t.Fatalf("aplicar: %+v %v", v, err)
	}
	if k := a.curK(); math.Abs(k-0.006) > 1e-9 {
		t.Fatalf("fator em uso %v", k)
	}
	a.mu.Lock()
	last := a.calLastLocked()
	a.mu.Unlock()
	if last.Source != srcIntercal || last.Note == "" {
		t.Fatalf("versão: %+v", last)
	}
	// a vigia começa a aprender a partir da calibração nova
	if c := a.GetConcord(); c.State != "sem-dados" && c.State != "aprendendo" {
		t.Fatalf("vigia logo depois de calibrar: %+v", c)
	}
	if err := a.CancelIntercal(); err != nil || a.GetIntercal().State != "parado" {
		t.Fatal("cancelar")
	}
}

func TestIntercalCollecting(t *testing.T) {
	a := newTestApp(t)
	a.loadCalHist()
	v, err := a.StartIntercal()
	if err != nil || v.State != "coletando" || v.Need != 1440 {
		t.Fatalf("%+v %v", v, err)
	}
	if _, err := a.ApplyIntercal(false); err == nil {
		t.Fatal("aplicar antes de terminar deveria falhar")
	}
}
