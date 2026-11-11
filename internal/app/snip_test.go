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
	return synthAccumSrc(withBg, beta, 661.7, csN, 0, seed)
}

// synthAccumSrc: fonte com pico em pe (n contagens), platô Compton e, se bs >
// 0, o morro do retroespalhamento (bs × n contagens, 2,5× a largura do pico).
func synthAccumSrc(withBg bool, beta, pe, csN, bs float64, seed int64) *MeasureSpectrum {
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
			ce := pe * (2 * pe / 511) / (1 + 2*pe/511)
			sg := specFWHM(ce) / 2.355
			c += 3 * csN / ce * (1 + 0.3*e/ce) * 0.5 * math.Erfc((e-ce)/(sg*math.Sqrt2)) * de
			c += g(e, pe, csN) * de
			if bs > 0 {
				eb := pe / (1 + 2*pe/511)
				sb := 2.5 * specFWHM(eb) / 2.355
				c += bs * csN / (sb * math.Sqrt(2*math.Pi)) * math.Exp(-math.Pow((e-eb)/sb, 2)/2) * de
			}
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

// o morro do retroespalhamento (E/(1+2E/511), bem mais largo que um pico)
// não pode virar U-235 em 186 nem Pb-212 em 239
func TestFindPeaksSnipBackscatter(t *testing.T) {
	for _, src := range []struct {
		kev float64
		nuc string
	}{{661.7, "Cs-137"}, {1460.8, "K-40"}} {
		for _, n := range []float64{2e5, 2e6} {
			for seed := int64(1); seed <= 5; seed++ {
				// morro com 10% da altura do pico (25% da área), como no teste da QA
				p := synthAccumSrc(false, 120000, src.kev, n, 0.25, seed).findPeaks()
				if len(p) != 1 || p[0].Nuclide != src.nuc {
					t.Fatalf("%s %.0f seed %d: %+v", src.nuc, n, seed, p)
				}
			}
		}
	}
}

// com o morro e o fundo de verdade juntos, os picos reais continuam lá (o
// Pb-212 de 239, com 60 mil contagens, some embaixo do platô do Cs-137 de
// qualquer jeito); e o Pb-214 de 295 não cai como retroespalhamento do 609
func TestFindPeaksSnipBackscatterKeepsReal(t *testing.T) {
	for seed := int64(1); seed <= 5; seed++ {
		got := map[string]bool{}
		for _, p := range synthAccumSrc(true, 120000, 661.7, 2e5, 0.25, seed).findPeaks() {
			got[p.Nuclide] = true
		}
		for _, want := range []string{"Cs-137", "Pb-214", "Bi-214", "K-40", "Tl-208"} {
			if !got[want] {
				t.Fatalf("seed %d: perdeu %s: %v", seed, want, got)
			}
		}
		if got["U-235"] {
			t.Fatalf("seed %d: U-235 falso no retroespalhamento", seed)
		}
	}
}

func TestDropBackscatter(t *testing.T) {
	ps := []SpecPeak{
		{Nuclide: "Tl-208", KeV: 2614.5, Net: 1e5},
		{Nuclide: "Pb-212", KeV: 238.6, Net: 5e5}, // tório: bem mais forte que o Tl-208
		{Nuclide: "Bi-214", KeV: 609.3, Net: 6.6e4},
		{Nuclide: "Pb-214", KeV: 295.2, Net: 3.4e4}, // metade do 609: fica
		{Nuclide: "Cs-137", KeV: 661.7, Net: 1.6e6},
		{Nuclide: "U-235", KeV: 185.7, Net: 5.8e4}, // retroespalhamento do Cs-137
	}
	got := map[string]bool{}
	for _, p := range dropBackscatter(ps) {
		got[p.Nuclide] = true
	}
	if !got["Pb-212"] || !got["Pb-214"] || !got["Tl-208"] || !got["Cs-137"] || got["U-235"] {
		t.Fatalf("%v", got)
	}
}
