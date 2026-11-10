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
