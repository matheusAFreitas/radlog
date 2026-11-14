package app

import (
	"testing"
	"time"
)

// Dados reais de 14/11, 14h11: o GMC-800 encostou na pedra por uns 20 s no
// meio do minuto. O último CPM do minuto (101) perdia o pico; a média dos
// CPS fica com ele.
func TestGMCMinuteKeepsShortPeak(t *testing.T) {
	cpm := []uint32{312, 311, 317, 310, 309, 312, 302, 294, 292, 287, 283, 276, 286, 297, 289, 288, 292, 288, 283, 280, 276, 281, 287, 285, 280, 273, 304, 1580, 2680, 3340, 3260, 3330, 3384, 3400, 3051, 1480, 360, 420, 740, 880, 860, 840, 1020, 1400, 1920, 2000, 1500, 680, 140, 180, 180, 156, 140, 128, 120, 113, 108, 103, 95, 101}
	m0 := time.Date(2026, 11, 14, 14, 11, 0, 0, time.Local)
	var g gmcMinute
	var lv Levels
	sum := 0.0
	for i, c := range cpm {
		cps := c / 60 // as contagens de cada segundo seguem a estimativa
		sum += float64(cps)
		if _, ok := g.add(m0.Add(time.Duration(i)*time.Second+300*time.Millisecond), c, c, cps, cps, lv); ok {
			t.Fatal("gravou no meio do minuto")
		}
	}
	p, ok := g.add(m0.Add(time.Minute+300*time.Millisecond), 90, 90, 1, 1, lv)
	if !ok || !p.T.Equal(m0) {
		t.Fatalf("não gravou o minuto: %+v %v", p, ok)
	}
	if p.CPM != sum || p.Raw != sum {
		t.Fatalf("minuto = %v, esperado %v (soma dos CPS)", p.CPM, sum)
	}
	if p.CPM < 700 {
		t.Fatalf("o pico sumiu: %v", p.CPM)
	}
}

// Poucas leituras no minuto (o app conectou aos 48 s): fica o CPM do
// contador; e sem ler até perto do fim, não grava.
func TestGMCMinuteFewSamples(t *testing.T) {
	m0 := time.Date(2026, 11, 14, 10, 0, 0, 0, time.Local)
	var g gmcMinute
	var lv Levels
	for s := 48; s < 60; s += 3 {
		g.add(m0.Add(time.Duration(s)*time.Second), 25, 25, 0, 0, lv)
	}
	p, ok := g.add(m0.Add(time.Minute), 26, 26, 1, 1, lv)
	if !ok || p.CPM != 25 {
		t.Fatalf("%+v %v", p, ok)
	}
	// minuto lido só até os 30 s
	for s := 1; s <= 30; s++ {
		g.add(m0.Add(time.Minute+time.Duration(s)*time.Second), 26, 26, 0, 0, lv)
	}
	if _, ok := g.add(m0.Add(3*time.Minute), 26, 26, 0, 0, lv); ok {
		t.Fatal("gravou um minuto lido pela metade")
	}
}
