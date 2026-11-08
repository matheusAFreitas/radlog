package app

import (
	"math"
	"testing"
	"time"

	coredose "radlog/internal/core/dose"
	"radlog/internal/gmc"
	"radlog/internal/store"
)

func TestMyDose(t *testing.T) {
	a := newTestApp(t)
	now := time.Now()
	end := now.Truncate(time.Minute)
	// última hora a 20 CPM no GMC (sem RadiaCode)
	var pts []store.Point
	for i := 60; i >= 1; i-- {
		pts = append(pts, store.Point{T: end.Add(-time.Duration(i) * time.Minute), CPM: 20, Src: store.SrcLive})
	}
	if _, err := a.store.Import(pts); err != nil {
		t.Fatal(err)
	}
	md, err := a.myDose(now, "publico")
	if err != nil {
		t.Fatal(err)
	}
	if md.Default != "casa" || md.Personal.HasData || !md.Home.HasData {
		t.Fatalf("aparelho padrão %q, pessoal %v, casa %v", md.Default, md.Personal.HasData, md.Home.HasData)
	}
	if len(md.Home.Periods) != 4 {
		t.Fatalf("%d períodos", len(md.Home.Periods))
	}
	usv := gmc.DefaultCalibration().USv(20)
	cal := coredose.Calendar(now.Truncate(time.Minute).Add(time.Minute))
	for i, p := range md.Home.Periods {
		if p.Key != cal[i].Key {
			t.Fatalf("ordem: %s", p.Key)
		}
		n := 0
		for _, q := range pts {
			if !q.T.Before(cal[i].From) {
				n++
			}
		}
		if p.MeasuredMinutes != n || math.Abs(p.Total-float64(n)*usv/60) > 1e-9 {
			t.Errorf("%s: %d min (quero %d), total %v", p.Key, p.MeasuredMinutes, n, p.Total)
		}
		// fundo = a própria mediana: nada acima
		if math.Abs(p.Above) > 1e-9 || math.Abs(p.Excess) > 1e-9 {
			t.Errorf("%s: acima %v excesso %v", p.Key, p.Above, p.Excess)
		}
	}
	h := md.Home
	if math.Abs(h.YearProjection-usv*24*365.25) > 1e-6 || math.Abs(h.YearBackground-usv*24*365.25) > 1e-6 ||
		math.Abs(h.YearCeiling-h.YearBackground-1000) > 1e-6 {
		t.Fatalf("ano: projeção %v fundo %v teto %v", h.YearProjection, h.YearBackground, h.YearCeiling)
	}
	if md.LimitMSvYear != 1 {
		t.Fatalf("limite %v", md.LimitMSvYear)
	}
}
