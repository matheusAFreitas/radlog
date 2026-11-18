package app

import "testing"

// 2.101.8 (QA, item 119): em "Os dois aparelhos" com o GMC-800 principal, o
// RC102 de 2º aparelho vira "a dose subiu" quando a contagem não muda.
func TestSideRcDoseUp(t *testing.T) {
	a := newTestApp(t)
	m := &MeasureState{Phase: "fundo", Device: "gmc", Other: &MeasureSide{Device: devRC, Role: "objeto"}}
	feed := func(n int, usv float64) {
		for i := 0; i < n; i++ {
			d := 0.002 * float64(i%3-1) // um pouco de ruído
			a.feedSideLocked(m, measFeed{dev: devRC, cpm: 600, counts: 10, usv: usv + d})
		}
	}
	a.mu.Lock()
	defer a.mu.Unlock()
	feed(600, 0.100)
	a.endSidePhaseLocked(m, "fundo", 0, 600_000)
	if m.Other.Bg == nil || m.Other.Bg.ErrUSv <= 0 {
		t.Fatalf("fundo do 2º aparelho sem incerteza da dose: %+v", m.Other.Bg)
	}
	m.Phase = "objeto"
	feed(600, 0.130)
	a.endSidePhaseLocked(m, "objeto", 600_000, 1_200_000)
	r := m.Other.Result
	if r == nil || !r.DoseUp || r.Limit != 0 {
		t.Fatalf("esperava a dose subiu sem limite: %+v", r)
	}
	if h := dualHint(MeasureResult{Sigma: 0}, *r); h == "" {
		t.Fatalf("dualHint vazio")
	}
}
