package app

import (
	"math"
	"testing"
	"time"

	"radlog/internal/store"
)

func TestCoverageSpan(t *testing.T) {
	now := time.Date(2026, 11, 7, 18, 0, 0, 0, time.Local)
	first := now.Add(-48 * time.Hour)
	// período inteiro antes do primeiro dado: nada a cobrir
	c := coverageSpan(now.AddDate(0, 0, -10), now.AddDate(0, 0, -5), first, now)
	if c.Hist != 0 || c.Span != 5*1440 || c.Pre != 5*1440 || c.Ongoing {
		t.Fatalf("antes do primeiro dado: %+v", c)
	}
	// período que começa antes do primeiro dado: conta desde ele
	c = coverageSpan(now.AddDate(0, 0, -30), now, first, now)
	if c.Hist != 2*1440 || c.Span != 30*1440 || c.Pre != 28*1440 || !c.HistFrom.Equal(first) {
		t.Fatalf("30 dias com 2 de histórico: %+v", c)
	}
	// período em andamento: só até agora
	day := time.Date(2026, 11, 7, 0, 0, 0, 0, time.Local)
	c = coverageSpan(day, day.AddDate(0, 0, 1), first, now)
	if !c.Ongoing || c.Span != 18*60 || c.Hist != 18*60 || c.Pre != 0 {
		t.Fatalf("hoje: %+v", c)
	}
	// sem nenhum dado
	if c = coverageSpan(day, now, time.Time{}, now); c.Hist != 0 || c.Span != 18*60 {
		t.Fatalf("sem dado: %+v", c)
	}
}

func TestSummaryCoverageSinceFirst(t *testing.T) {
	a := newTestApp(t)
	now := time.Now().Truncate(time.Minute)
	start := now.Add(-48 * time.Hour)
	var pts []store.Point
	for m := start; m.Before(now); m = m.Add(time.Minute) {
		if m.Sub(start) >= 10*time.Hour && m.Sub(start) < 12*time.Hour {
			continue // lacuna de 2 h
		}
		pts = append(pts, store.Point{T: m, CPM: 20, Src: store.SrcDevice})
	}
	if _, err := a.store.Import(pts); err != nil {
		t.Fatal(err)
	}
	s, err := a.GetSummary(now.AddDate(0, 0, -30).UnixMilli(), now.Add(time.Hour).UnixMilli())
	if err != nil {
		t.Fatal(err)
	}
	n := len(pts)
	if s.Minutes != n || !s.Ongoing || s.HistFrom != start.UnixMilli() {
		t.Fatalf("resumo: min %d (quero %d), em andamento %v, desde %v", s.Minutes, n, s.Ongoing, time.UnixMilli(s.HistFrom))
	}
	if math.Abs(s.Coverage-float64(n)/float64(48*60)) > 0.002 || s.CoverageAll > 0.07 || s.PreMinutes < 27*1440 {
		t.Fatalf("cobertura %v / período inteiro %v / antes %d", s.Coverage, s.CoverageAll, s.PreMinutes)
	}
	// a lacuna é só a de 2 h (o tempo antes do primeiro dado não conta)
	if s.GapCount != 1 || s.Gaps[0].Minutes != 120 || s.GapMinutes > 125 {
		t.Fatalf("lacunas: %d, %+v, %d min", s.GapCount, s.Gaps, s.GapMinutes)
	}

	// maturidade: 24 h e 7 dias? 24 h pronta; 7 dias em diante em formação
	m := a.GetMaturity()
	if len(m.GMC) != 5 || m.GMC[0].State != matReady || m.GMC[1].State != matForming || m.GMC[4].State != matForming || len(m.RC) != 0 {
		t.Fatalf("maturidade: %+v", m)
	}
	if w := m.GMC[1]; math.Abs(w.HistDays-2) > 0.01 || w.Coverage < 0.9 || w.CoverageAll > 0.3 {
		t.Fatalf("7 dias: %+v", w)
	}
}

func TestMaturityWaiting(t *testing.T) {
	a := newTestApp(t)
	m := a.GetMaturity()
	for _, w := range m.GMC {
		if w.State != matWaiting || w.Coverage != 0 {
			t.Fatalf("sem dados: %+v", w)
		}
	}
	if m.GMCFirst != 0 {
		t.Fatal("primeiro dado sem dado")
	}
}
