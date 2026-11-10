package app

import (
	"math"
	"strings"
	"testing"
)

// espectro com o pico do Cs-137 em cima de um fundo plano
func csSpectrum() *MeasureSpectrum {
	const n = 1024
	s := &MeasureSpectrum{A: [3]float64{0, 3, 0}, BgSec: 1000, ObjSec: 100, Bg: make([]uint32, n), Obj: make([]uint32, n)}
	for i := 0; i < n; i++ {
		s.Bg[i] = 1000
		e := s.energy(i)
		g := 3000 / (9.2 * math.Sqrt(2*math.Pi)) * math.Exp(-math.Pow((e-661.7)/3/9.2, 2)/2)
		s.Obj[i] = uint32(100 + 40 + g)
	}
	s.Peaks = s.findPeaks()
	return s
}

func TestIsoGuesses(t *testing.T) {
	g := isoGuesses(csSpectrum())
	if len(g) != 1 || g[0].Group != "Césio-137" || g[0].Lines != 1 || g[0].Of != 1 || g[0].Conf < isoConfOK {
		t.Fatalf("Cs-137: %+v", g)
	}
	// uma linha só de uma série com várias: confiança baixa
	sp := &MeasureSpectrum{A: [3]float64{0, 3, 0}, Obj: make([]uint32, 1024),
		Peaks: []SpecPeak{{Group: "Rádio-226 (série do urânio)", Nuclide: "Bi-214", KeV: 609.3, Sigma: 30}}}
	if g := isoGuesses(sp); len(g) != 1 || g[0].Conf > 0.45 || g[0].Of < 3 {
		t.Fatalf("Ra-226 com uma linha: %+v", g)
	}
}

func measIn(net, bg float64, dist float64, size string) deriveIn {
	// GMC-like: k = 0,01 µSv/h por CPM
	bgPh := MeasurePhase{CPM: bg * 100, USv: bg, ErrCPM: 1}
	obj := MeasurePhase{CPM: (bg + net) * 100, USv: bg + net, ErrCPM: 1}
	r := compareMeasure(bgPh, obj, func(c float64) float64 { return c / 100 })
	return deriveIn{device: devRC, bg: bgPh, obj: obj, r: r, distCM: dist, size: size}
}

func TestDeriveSafeAndBq(t *testing.T) {
	in := measIn(0.4, 0.1, 5, "pequeno")
	in.spec = csSpectrum()
	d := deriveMeasure(in)
	// x = 5·√(0,4 ÷ 0,01) = 31,6 cm
	if !d.Safe.OK || math.Abs(d.Safe.CM-5*math.Sqrt(40)) > 1e-6 || !d.Safe.Approx {
		t.Fatalf("distância segura: %+v", d.Safe)
	}
	// A = 0,4 · 0,05² ÷ 0,089 MBq ≈ 11 kBq
	want := 0.4 * 0.05 * 0.05 / 0.089 * 1e6
	if d.Bq == nil || !d.Bq.OK || math.Abs(d.Bq.Bq-want) > 1 || d.Bq.Lo != d.Bq.Bq/2 || d.Bq.Hi != d.Bq.Bq*2 {
		t.Fatalf("Bq: %+v (queria %v)", d.Bq, want)
	}
	// espalhado: sem Bq, e diz o que faltou
	in.size = "espalhado"
	if d := deriveMeasure(in); d.Bq.OK || !strings.Contains(d.Bq.Missing, "pequeno") {
		t.Fatalf("espalhado: %+v", d.Bq)
	}
	// encostado: sem distância segura nem Bq
	in.size, in.distCM = "pequeno", 0
	if d := deriveMeasure(in); d.Safe.OK || d.Bq.OK || !strings.Contains(d.Bq.Missing, "distância") {
		t.Fatalf("encostado: %+v %+v", d.Safe, d.Bq)
	}
	// GMC: sem Bq nenhum (não tem espectro)
	in.device, in.distCM = "gmc", 5
	if d := deriveMeasure(in); d.Bq != nil || !d.Safe.OK {
		t.Fatalf("GMC: %+v", d)
	}
	// fraco: já na distância medida fica abaixo de 10% do fundo
	w := measIn(0.005, 0.1, 10, "pequeno")
	w.r.Level, w.device = "sim", "gmc"
	if d := deriveMeasure(w); !d.Safe.OK || !d.Safe.Below || d.Safe.CM != 10 {
		t.Fatalf("fraco: %+v", d.Safe)
	}
}

