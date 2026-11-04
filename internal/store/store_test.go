package store

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestAddRangeImportReopen(t *testing.T) {
	dir := t.TempDir()
	s, err := Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	now := time.Now().Truncate(time.Minute)
	for i := 30; i >= 1; i-- {
		if err := s.Add(Point{T: now.Add(-time.Duration(i) * time.Minute), CPM: float64(20 + i%3), Src: SrcLive}); err != nil {
			t.Fatal(err)
		}
	}
	// minuto repetido é ignorado
	_ = s.Add(Point{T: now.Add(-time.Minute), CPM: 999, Src: SrcLive})

	pts, _ := s.Range(now.Add(-time.Hour), now)
	if len(pts) != 30 {
		t.Fatalf("%d pontos", len(pts))
	}

	// importa 40 minutos do contador; 30 já existem
	var imp []Point
	for i := 40; i >= 1; i-- {
		imp = append(imp, Point{T: now.Add(-time.Duration(i) * time.Minute), CPM: 50, Src: SrcDevice})
	}
	added, err := s.Import(imp)
	if err != nil || added != 10 {
		t.Fatalf("importou %d, %v", added, err)
	}
	pts, _ = s.Range(now.Add(-time.Hour), now)
	if len(pts) != 40 || pts[0].Src != SrcDevice || pts[39].Src != SrcLive || pts[39].CPM == 999 {
		t.Fatalf("depois da importação: %d pontos, primeiro %+v, último %+v", len(pts), pts[0], pts[len(pts)-1])
	}

	// reabre do disco
	s2, err := Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	pts2, _ := s2.Range(now.Add(-time.Hour), now)
	if len(pts2) != 40 {
		t.Fatalf("depois de reabrir: %d", len(pts2))
	}

	var buf bytes.Buffer
	if err := s2.ExportCSV(&buf, now.Add(-time.Hour), now, func(c float64) float64 { return c * 0.0065 }); err != nil {
		t.Fatal(err)
	}
	if lines := strings.Count(buf.String(), "\n"); lines != 41 {
		t.Fatalf("csv com %d linhas", lines)
	}
	if ft, ok := s2.FirstTime(); !ok || !ft.Equal(now.Add(-40*time.Minute)) {
		t.Fatalf("FirstTime = %v %v", ft, ok)
	}
}

func TestBrokenLastLineIsIgnored(t *testing.T) {
	dir := t.TempDir()
	s, _ := Open(dir)
	now := time.Now().Truncate(time.Minute)
	_ = s.Add(Point{T: now.Add(-2 * time.Minute), CPM: 20, Src: SrcLive})
	f, _ := os.OpenFile(filepath.Join(dir, "leituras", now.Add(-2*time.Minute).Format("2006-01")+".csv"), os.O_APPEND|os.O_WRONLY, 0)
	f.WriteString("2026-1")
	f.Close()
	s2, err := Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	pts, _ := s2.Range(now.Add(-time.Hour), now)
	if len(pts) != 1 {
		t.Fatalf("%d pontos", len(pts))
	}
	// a próxima gravação não pode grudar na linha quebrada
	_ = s2.Add(Point{T: now.Add(-time.Minute), CPM: 21, Src: SrcLive})
	s3, _ := Open(dir)
	pts, _ = s3.Range(now.Add(-time.Hour), now)
	if len(pts) != 2 || pts[1].CPM != 21 {
		t.Fatalf("depois de gravar de novo: %+v", pts)
	}
}
