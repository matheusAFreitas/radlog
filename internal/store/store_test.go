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
	if err := s2.ExportCSV(&buf, now.Add(-time.Hour), now, func(p Point) (float64, string) { return p.CPM * 0.0065, "v1" }); err != nil {
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

func TestTraceColumns(t *testing.T) {
	dir := t.TempDir()
	s, err := Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	now := time.Now().Truncate(time.Minute)
	// arquivo de antes da 2.1: cabeçalho e linhas de 3 colunas
	old := now.Add(-3 * time.Minute)
	os.WriteFile(s.monthFile(old), []byte("data_hora,cpm,origem\n"+old.Format(time.RFC3339)+",25,ao vivo\n"), 0o644)
	s, _ = Open(dir)
	if err := s.Add(Point{T: now.Add(-2 * time.Minute), CPM: 30, Src: SrcLive, Has: true, Raw: 30, Corr: 30}); err != nil {
		t.Fatal(err)
	}
	if err := s.Add(Point{T: now.Add(-time.Minute), CPM: 5200, Src: SrcLive, Has: true, Raw: 4400, Corr: 5200, TauUs: 200, DTOn: true, Obs: ObsSaturation + "+" + ObsInterference}); err != nil {
		t.Fatal(err)
	}
	pts, _ := s.Range(now.Add(-time.Hour), now.Add(time.Minute))
	if len(pts) != 3 {
		t.Fatalf("pontos: %+v", pts)
	}
	if pts[0].Has || pts[0].RawCPM() != 25 {
		t.Fatalf("antigo: %+v", pts[0])
	}
	if !pts[1].Has || pts[1].DTOn || pts[1].Raw != 30 || pts[1].TauUs != 0 {
		t.Fatalf("sem correção: %+v", pts[1])
	}
	p := pts[2]
	if !p.Has || !p.DTOn || p.Raw != 4400 || p.Corr != 5200 || p.CPM != 5200 || p.TauUs != 200 || !p.HasObs(ObsSaturation) || !p.HasObs(ObsInterference) || p.HasObs(ObsSeconds) {
		t.Fatalf("com correção: %+v", p)
	}
	// reescrever o arquivo (importação) não inventa colunas para o minuto antigo
	if _, err := s.Import([]Point{{T: now.Add(-10 * time.Minute), CPM: 22, Src: SrcDevice, Has: true, Raw: 22, Corr: 22, Obs: ObsSeconds}}); err != nil {
		t.Fatal(err)
	}
	b, _ := os.ReadFile(s.monthFile(now))
	txt := string(b)
	if !strings.Contains(txt, old.Format(time.RFC3339)+",25,ao vivo\n") || !strings.HasPrefix(txt, header+"\n") || !strings.Contains(txt, ",22,contador,22,22,,desligado,seg\n") {
		t.Fatalf("arquivo:\n%s", txt)
	}
	var buf strings.Builder
	s.ExportCSV(&buf, now.Add(-time.Hour), now.Add(time.Minute), func(p Point) (float64, string) { return p.CPM / 100, "v1" })
	if !strings.Contains(buf.String(), "sem registro (antes da 2.1)") || !strings.Contains(buf.String(), ",4400,5200,200,ligado,sat+interf,v1") {
		t.Fatalf("export:\n%s", buf.String())
	}
}

// Uma linha estragada no meio do mês (aspa solta de uma gravação cortada)
// não pode esconder as linhas seguintes (2.70.1).
func TestParseCSVBadLineInTheMiddle(t *testing.T) {
	in := header + "\n" +
		"2026-11-01T00:00:00-03:00,20,ao vivo\n" +
		"2026-11-01T00:01:00-03:00,2\"1,ao vivo\n" +
		"\x00\x00\x00\x00\n" +
		"lixo sem sentido\n" +
		"2026-11-01T00:02:00-03:00,22,ao vivo,22,22,,desligado,sat\n" +
		"2026-11-01T00:03:00-03:00,23,ao vivo"
	pts, bad := parseCSV(strings.NewReader(in))
	if len(pts) != 3 {
		t.Fatalf("pontos = %d, quero 3", len(pts))
	}
	if pts[1].CPM != 22 || pts[1].Obs != "sat" || pts[2].CPM != 23 {
		t.Fatalf("pontos errados: %+v", pts)
	}
	if len(bad) != 2 {
		t.Fatalf("linhas ilegíveis = %d (%q), quero 2", len(bad), bad)
	}
}

// Importar num mês com linha estragada não apaga as linhas boas que vêm
// depois dela, e guarda a estragada num arquivo ao lado.
func TestImportKeepsLinesAfterBadOne(t *testing.T) {
	dir := t.TempDir()
	s, err := Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	day := time.Date(2020, 3, 10, 0, 0, 0, 0, time.Local)
	path := s.monthFile(day)
	var sb strings.Builder
	sb.WriteString(header + "\n")
	sb.WriteString(formatLine(Point{T: day, CPM: 20, Src: SrcLive}) + "\n")
	sb.WriteString("2020-03-10T00:01:00-03:00,2\"1,ao vivo\n")
	sb.WriteString(formatLine(Point{T: day.Add(2 * time.Minute), CPM: 22, Src: SrcLive}) + "\n")
	sb.WriteString(formatLine(Point{T: day.Add(3 * time.Minute), CPM: 23, Src: SrcLive}) + "\n")
	if err := os.WriteFile(path, []byte(sb.String()), 0o644); err != nil {
		t.Fatal(err)
	}
	added, err := s.ImportNew([]Point{{T: day.Add(10 * time.Minute), CPM: 30, Src: SrcDevice}})
	if err != nil || len(added) != 1 {
		t.Fatalf("ImportNew: %v, %d", err, len(added))
	}
	pts, err := readFile(path)
	if err != nil || len(pts) != 4 {
		t.Fatalf("depois de importar: %d pontos (%v), quero 4", len(pts), err)
	}
	b, err := os.ReadFile(path + ".ilegivel")
	if err != nil || !strings.Contains(string(b), `2"1`) {
		t.Fatalf("a linha estragada não foi guardada: %v %q", err, b)
	}
}
