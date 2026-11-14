package app

import (
	"math"
	"time"

	"radlog/internal/gmc"
	"radlog/internal/store"
)

// gmcSim dá leituras ao GMC-800 sem o aparelho, pelo mesmo caminho da
// leitura ao vivo (gmcReading), um segundo de cada vez.
type gmcSim struct {
	a *App
	t time.Time
	m gmcMinute
}

// começa meia hora atrás, para os alertas não ficarem no futuro
func newGMCSim(a *App) *gmcSim { return &gmcSim{a: a, t: time.Now().Add(-30 * time.Minute)} }

// feed dá n segundos com o CPM dado.
func (s *gmcSim) feed(cpm float64, n int) {
	for i := 0; i < n; i++ {
		s.t = s.t.Add(time.Second)
		s.a.gmcReading(s.t, uint32(math.Round(cpm)), uint32(math.Round(cpm/60)), &s.m)
	}
}

// fakeGMC finge o GMC-800 conectado (só para poder medir) e grava a última
// hora com o CPM dado (o fundo da casa das medições).
func fakeGMC(t interface{ Fatal(...any) }, a *App, bgCPM float64) {
	a.mu.Lock()
	a.dev = &gmc.Device{}
	a.settings.NoMeasureReport = true
	a.mu.Unlock()
	now := time.Now().Truncate(time.Minute)
	for i := 60; i >= 1; i-- {
		if err := a.store.Add(store.Point{T: now.Add(-time.Duration(i) * time.Minute), CPM: bgCPM, Src: store.SrcLive}); err != nil {
			t.Fatal(err)
		}
	}
}
