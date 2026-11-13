package store

import (
	"testing"
	"time"
)

// o horário lido à mão é igual ao do time.Parse
func TestParseTimeFast(t *testing.T) {
	for _, v := range []string{
		"2026-11-13T05:30:00-03:00", "2026-01-01T00:00:00Z", "2025-12-31T23:59:59+05:30",
		"2024-02-29T12:00:00+00:00", "2026-11-13T05:30:00.5-03:00", "2026-11-13 05:30:00",
		"2026-13-01T00:00:00Z", "2026-11-13T25:00:00-03:00", "x026-11-13T05:30:00-03:00", "",
	} {
		want, err := time.Parse(time.RFC3339, v)
		got, ok := parseTime(v)
		if ok != (err == nil) {
			t.Fatalf("%q: ok=%v, time.Parse err=%v", v, ok, err)
		}
		if ok && !got.Equal(want) {
			t.Fatalf("%q: %v, quer %v", v, got, want)
		}
		if ok && got.Location() != time.Local {
			t.Fatalf("%q: fora do fuso do PC", v)
		}
	}
}

// uma consulta que pega meses antigos e os dias em memória não perde nem
// repete minutos na emenda
func TestRangeAcrossCache(t *testing.T) {
	s, err := Open(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	start := startOfDay(time.Now()).AddDate(0, 0, -40)
	var pts []Point
	for tm := start; tm.Before(time.Now().Add(-time.Hour)); tm = tm.Add(10 * time.Minute) {
		pts = append(pts, Point{T: tm, CPM: 17, Src: SrcLive})
	}
	if _, err := s.Import(pts); err != nil {
		t.Fatal(err)
	}
	got, err := s.Range(start, time.Now())
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != len(pts) {
		t.Fatalf("%d minutos, quer %d", len(got), len(pts))
	}
	for i := range got {
		if !got[i].T.Equal(pts[i].T) {
			t.Fatalf("minuto %d: %v, quer %v", i, got[i].T, pts[i].T)
		}
	}
	// só a parte antiga e só a nova
	cut := s.cacheMin
	old, _ := s.Range(start, cut)
	recent, _ := s.Range(cut, time.Now())
	if len(old)+len(recent) != len(pts) || len(old) == 0 || len(recent) == 0 {
		t.Fatalf("antigo %d + recente %d, quer %d", len(old), len(recent), len(pts))
	}
}
