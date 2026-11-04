package main

import (
	"math"
	"testing"
	"time"

	"radlog/internal/gmc"
	"radlog/internal/store"
)

func TestDeadTime(t *testing.T) {
	lv := DefaultLevels().norm()
	if lv.dtCPM(30000) != 30000 || lv.dtCPS(500) != 500 {
		t.Fatal("desligada não pode mudar nada")
	}
	lv.DeadTimeOn = true // τ = 200 µs
	near := func(got, want, tol float64) bool { return math.Abs(got-want) <= tol }
	// 500 pulsos/s: 500·0,0002 = 0,1 → ÷0,9
	if g := lv.dtCPS(500); !near(g, 555.56, 0.01) {
		t.Fatalf("500 cps → %.2f", g)
	}
	if g := lv.dtCPM(30000); !near(g, 33333.3, 0.1) {
		t.Fatalf("30000 cpm → %.1f", g)
	}
	// fundo: praticamente nada
	if g := lv.dtCPM(28); !near(g, 28.0026, 0.0001) {
		t.Fatalf("28 cpm → %.4f", g)
	}
	// teto de 10×
	if g := lv.dtCPS(10000); !near(g, 100000, 0.5) {
		t.Fatalf("teto: %.1f", g)
	}
	// τ fora da faixa volta para dentro
	lv.DeadTimeUs = 9999
	if lv.norm().DeadTimeUs != 500 {
		t.Fatal("τ não foi limitado a 500 µs")
	}
}

func TestRecordsToPointsDeadTime(t *testing.T) {
	base := time.Date(2026, 11, 2, 10, 0, 0, 0, time.Local)
	var recs []gmc.Record
	for s := 1; s <= 60; s++ { // um minuto gravado por segundo, 500 pulsos/s
		recs = append(recs, gmc.Record{Time: base.Add(time.Duration(s) * time.Second), Count: 500, Unit: "CPS"})
	}
	recs = append(recs, gmc.Record{Time: base.Add(2 * time.Minute), Count: 30000, Unit: "CPM"})
	find := func(pts []store.Point, at time.Time) float64 {
		for _, p := range pts {
			if p.T.Equal(at) {
				return p.CPM
			}
		}
		return -1
	}
	off := recordsToPoints(recs, DefaultLevels().norm())
	lv := DefaultLevels().norm()
	lv.DeadTimeOn = true
	on := recordsToPoints(recs, lv)
	if find(off, base) != 30000 || find(on, base) != 33333 {
		t.Fatalf("minuto por segundo: sem %v, com %v", find(off, base), find(on, base))
	}
	if find(off, base.Add(2*time.Minute)) != 30000 || find(on, base.Add(2*time.Minute)) != 33333 {
		t.Fatalf("registro CPM: sem %v, com %v", find(off, base.Add(2*time.Minute)), find(on, base.Add(2*time.Minute)))
	}
}
