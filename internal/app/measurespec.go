package app

import (
	"math"
	"sort"
)

// ---------- espectro da medição de objeto (Radiacode) ----------
//
// Com o espectro de cada fase dá para ver em que energias o objeto sobe
// acima do fundo. Para cada linha gama conhecida, compara as contagens
// líquidas (objeto − fundo no mesmo tempo) numa janela em volta da energia com
// as das duas janelas vizinhas: um pico de verdade sobe acima da reta entre
// os lados (o espalhamento Compton de linhas mais altas sobe tudo por igual e
// não conta). É uma pista do radionuclídeo, não uma identificação garantida.

// SpecPeak é uma linha gama que apareceu acima do fundo.
type SpecPeak struct {
	Group   string  `json:"group"`   // ex.: Césio-137
	Nuclide string  `json:"nuclide"` // quem emite a linha (ex.: Pb-214)
	KeV     float64 `json:"kev"`
	Net     float64 `json:"net"`   // contagens do pico (acima dos lados)
	Sigma   float64 `json:"sigma"` // pico ÷ incerteza
}

type specLine struct {
	group, nuclide string
	kev            float64
}

// specLines são as linhas procuradas (as mais fortes de cada fonte comum).
var specLines = []specLine{
	{"Amerício-241", "Am-241", 59.5},
	{"Urânio natural (Th-234, Pa-234m)", "Th-234", 63.3},
	{"Urânio natural (Th-234, Pa-234m)", "Th-234", 92.6},
	{"Urânio-235", "U-235", 185.7},
	{"Tório-232 (série)", "Pb-212", 238.6},
	{"Rádio-226 (série do urânio)", "Pb-214", 295.2},
	{"Rádio-226 (série do urânio)", "Pb-214", 351.9},
	{"Tório-232 (série)", "Tl-208", 583.2},
	{"Rádio-226 (série do urânio)", "Bi-214", 609.3},
	{"Césio-137", "Cs-137", 661.7},
	{"Tório-232 (série)", "Ac-228", 911.2},
	{"Urânio natural (Th-234, Pa-234m)", "Pa-234m", 1001.0},
	{"Cobalto-60", "Co-60", 1173.2},
	{"Cobalto-60", "Co-60", 1332.5},
	{"Potássio-40", "K-40", 1460.8},
	{"Rádio-226 (série do urânio)", "Bi-214", 1764.5},
	{"Tório-232 (série)", "Tl-208", 2614.5},
}

const specPeakSigma = 3.0

// specFWHM é a largura dos picos do Radiacode 10x (~9,8% em 662 keV,
// crescendo com √E).
func specFWHM(kev float64) float64 { return 65 * math.Sqrt(math.Max(kev, 1)/661.7) }

// energy é a energia (keV) do centro de um canal.
func (s *MeasureSpectrum) energy(ch int) float64 {
	c := float64(ch) + 0.5
	return s.A[0] + s.A[1]*c + s.A[2]*c*c
}

