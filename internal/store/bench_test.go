package store

import (
	"testing"
	"time"
)

// um ano de minutos lido dos arquivos (auditoria P1: era ~0,6 s e 750 MB
// alocados por consulta, segurando a trava da gravação)
func BenchmarkRangeYear(b *testing.B) {
	dir := b.TempDir()
	s, _ := Open(dir)
	end := startOfDay(time.Now()).AddDate(0, 0, -10)
	start := end.AddDate(-1, 0, 0)
	var pts []Point
	for t := start; t.Before(end); t = t.Add(time.Minute) {
		pts = append(pts, Point{T: t, CPM: 17, Src: SrcLive, Has: true, Raw: 17, Corr: 17, TauUs: 0})
	}
	if _, err := s.Import(pts); err != nil {
		b.Fatal(err)
	}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		r, _ := s.Range(start, end)
		if len(r) < 500000 {
			b.Fatal(len(r))
		}
	}
}