func TestInvSquare(t *testing.T) {
	// fonte pontual: de 5 para 10 cm cai 4×
	in := measIn(0.1, 0.1, 10, "pequeno")
	in.prev = []MeasureAt{{DistCM: 5, NetUSv: 0.4, ErrUSv: 0.01, Level: "sim"}}
	d := deriveMeasure(in)
	if d.InvSq == nil || !d.InvSq.OK || math.Abs(d.InvSq.Power-2) > 1e-6 || d.Safe.Approx {
		t.Fatalf("pontual: %+v %+v", d.InvSq, d.Safe)
	}
	// espalhado: cai só 1,5×
	in.r = compareMeasure(in.bg, MeasurePhase{CPM: (0.1 + 0.4/1.5) * 100, USv: 0.1 + 0.4/1.5, ErrCPM: 1}, func(c float64) float64 { return c / 100 })
	in.obj = MeasurePhase{CPM: (0.1 + 0.4/1.5) * 100, USv: 0.1 + 0.4/1.5, ErrCPM: 1}
	if d := deriveMeasure(in); d.InvSq == nil || d.InvSq.OK || !strings.Contains(d.InvSq.Text, "espalhado") || !d.Safe.Approx {
		t.Fatalf("espalhado: %+v", d.InvSq)
	}
	// espalhado pelas duas distâncias: sem Bq mesmo marcado como pequeno
	in.spec = csSpectrum()
	if d := deriveMeasure(in); d.Bq == nil || d.Bq.OK || !strings.Contains(d.Bq.Missing, "pequeno") {
		t.Fatalf("Bq com objeto espalhado: %+v", d.Bq)
	}
	// mesma distância: não compara
	in.prev[0].DistCM = 10
	if d := deriveMeasure(in); d.InvSq != nil {
		t.Fatalf("mesma distância: %+v", d.InvSq)
	}
}

func TestDistCMFrom(t *testing.T) {
	for in, want := range map[string]float64{"encostado": 0, "5 cm": 5, "5cm": 5, "2,5 cm": 2.5, "1 m": 100, "30": 30, "8 mm": 0.8, "": -1, "perto": -1} {
		if got := distCMFrom(in); math.Abs(got-want) > 1e-9 {
			t.Errorf("%q: %v (queria %v)", in, got, want)
		}
	}
}

func TestMeasureStopAndAgain(t *testing.T) {
	a := newTestApp(t)
	a.demo.Active, a.demo.CPM = true, 25
	if _, err := a.StartMeasure(MeasureOpts{Name: "pedra", Distance: "5 cm", DistCM: -1, Size: "pequeno", Auto: false, Seconds: 600, HomeBg: true}); err != nil {
		t.Fatal(err)
	}
	if st := a.GetMeasure(); st.DistCM != 5 || st.Size != "pequeno" {
		t.Fatalf("opções: %+v", st)
	}
	for i := 0; i < 3; i++ {
		a.feedMeasure(0, 8, true)
	}
	a.StopMeasure()
	// espera o mínimo de segundos antes de parar
	for i := 0; i < 10 && a.GetMeasure().Phase == "objeto"; i++ {
		a.feedMeasure(0, 8, true)
	}
	st := a.GetMeasure()
	if st.Phase != "pronto" || st.Result.Stop != "manual" || st.Obj.Seconds != measStopMinSec || st.Derived == nil || !st.Derived.Safe.OK {
		t.Fatalf("parar: %+v %+v %+v", st, st.Result, st.Derived)
	}
	// outra distância: mesmo fundo, a anterior vai para Prev
	st, err := a.MeasureAgainAt("10 cm", 10)
	if err != nil || st.Phase != "objeto" || len(st.Prev) != 1 || st.Prev[0].DistCM != 5 || st.DistCM != 10 || st.Bg == nil || st.Bg.Source != "casa" {
		t.Fatalf("de novo: %v %+v", err, st)
	}
	for i := 0; i < 600 && a.GetMeasure().Phase == "objeto"; i++ {
		c := uint32(2)
		if i%2 == 0 {
			c = 3
		}
		a.feedMeasure(0, c, true)
	}
	st = a.GetMeasure()
	if st.Phase != "pronto" || st.Derived == nil || st.Derived.InvSq == nil {
		t.Fatalf("segunda distância: %+v %+v", st, st.Derived)
	}
	// só depois de terminar
	a.CancelMeasure()
	if _, err := a.MeasureAgainAt("10 cm", 10); err == nil {
		t.Fatal("deixou medir de novo sem medição")
	}
}
