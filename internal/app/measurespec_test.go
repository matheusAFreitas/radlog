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
		{5, 1, "principalmente beta"}, {5, 2.5, "bastante beta"}, {1, 5, "Só o RC102"}, {2.5, 5, "Só o RC102"}, {1, 1, "Nenhum"},
	}
	for _, c := range cases {
		if h := dualHint(MeasureResult{Sigma: c.g, Ratio: 2}, MeasureResult{Sigma: c.r, Ratio: 1.5}); !strings.Contains(h, c.want) {
			t.Errorf("%v/%v: %q", c.g, c.r, h)
		}
	}
}

// fundo real do Radiacode (7,4 h): sem fundo para subtrair, só o objeto
// "igual ao fundo" não pode dar pico nenhum; e as linhas do Bi-214 (1120 e
// 1238) em volta de 1173 não podem virar Co-60
func TestFindPeaksBi214NotCo60(t *testing.T) {
	const n = 1024
	s := &MeasureSpectrum{A: [3]float64{5.43, 2.351, 0.000381}, BgSec: 1000, ObjSec: 1000, Bg: make([]uint32, n), Obj: make([]uint32, n)}
	for i := 0; i < n; i++ {
		e := s.energy(i)
		cont := 400 * math.Exp(-e/300)
		s.Bg[i] = uint32(cont)
		g := func(pe, c float64) float64 {
			sg := specFWHM(pe) / 2.355
			return c * 2.35 / (sg * math.Sqrt(2*math.Pi)) * math.Exp(-math.Pow((e-pe)/sg, 2)/2)
		}
		// objeto com rádio: 609, 1120, 1238, 1764
		s.Obj[i] = uint32(cont + 2*cont + g(609.3, 4000) + g(1120.3, 1500) + g(1238.1, 600) + g(1764.5, 1500))
	}
	p := s.findPeaks()
	for _, x := range p {
		if x.Group == "Cobalto-60" || x.Group == "Amerício-241" {
			t.Fatalf("pico errado: %+v", p)
		}
	}
	found := false
	for _, x := range p {
		if x.Group == "Rádio-226 (série do urânio)" {
			found = true
		}
	}
	if !found {
		t.Fatalf("não achou o rádio: %+v", p)
	}
}

// Dados reais de 14/11: numa pedra de urânio o pico de 583-609 keV saía
// como "Tório-232". Sem 911 nem 2.614 keV, não é tório; com o 2.614, é.
func TestFindPeaksThoriumNeedsConfirm(t *testing.T) {
	const n = 1024
	mk := func(withTh bool) *MeasureSpectrum {
		s := &MeasureSpectrum{A: [3]float64{5.43, 2.351, 0.000381}, BgSec: 1000, ObjSec: 1000, Bg: make([]uint32, n), Obj: make([]uint32, n)}
		for i := 0; i < n; i++ {
			e := s.energy(i)
			cont := 400 * math.Exp(-e/300)
			s.Bg[i] = uint32(cont)
			g := func(pe, c float64) float64 {
				sg := specFWHM(pe) / 2.355
				return c * 2.35 / (sg * math.Sqrt(2*math.Pi)) * math.Exp(-math.Pow((e-pe)/sg, 2)/2)
			}
			// o pico largo entre 583 e 609 puxado para o lado do Tl-208
			v := cont + 2*cont + g(583.2, 4000) + g(351.9, 3000)
			if withTh {
				v += g(2614.5, 1500) + g(911.2, 1500)
			}
			s.Obj[i] = uint32(v)
		}
		return s
	}
	for _, x := range mk(false).findPeaks() {
		if x.Group == "Tório-232 (série)" {
			t.Fatalf("tório sem 911 nem 2.614: %+v", x)
		}
	}
	th := false
	for _, x := range mk(true).findPeaks() {
		if x.Group == "Tório-232 (série)" {
			th = true
		}
	}
	if !th {
		t.Fatal("com 2.614 keV, o tório tem que aparecer")
	}
}

// 2.102.1: graduação dos picos da tela do espectro (mesmo critério do celular)
func TestGradePeaks(t *testing.T) {
	ps := []SpecPeak{
		{Group: "Urânio-235", Nuclide: "U-235", KeV: 185.7, Net: 50, Cont: 100, Sigma: 12},
		{Group: "Rádio-226 (série do urânio)", Nuclide: "Bi-214", KeV: 609.3, Net: 40, Cont: 100, Sigma: 11},
		{Group: "Césio-137", Nuclide: "Cs-137", KeV: 661.7, Net: 5, Cont: 100, Sigma: 8},
		{Group: "Potássio-40", Nuclide: "K-40", KeV: 1460.8, Net: 70, Cont: 100, Sigma: 15},
		{Group: "Amerício-241", Nuclide: "Am-241", KeV: 59.5, Net: 20, Cont: 100, Sigma: 6},
		{Group: "Cobalto-60", Nuclide: "Co-60", KeV: 1173.2, Net: 40, Cont: 100, Sigma: 3},
	}
	live := gradePeaks(append([]SpecPeak(nil), ps...), true)
	got := map[string]string{}
	for _, p := range live {
		got[p.Nuclide] = p.Level
		if p.KeV == 185.7 && p.Group != "Rádio-226 (série do urânio)" {
			t.Errorf("186 keV deveria ir para o Ra-226, veio %q", p.Group)
		}
	}
	want := map[string]string{"Ra-226": "forte", "Bi-214": "forte", "K-40": "forte", "Am-241": "provável", "Co-60": "fraco"}
	for k, v := range want {
		if got[k] != v {
			t.Errorf("%s: nível %q, queria %q", k, got[k], v)
		}
	}
	if _, ok := got["Cs-137"]; ok {
		t.Error("Cs-137 sozinho com 5% ao vivo deveria sair")
	}
	if len(live) != 5 || live[len(live)-1].Level != "fraco" || live[0].Level != "forte" {
		t.Errorf("ordem errada: %+v", live)
	}
	acc := gradePeaks(append([]SpecPeak(nil), ps...), false)
	found := false
	for _, p := range acc {
		if p.Nuclide == "Cs-137" {
			found = true
			if p.Level != "fraco" {
				t.Errorf("Cs-137 no acumulado: %q", p.Level)
			}
		}
	}
	if !found {
		t.Error("no acumulado a linha sozinha fica (só ao vivo sai)")
	}
}
