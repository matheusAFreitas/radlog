package app

import (
	"math"
	"testing"
	"time"
)

func TestRcHardness(t *testing.T) {
	a := newTestApp(t)
	t0 := time.Now().Add(-6 * time.Hour).Truncate(time.Hour)
	c := make([]uint32, 1024) // A1 = 3 keV por canal
	sec := 100.0
	acc := make([]float64, 1024)
	for i := range acc { // começa desencontrado, para não juntar tudo no mesmo minuto
		acc[i] = math.Mod(float64(i)*0.618034, 1)
	}
	spread := func(lo, hi int, n float64) { // n contagens espalhadas por igual
		for ch := lo; ch < hi; ch++ {
			acc[ch] += n / float64(hi-lo)
			if acc[ch] >= 1 {
				c[ch] += uint32(acc[ch])
				acc[ch] -= float64(uint32(acc[ch]))
			}
		}
	}
	a.rcSpecAdd("RC-1", t0, fakeSpec(sec, c))
	for i := 1; i <= 5*60; i++ {
		mid, hi := uint32(200), uint32(4)
		switch {
		case i > 240 && i <= 270: // tório perto: sobe acima de 1.500 keV
			mid, hi = 300, 12
		case i > 180 && i <= 210: // chuva: só embaixo
			mid = 400
			c[203] += 40 // Bi-214 (609 keV)
		}
		for ch := 60; ch < 500; ch++ { // contínuo (quase nada acima de 1.500 keV)
			c[ch]++
		}
		spread(100, 300, float64(mid))
		spread(520, 900, float64(hi))
		sec += 60
		a.rcSpecAdd("RC-1", t0.Add(time.Duration(i)*time.Minute), fakeSpec(sec, c))
	}
	h, err := a.GetRcHardness(t0.UnixMilli(), t0.Add(5*time.Hour).UnixMilli(), 60)
	if err != nil {
		t.Fatal(err)
	}
	if len(h.CPS) != 300 || h.Ref <= 0 || h.WinSec != 21*60 {
		t.Fatalf("n %d ref %v win %d", len(h.CPS), h.Ref, h.WinSec)
	}
	near := func(v, want, tol float64) bool { return math.Abs(v-want) <= tol }
	if !near(h.Hard[60], 1, 0.08) || h.HardErr[60] <= 0 {
		t.Fatalf("fundo: %v ± %v", h.Hard[60], h.HardErr[60])
	}
	if h.Hard[255] < 1.6 {
		t.Fatalf("tório deveria subir: %v", h.Hard[255])
	}
	if h.Hard[195] > 0.85 {
		t.Fatalf("chuva deveria baixar: %v", h.Hard[195])
	}
	if h.CPS[195] < h.CPS[60]*1.3 || h.CPS[0] <= 0 {
		t.Fatalf("cps %v %v", h.CPS[195], h.CPS[60])
	}
	// com o que parece a chuva
	p := a.GetRcWindowPeaks(t0.Add(185*time.Minute).UnixMilli(), t0.Add(205*time.Minute).UnixMilli())
	ok := false
	for _, x := range p {
		if x.Nuclide == "Bi-214" {
			ok = true
		}
	}
	if !ok {
		t.Fatalf("picos na chuva: %+v", p)
	}
	if p := a.GetRcWindowPeaks(t0.Add(60*time.Minute).UnixMilli(), t0.Add(80*time.Minute).UnixMilli()); len(p) != 0 {
		t.Fatalf("no fundo não deveria achar nada: %+v", p)
	}
	// cache: o segundo pedido não relê nada e dá o mesmo
	h2, _ := a.GetRcHardness(t0.UnixMilli(), t0.Add(5*time.Hour).UnixMilli(), 300)
	if len(h2.CPS) != 60 || !near(h2.Hard[12], 1, 0.08) {
		t.Fatalf("5 min: %v", h2.Hard[12])
	}
}
