package app

import (
	"math"
	"strings"
	"testing"
	"time"

	"radlog/internal/store"
)

func distIn(pts [][3]float64) deriveIn {
	// cada ponto: cm, líquido (µSv/h), σ; o último é a medição atual
	in := deriveIn{test: "distancia"}
	for _, p := range pts[:len(pts)-1] {
		in.prev = append(in.prev, MeasureAt{DistCM: p[0], NetUSv: p[1], ErrUSv: p[2], Level: "sim", Seconds: 60})
	}
	l := pts[len(pts)-1]
	in.distCM = l[0]
	in.r = MeasureResult{NetUSv: l[1], NetErr: l[2] * 100, Level: "sim"}
	in.obj = MeasurePhase{CPM: 100, USv: 1, Seconds: 300}
	return in
}

func TestDistFit(t *testing.T) {
	// fonte pontual: k = 100 µSv/h·cm²
	f := distFit(distIn([][3]float64{{5, 4, 0.2}, {10, 1, 0.08}, {20, 0.25, 0.03}, {40, 0.0625, 0.02}}), 0.01)
	if f.Verdict != "seguiu" || math.Abs(f.K-100) > 1e-6 || len(f.Points) != 4 || math.Abs(f.Points[3].Predict-100.0/1600) > 1e-9 {
		t.Fatalf("pontual: %+v", f)
	}
	// espalhada: longe fica bem acima da curva
	f = distFit(distIn([][3]float64{{5, 4, 0.1}, {10, 2.2, 0.05}, {20, 1.0, 0.03}, {40, 0.45, 0.02}}), 0.01)
	if f.Verdict != "devagar" || !strings.Contains(f.Title, "devagar") {
		t.Fatalf("espalhada: %+v", f)
	}
	// longe some no fundo
	f = distFit(distIn([][3]float64{{5, 4, 0.2}, {10, 1, 0.08}, {20, 0.25, 0.03}, {40, 0.03, 0.02}}), 0.01)
	if !f.Points[3].Lost || !strings.Contains(f.Text, "40 cm") {
		t.Fatalf("sumiu: %+v", f)
	}
}

func TestShieldCut(t *testing.T) {
	in := deriveIn{test: "blindagem", prev: []MeasureAt{{NetUSv: 1, ErrUSv: 0.02, Level: "sim", Seconds: 120}},
		r: MeasureResult{NetUSv: 0.7, NetErr: 2, Level: "sim"}, obj: MeasurePhase{CPM: 100, USv: 1, Seconds: 120}}
	s := shieldCut(in, 0.01)
	if !s.Seen || math.Abs(s.Cut-0.3) > 1e-9 || s.CutErr <= 0 || !strings.Contains(s.Title, "30%") {
		t.Fatalf("corte: %+v", s)
	}
	in.r.NetUSv, in.r.NetErr = 0.98, 3
	if s := shieldCut(in, 0.01); s.Seen || !strings.Contains(s.Title, "Não deu") {
		t.Fatalf("sem diferença: %+v", s)
	}
}

func TestMeasureRelErrStop(t *testing.T) {
	a := newTestApp(t)
	a.demo.Active, a.demo.CPM = true, 25
	if _, err := a.StartMeasure(MeasureOpts{Name: "teste", DistCM: 5, Seconds: 300, HomeBg: true, Test: "distancia", RelErr: 0.1}); err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 300 && a.GetMeasure().Phase == "objeto"; i++ {
		a.feedMeasure(0, 60, true) // bem acima do fundo
	}
	st := a.GetMeasure()
	if st.Phase != "pronto" || st.Result.Stop != "precisao" || st.Obj.Seconds >= 300 || st.Test != "distancia" {
		t.Fatalf("parou: %+v %+v", st, st.Result)
	}
	if st.Derived == nil || st.Derived.Dist == nil || len(st.Derived.Dist.Points) != 1 {
		t.Fatalf("derivado: %+v", st.Derived)
	}
}

func TestDoseOrigin(t *testing.T) {
	a := newTestApp(t)
	end := time.Now().Truncate(stripStep)
	from := end.Add(-2 * time.Hour)
	var pts []store.Point
	for i := 0; i < 120; i++ {
		cpm := 20.0
		if i >= 60 && i < 80 {
			cpm = 200
		}
		pts = append(pts, store.Point{T: from.Add(time.Duration(i) * time.Minute), CPM: cpm, Src: store.SrcLive})
	}
	if _, err := a.store.Import(pts); err != nil {
		t.Fatal(err)
	}
	// os dois em casa (sem dosímetro com você): a dose pessoal usa a de casa
	if _, err := a.SavePeriod(Period{From: from.UnixMilli(), To: end.UnixMilli(), GMC: "casa", RC: "casa"}); err != nil {
		t.Fatal(err)
	}
	o, err := a.GetDoseOrigin(1)
	if err != nil {
		t.Fatal(err)
	}
	if o.Minutes != 120 || o.Above <= 0 || len(o.Items) != 1 || o.Items[0].To-o.Items[0].From != 20*60e3 {
		t.Fatalf("origem: %+v", o)
	}
	if math.Abs(o.Total-(o.HomeBg+o.AwayBg+o.Above)) > 1e-9 {
		t.Fatalf("soma: %+v", o)
	}
}
