package app

import (
	"math"
	"strings"
	"testing"
)

func TestFindPeaks(t *testing.T) {
	const n = 1024
	s := &MeasureSpectrum{A: [3]float64{0, 3, 0}, BgSec: 1000, ObjSec: 100, Bg: make([]uint32, n), Obj: make([]uint32, n)}
	for i := 0; i < n; i++ {
		s.Bg[i] = 1000 // 1 cps por canal
		// objeto: o mesmo fundo + um contínuo (Compton) + o pico do Cs-137
		e := s.energy(i)
		g := 3000 / (9.2 * math.Sqrt(2*math.Pi)) * math.Exp(-math.Pow((e-661.7)/3/9.2, 2)/2)
		s.Obj[i] = uint32(100 + 40 + g)
	}
	p := s.findPeaks()
	if len(p) != 1 || p[0].Group != "Césio-137" || p[0].Sigma < 10 {
		t.Fatalf("picos: %+v", p)
	}
	// sem pico, só o contínuo: nada
	for i := range s.Obj {
		s.Obj[i] = 140
	}
	if p := s.findPeaks(); len(p) != 0 {
		t.Fatalf("achou pico no contínuo: %+v", p)
	}
	// sem espectro do fundo: não procura
	s.Bg = nil
	if s.findPeaks() != nil {
		t.Fatal("sem fundo")
	}
}

func TestDualHint(t *testing.T) {
	cases := []struct {
		g, r float64
		want string
	}{
		{5, 1, "principalmente beta"}, {5, 2.5, "bastante beta"}, {1, 5, "Só o RadiaCode"}, {2.5, 5, "Só o RadiaCode"}, {1, 1, "Nenhum"},
	}
	for _, c := range cases {
		if h := dualHint(MeasureResult{Sigma: c.g, Ratio: 2}, MeasureResult{Sigma: c.r, Ratio: 1.5}); !strings.Contains(h, c.want) {
			t.Errorf("%v/%v: %q", c.g, c.r, h)
		}
	}
}
