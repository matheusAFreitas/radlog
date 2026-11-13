package app

import (
	"math"
	"strings"
	"testing"
	"time"

	"radlog/internal/store"
)

func TestHomeBackground(t *testing.T) {
	a := newTestApp(t)
	if hb := a.homeBackgroundGMC(); hb.OK {
		t.Fatalf("sem dados não deveria dar: %+v", hb)
	}
	now := time.Now().Truncate(time.Minute)
	var pts []store.Point
	for i := 59; i >= 1; i-- {
		v := 24.0 + float64(i%3) // 24, 25, 26
		if i == 30 {
			v = 400 // pico de interferência: fica de fora
		}
		pts = append(pts, store.Point{T: now.Add(-time.Duration(i) * time.Minute), CPM: v, Src: "live"})
	}
	if _, err := a.store.Import(pts); err != nil {
		t.Fatal(err)
	}
	hb := a.homeBackgroundGMC()
	if !hb.OK || hb.Minutes != 58 || math.Abs(hb.CPM-25) > 0.1 {
		t.Fatalf("fundo da casa: %+v", hb)
	}
	if want := math.Sqrt(25*58) / 58; hb.ErrCPM < want-1e-9 || hb.ErrCPM > 0.8 {
		t.Fatalf("incerteza %v (Poisson %v)", hb.ErrCPM, want)
	}
}

// automático com o fundo da casa (demo a 25 CPM): um objeto forte para cedo
// com "achou"; um igual ao fundo para com "nada" antes do limite.
func TestMeasureAuto(t *testing.T) {
	a := newTestApp(t)
	a.demo.Active, a.demo.CPM = true, 25
	st, err := a.StartMeasure(MeasureOpts{Name: "pedra", Distance: "encostado", Auto: true, HomeBg: true, MaxSec: 1800, Pct: 20})
	if err != nil {
		t.Fatal(err)
	}
	if st.Phase != "objeto" || st.Bg == nil || st.Bg.Source != "casa" || st.Bg.CPM != 25 || st.Seconds != 1800 {
		t.Fatalf("início: %+v %+v", st, st.Bg)
	}
	// 4 contagens/s = 240 CPM
	for i := 0; i < 120 && a.GetMeasure().Phase == "objeto"; i++ {
		a.feedMeasure(0, 4, true)
	}
	st = a.GetMeasure()
	if st.Phase != "pronto" || st.Result.Stop != "achou" || st.Obj.Seconds != autoMinSec || st.Result.Level != "sim" {
		t.Fatalf("forte: %+v %+v", st, st.Result)
	}

	// igual ao fundo: 25 CPM ≈ 5 contagens a cada 12 s
	if _, err := a.StartMeasurePhase("objeto", 0, ""); err != nil {
		t.Fatal(err)
	}
	sawEta := false
	for i := 0; i < 1800 && a.GetMeasure().Phase == "objeto"; i++ {
		c := uint32(0)
		if i%12 < 5 {
			c = 1
		}
		a.feedMeasure(0, c, true)
		if s := a.GetMeasure(); s.Phase == "objeto" && s.Eta > 0 && s.Live != nil {
			sawEta = true
		}
	}
	st = a.GetMeasure()
	r := st.Result
	if st.Phase != "pronto" || r.Stop != "nada" || r.Limit <= 0 || r.Limit >= 5 || st.Obj.Seconds >= 1800 || !sawEta {
		t.Fatalf("igual ao fundo: %+v %+v (eta visto %v)", st, r, sawEta)
	}
	if !strings.Contains(r.Verdict, "menos que +") || math.Abs(r.Threshold-5) > 1e-9 {
		t.Fatalf("veredito: %q limiar %v", r.Verdict, r.Threshold)
	}
	// ~4–5 min com o fundo da casa (o fundo medido de 5 min levaria ~18)
	if st.Obj.Seconds < 120 || st.Obj.Seconds > 600 {
		t.Fatalf("tempo para decidir: %d s", st.Obj.Seconds)
	}
}

func TestMeasureAutoLimit(t *testing.T) {
	a := newTestApp(t)
	a.demo.Active, a.demo.CPM = true, 25
	if _, err := a.StartMeasure(MeasureOpts{Auto: true, HomeBg: true, MaxSec: 60, Pct: 20}); err != nil {
		t.Fatal(err)
	}
	// +15% do fundo em 60 s: não dá para decidir
	for i := 0; i < 60; i++ {
		c := uint32(0)
		if i%60 < 29 {
			c = 1
		}
		a.feedMeasure(0, c, true)
	}
	st := a.GetMeasure()
	if st.Phase != "pronto" || st.Result.Stop != "limite" || st.Result.Limit <= 0 {
		t.Fatalf("limite: %+v %+v", st, st.Result)
	}
}

func TestDetectionLimitUnequal(t *testing.T) {
	bg := MeasurePhase{Counts: 100, Seconds: 240}
	// tempos iguais: o clássico 2,71 + 4,65·√B
	if got, want := detectionLimitCPM(bg, 240), (2.71+3.29*math.Sqrt(200))/4; math.Abs(got-want) > 1e-9 {
		t.Fatalf("iguais: %v, quero %v", got, want)
	}
	// fundo bem mais longo que o objeto: limite menor que com fundo curto
	long := MeasurePhase{Counts: 1500, Seconds: 3600}
	if detectionLimitCPM(long, 240) >= detectionLimitCPM(bg, 240) {
		t.Fatal("fundo longo deveria baixar o limite")
	}
}

func TestMeasureReportHome(t *testing.T) {
	m := SavedMeasure{Name: "pedra", Time: time.Now().UnixMilli(), Distance: "encostado",
		Bg:     MeasurePhase{Counts: 1500, Seconds: 3600, CPM: 25, ErrCPM: 0.65, Source: "casa", Start: 1, End: 2},
		Obj:    MeasurePhase{Counts: 120, Seconds: 270, CPM: 26.7, ErrCPM: 2.4, Start: 3, End: 4},
		Result: MeasureResult{NetCPM: 1.7, NetErr: 2.5, Level: "nada", Verdict: "igual ao fundo", Stop: "nada", Limit: 4.9, Pct: 20, Threshold: 5}}
	for i := 0; i < 270; i++ {
		m.Samples = append(m.Samples, MeasureSample{T: int64(i) * 1000, Phase: "objeto", CPS: uint32(i % 2)})
	}
	b := renderMeasurePDF(m, "", "x", "y")
	if len(b) < 1000 {
		t.Fatal("PDF vazio")
	}
}