// findPeaks procura as linhas conhecidas no espectro líquido.
func (s *MeasureSpectrum) findPeaks() []SpecPeak {
	if s == nil || len(s.Obj) == 0 || len(s.Bg) != len(s.Obj) || s.BgSec <= 0 || s.ObjSec <= 0 || s.A[1] <= 0 {
		return nil
	}
	k := s.ObjSec / s.BgSec // fundo no tempo do objeto
	emax := s.energy(len(s.Obj) - 1)
	type win struct{ net, vr, n float64 }
	sum := func(lo, hi float64) win {
		var w win
		for i := range s.Obj {
			if e := s.energy(i); e >= lo && e < hi {
				o, b := float64(s.Obj[i]), float64(s.Bg[i])
				w.net += o - b*k
				w.vr += o + b*k*k
				w.n++
			}
		}
		return w
	}
	var out []SpecPeak
	for _, l := range specLines {
		h := 0.6 * specFWHM(l.kev)
		if l.kev+3*h > emax || l.kev-3*h < s.energy(0) {
			continue
		}
		c, lf, rt := sum(l.kev-h, l.kev+h), sum(l.kev-3*h, l.kev-h), sum(l.kev+h, l.kev+3*h)
		if c.n == 0 || lf.n+rt.n == 0 {
			continue
		}
		f := c.n / (lf.n + rt.n)
		// fundo embaixo do pico: o contínuo cai mais ou menos exponencial com
		// a energia, então a média geométrica dos lados (reta no log) erra
		// menos que a reta; com lado zerado ou negativo, vale a reta
		base := f * (lf.net + rt.net)
		if lf.net > 0 && rt.net > 0 {
			base = c.n * math.Sqrt(lf.net/lf.n*rt.net/rt.n)
		}
		peak := c.net - base
		vr := c.vr + f*f*(lf.vr+rt.vr)
		if vr <= 0 {
			continue
		}
		// além de 3σ, o pico tem que se destacar do contínuo (com muitas
		// contagens, a curvatura do contínuo sozinha já passa de 3σ); embaixo
		// de 100 keV o contínuo é mais torto, então pede mais
		prom := 0.05
		if l.kev < 100 {
			prom = 0.5
		}
		if base > 0 && peak < prom*base {
			continue
		}
		if sg := peak / math.Sqrt(vr); sg >= specPeakSigma {
			out = append(out, SpecPeak{Group: l.group, Nuclide: l.nuclide, KeV: l.kev, Net: peak, Sigma: sg})
		}
	}
	// grupos de duas linhas que sempre vêm juntas (Co-60: 1173 e 1332): uma
	// só é outra coisa (ex.: 1120 e 1238 do Bi-214 em volta de 1173)
	need := map[string]int{"Cobalto-60": 2}
	cnt := map[string]int{}
	for _, p := range out {
		cnt[p.Group]++
	}
	kept0 := out[:0]
	for _, p := range out {
		if n, ok := need[p.Group]; !ok || cnt[p.Group] >= n {
			kept0 = append(kept0, p)
		}
	}
	out = kept0
	// linhas mais perto que a largura do pico (ex.: 583 do Tl-208 e 609 do
	// Bi-214) não dá para separar: fica só a mais forte
	sort.SliceStable(out, func(i, j int) bool { return out[i].Sigma > out[j].Sigma })
	var kept []SpecPeak
	for _, pk := range out {
		ok := true
		for _, k := range kept {
			if math.Abs(k.KeV-pk.KeV) < specFWHM((k.KeV+pk.KeV)/2) {
				ok = false
				break
			}
		}
		if ok {
			kept = append(kept, pk)
		}
	}
	return kept
}

// specBin é uma faixa de energia do gráfico, em contagens/s.
type specBin struct {
	E0, E1  float64
	Obj, Bg float64 // contagens/s
	HasBg   bool
}

// bins junta os canais em faixas de energia da largura de meio pico (mais
// estreitas embaixo, mais largas em cima, onde há poucas contagens), de
// 20 keV até 3 MeV ou o fim do aparelho, para o gráfico.
func (s *MeasureSpectrum) bins() []specBin {
	if s == nil || len(s.Obj) == 0 || s.ObjSec <= 0 || s.A[1] <= 0 {
		return nil
	}
	lo, hi := 20.0, math.Min(3000, s.energy(len(s.Obj)-1))
	if hi <= lo {
		return nil
	}
	hasBg := len(s.Bg) == len(s.Obj) && s.BgSec > 0
	var out []specBin
	for e := lo; e < hi; {
		w := math.Max(specFWHM(e)/2, 2*s.A[1])
		out = append(out, specBin{E0: e, E1: math.Min(e+w, hi), HasBg: hasBg})
		e += w
	}
	j := 0
	for i, c := range s.Obj {
		e := s.energy(i)
		if e < lo || e >= hi {
			continue
		}
		for j < len(out)-1 && e >= out[j].E1 {
			j++
		}
		b := &out[j]
		b.Obj += float64(c) / s.ObjSec
		if hasBg {
			b.Bg += float64(s.Bg[i]) / s.BgSec
		}
	}
	return out
}

// AnalyzeSpectrum procura as linhas conhecidas num espectro com o fundo (a
// tela do espectro ao vivo manda o que juntou e o acumulado do aparelho).
func (a *App) AnalyzeSpectrum(s MeasureSpectrum) []SpecPeak {
	p := s.findPeaks()
	if p == nil {
		p = []SpecPeak{}
	}
	return p
}
