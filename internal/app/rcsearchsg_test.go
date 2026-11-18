package app

import (
	"math"
	"testing"
	"time"

	"radlog/internal/radiacode"
)

func TestRcSgBin(t *testing.T) {
	c := make([]uint32, 1024)
	c[200] = 10 // 600 keV com 3 keV/canal (centro 601,5)
	c[600] = 7  // 1.800 keV: fora
	b := rcSgBin(c, [3]float64{0, 3, 0})
	if len(b) != rcSgFastBins {
		t.Fatalf("faixas %d", len(b))
	}
	e := 601.5
	k := int(e / rcSgFastEMax * rcSgFastBins)
	sum := 0.0
	for _, v := range b {
		sum += v
	}
	if b[k] != 10 || sum != 10 {
		t.Fatalf("faixa %d = %v, soma %v", k, b[k], sum)
	}
}

func TestRcSearchSg(t *testing.T) {
	a := newTestApp(t)
	// sem aparelho: nada, e não liga a leitura rápida
	if r := a.GetRcSearchSg(0, 120); r.On || len(r.Rows) != 0 {
		t.Fatalf("sem aparelho: %+v", r)
	}
	a.rc.dev = radiacode.NewRaw(&rcRegs{regs: map[uint32]uint32{}})
	now := time.Now()
	c := make([]uint32, 1024)
	sec := 500.0
	a.rcSgFastAdd("RC-1", now.Add(-10*time.Second), fakeSpec(sec, c)) // referência
	for i := 1; i <= 4; i++ {
		c[200] += 20
		sec += 2
		a.rcSgFastAdd("RC-1", now.Add(time.Duration(-10+2*i)*time.Second), fakeSpec(sec, c))
	}
	// buraco (aba fechada): a leitura seguinte só vira referência
	c[200] += 500
	sec += 60
	a.rcSgFastAdd("RC-1", now.Add(60*time.Second), fakeSpec(sec, c))
	r := a.GetRcSearchSg(0, 120)
	if !r.On || len(r.Rows) != 4 || r.Bins != rcSgFastBins || r.EMax != rcSgFastEMax {
		t.Fatalf("colunas %d on %v", len(r.Rows), r.On)
	}
	e := 601.5
	k := int(e / rcSgFastEMax * rcSgFastBins)
	if r.Rows[0].S != 2 || math.Abs(r.Rows[0].C[k]-20) > 1e-9 {
		t.Fatalf("1ª coluna: s %v faixa %v", r.Rows[0].S, r.Rows[0].C[k])
	}
	// só o que veio depois de sinceMs
	if r2 := a.GetRcSearchSg(r.Rows[1].T, 120); len(r2.Rows) != 2 {
		t.Fatalf("depois da 2ª: %d", len(r2.Rows))
	}
	// a tela perguntou: a leitura rápida fica ligada; parou de perguntar, desliga e esquece
	a.rc.sgFast.mu.Lock()
	seen := time.Since(a.rc.sgFast.seen) < rcSgFastSeen
	a.rc.sgFast.seen = time.Time{}
	a.rc.sgFast.mu.Unlock()
	if !seen {
		t.Fatal("deveria estar ligada")
	}
	a.rcSgFastTick(nil, false) // desligada: não lê (dev nil não é usado)
	if len(a.rc.sgFast.rows) != 0 || a.rc.sgFast.prev != nil {
		t.Fatal("aba fechada: deveria esquecer")
	}
}
