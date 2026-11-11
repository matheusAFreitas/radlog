package app

import (
	"math"
	"math/rand"
	"testing"
)

// acumulado sintético parecido com o do RC102 do ACE (121 h, ~29 milhões de
// contagens): fundo da casa com rádio, tório e potássio, mais um contínuo
// liso e enorme de beta (o teste encostado nos eletrodos de tório)
func synthAccum(withBg bool, beta float64, seed int64) *MeasureSpectrum {
	return synthAccumCs(withBg, beta, 0, seed)
}

// synthAccumCs soma uma fonte de Cs-137 (pico em 662 e platô Compton até a
// borda de 477 keV, borrada pela resolução) com csN contagens no pico
func synthAccumCs(withBg bool, beta, csN float64, seed int64) *MeasureSpectrum {
	const n = 1024
	r := rand.New(rand.NewSource(seed))
	s := &MeasureSpectrum{A: [3]float64{5.43, 2.351, 0.000381}, BgSec: 1, ObjSec: 435600, Bg: make([]uint32, n), Obj: make([]uint32, n)}
	g := func(e, pe, c float64) float64 {
		sg := specFWHM(pe) / 2.355
		return c / (sg * math.Sqrt(2*math.Pi)) * math.Exp(-math.Pow((e-pe)/sg, 2)/2)
	}
	for i := 0; i < n; i++ {
		e := s.energy(i)
		de := s.energy(i+1) - e
		// contínuo do fundo (≈ 5 milhões de contagens) e o beta (liso)
		c := 9000 * math.Exp(-e/250) * de
		c += beta * (0.75*math.Exp(-e/55) + 0.22*math.Exp(-e/170) + 0.03*math.Exp(-e/700)) * de
		if csN > 0 {
			// platô com 3× as contagens do pico, subindo 30% até a borda
			sg := specFWHM(477) / 2.355
			c += 3 * csN / 477 * (1 + 0.3*e/477) * 0.5 * math.Erfc((e-477)/(sg*math.Sqrt2)) * de
			c += g(e, 661.7, csN) * de
		}
		if withBg {
			for _, p := range [][2]float64{{238.6, 60e3}, {295.2, 45e3}, {351.9, 80e3}, {583.2, 25e3}, {609.3, 70e3}, {911.2, 18e3}, {1120.3, 18e3}, {1460.8, 30e3}, {1764.5, 15e3}, {2614.5, 12e3}} {
				c += g(e, p[0], p[1]) * de
			}
		}
		v := c + r.NormFloat64()*math.Sqrt(c)
		if v < 0 {
			v = 0
		}
		s.Obj[i] = uint32(v)
	}
	return s
}

func TestFindPeaksSnipAccum(t *testing.T) {
	// só contínuo, sem linha nenhuma: não pode inventar isótopo
	for _, beta := range []float64{0, 40000, 120000, 400000} {
		for seed := int64(1); seed <= 10; seed++ {
			if p := synthAccum(false, beta, seed).findPeaks(); len(p) != 0 {
				t.Fatalf("beta %.0f seed %d: achou pico no contínuo liso: %+v", beta, seed, p)
			}
		}
	}
	// com o fundo da casa por baixo do beta: acha rádio, potássio e tório
	for _, beta := range []float64{0, 400000} {
		for seed := int64(1); seed <= 10; seed++ {
			p := synthAccum(true, beta, seed).findPeaks()
			for _, x := range p {
				if x.Nuclide == "Co-60" || x.Nuclide == "Cs-137" || x.Nuclide == "Am-241" {
					t.Fatalf("beta %.0f seed %d: %+v", beta, seed, p)
				}
			}
		}
	}
	p := synthAccum(true, 120000, 7).findPeaks()
	got := map[string]bool{}
	for _, x := range p {
		got[x.Nuclide] = true
		t.Logf("%s %.0f keV %.1fσ", x.Nuclide, x.KeV, x.Sigma)
	}
	for _, want := range []string{"Pb-214", "Bi-214", "K-40", "Tl-208"} {
		if !got[want] {
			t.Errorf("não achou %s: %+v", want, p)
		}
	}
	if got["Co-60"] || got["Cs-137"] || got["Am-241"] {
		t.Errorf("isótopo que não está lá: %+v", p)
	}
}

// QA: um Cs-137 no acumulado não pode virar rádio, tório nem urânio (o platô
// Compton fica côncavo no log e o SNIP passava por baixo dele)
func TestFindPeaksSnipCompton(t *testing.T) {
	for _, n := range []float64{2e5, 2e6} {
		for seed := int64(1); seed <= 5; seed++ {
			p := synthAccumCs(false, 120000, n, seed).findPeaks()
			if len(p) != 1 || p[0].Nuclide != "Cs-137" {
				t.Fatalf("cs %.0f seed %d: %+v", n, seed, p)
			}
		}
	}
}
