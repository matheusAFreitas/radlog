package app

import (
	"math"
	"strings"
	"testing"
	"time"
)

func TestPressVerdict(t *testing.T) {
	if v := pressVerdict(nil, nil); v.Status != "sem" {
		t.Fatalf("sem pressão: %+v", v)
	}
	t0 := time.Date(2026, 9, 1, 0, 0, 0, 0, time.Local).Unix()
	mk := func(days int, sign float64) (map[int64]float64, map[int64]float64) {
		p, b := map[int64]float64{}, map[int64]float64{}
		for h := 0; h < days*24; h++ {
			x := 1013 + 6*math.Sin(float64(h)/37) + 3*math.Sin(float64(h)/11)
			p[t0+int64(h)*3600] = x
		}
		for h := 4; h < days*24; h++ {
			b[t0+int64(h)*3600] = 20 + sign*0.8*(p[t0+int64(h-4)*3600]-1013)
		}
		return p, b
	}
	p, b := mk(10, -1)
	v := pressVerdict(p, b)
	if v.Status != "pista" || v.Lag != 4 || v.R > -0.99 || v.Days < 9 || !strings.Contains(v.Title, "cerca de 4 h depois") {
		t.Fatalf("pista: %+v", v)
	}
	p, b = mk(30, -1)
	if v := pressVerdict(p, b); v.Status != "relacao" || !strings.HasPrefix(v.Title, "Relação:") {
		t.Fatalf("relação: %+v", v)
	}
	p, b = mk(10, 1)
	if v := pressVerdict(p, b); v.R < 0.99 || !strings.Contains(v.Title, "ao contrário") {
		t.Fatalf("positiva: %+v", v)
	}
	// sem relação: fundo constante com ruído que não acompanha
	p, _ = mk(10, -1)
	b = map[int64]float64{}
	for k := range p {
		b[k] = 20 + math.Mod(float64(k)*7919, 13)/13
	}
	if v := pressVerdict(p, b); !strings.HasPrefix(v.Title, "Não deu") {
		t.Fatalf("fraca: %+v", v)
	}
	// pouco dado
	p, b = mk(2, -1)
	if v := pressVerdict(p, b); v.Status != "pouco" {
		t.Fatalf("pouco: %+v", v)
	}
}

// dois ciclos diários se parecem sem um causar o outro: sem o ciclo do dia,
// não sobra relação (2.46)
func TestPressVerdictDailyCycle(t *testing.T) {
	t0 := time.Date(2026, 9, 1, 0, 0, 0, 0, time.Local).Unix()
	p, b := map[int64]float64{}, map[int64]float64{}
	seed := uint32(7)
	rnd := func() float64 { seed = seed*1664525 + 1013904223; return float64(seed>>8)/float64(1<<24) - 0.5 }
	walk := 0.0
	for h := 0; h < 40*24; h++ {
		k := t0 + int64(h)*3600
		hr := float64(time.Unix(k, 0).Hour())
		walk += rnd()
		p[k] = 1013 + 1.2*math.Cos(2*math.Pi*hr/12) + 0.3*walk // maré + tempo
		b[k] = 20 - 2*math.Cos(2*math.Pi*hr/12) + 0.2*rnd()    // só o ciclo da casa
	}
	if v := pressVerdict(p, b); strings.HasPrefix(v.Title, "Relação") || strings.HasPrefix(v.Title, "Pista") {
		t.Fatalf("ciclo diário virou relação: %+v", v)
	}
	// com ligação de verdade (a parte do tempo), continua achando
	for k := range b {
		b[k] -= 0.8 * (p[k] - 1013 - 1.2*math.Cos(2*math.Pi*float64(time.Unix(k, 0).Hour())/12))
	}
	if v := pressVerdict(p, b); v.Status != "relacao" || v.Chance >= pressAlpha {
		t.Fatalf("relação de verdade: %+v", v)
	}
}
