package app

import (
	"testing"
	"time"

	"radlog/internal/radiacode"
)

func TestRcSearch(t *testing.T) {
	a := newTestApp(t)
	now := time.Now()
	// 3 leituras de 2 amostras de 0,5 s
	for i := 0; i < 3; i++ {
		a.rcSearchAdd(radiacode.RawSamples{Period: 500 * time.Millisecond, CPS: []float64{10, 12}, DoseRate: []float64{0.1, 0.12}}, now.Add(time.Duration(i)*time.Second))
	}
	if len(a.rc.search) != 6 || a.rc.search[5].T != now.Add(2*time.Second).UnixMilli() || a.rc.search[4].T != a.rc.search[5].T-500 {
		t.Fatalf("amostras: %+v", a.rc.search)
	}
	// duas leituras no mesmo instante não se sobrepõem
	a.rcSearchAdd(radiacode.RawSamples{Period: 500 * time.Millisecond, CPS: []float64{50, 60}}, now.Add(2*time.Second))
	if a.rc.search[7].T != a.rc.search[5].T+1000 || a.rc.search[6].T != a.rc.search[5].T+500 {
		t.Fatalf("encaixe: %+v", a.rc.search[5:])
	}
	// antigas saem
	a.rcSearchAdd(radiacode.RawSamples{Period: 500 * time.Millisecond, CPS: []float64{1}}, now.Add(10*time.Minute))
	if len(a.rc.search) != 1 {
		t.Fatalf("deveria guardar só os últimos minutos: %d", len(a.rc.search))
	}
	s := a.GetRcSearch(0)
	if s.Live || len(s.Pts) != 1 || !s.Raw {
		t.Fatalf("busca: %+v", s)
	}
	if s := a.GetRcSearch(s.Pts[0].T); len(s.Pts) != 0 {
		t.Fatalf("depois da última não tem nada: %+v", s)
	}
}
