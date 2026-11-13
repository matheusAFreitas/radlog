package store

import (
	"math"
	"os"
	"path/filepath"
	"testing"
	"time"
)

// B1: histórico com relógio 3 h adiantado não trava a gravação ao vivo, e o
// que vem do futuro nem entra
func TestLiveAfterFutureHistory(t *testing.T) {
	s, err := Open(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	now := time.Now().Truncate(time.Minute)
	var hist []Point
	for m := -60; m < 180; m++ {
		hist = append(hist, Point{T: now.Add(time.Duration(m) * time.Minute), CPM: 20, Src: "historico"})
	}
	added, err := s.ImportNew(hist)
	if err != nil {
		t.Fatal(err)
	}
	for _, p := range added {
		if p.T.After(now.Add(FutureSlack)) {
			t.Fatalf("minuto do futuro entrou: %v", p.T)
		}
	}
	// um minuto do futuro que já estivesse gravado (versão antiga) também não trava
	s.mu.Lock()
	s.cache = append(s.cache, Point{T: now.Add(3 * time.Hour), CPM: 20, Src: "historico"})
	s.mu.Unlock()
	for m := 1; m <= 5; m++ {
		if err := s.Add(Point{T: now.Add(time.Duration(m) * time.Minute), CPM: 18, Src: SrcLive}); err != nil {
			t.Fatal(err)
		}
	}
	got, _ := s.Range(now.Add(30*time.Second), now.Add(6*time.Minute))
	// now+1 e now+2 o histórico já trouxe (estão dentro da folga de 2 min)
	if len(got) != 5 {
		t.Fatalf("%d de 5 minutos", len(got))
	}
	live := 0
	for _, p := range got {
		if p.Src == SrcLive {
			live++
		}
	}
	if live != 3 {
		t.Fatalf("gravou %d de 3 minutos ao vivo", live)
	}
	// ano errado também fica fora
	if a, _ := s.ImportNew([]Point{{T: time.Date(2009, 1, 1, 0, 0, 0, 0, time.Local), CPM: 1, Src: "historico"}}); len(a) != 0 {
		t.Fatal("2009 entrou")
	}
}

// B4: NaN, infinito e negativo vão para o .ilegivel em vez de virar leitura
func TestInsaneValues(t *testing.T) {
	for _, v := range []float64{math.NaN(), math.Inf(1), -1, MaxCPM} {
		if SaneCPM(v) {
			t.Fatalf("%v aceito", v)
		}
	}
	if !SaneCPM(0) || !SaneCPM(25) || !SaneUSvh(0.12) {
		t.Fatal("valor bom recusado")
	}
	for _, l := range []string{"2026-11-13T05:30:00-03:00,NaN,ao_vivo", "2026-11-13T05:31:00-03:00,+Inf,ao_vivo", "2026-11-13T05:32:00-03:00,-3,ao_vivo"} {
		if _, ok, _ := parseLine(l); ok {
			t.Fatalf("%q aceito", l)
		}
	}
}

// B2: com o arquivo do mês travado, o minuto fica na memória e grava depois
func TestAddPendingWhenLocked(t *testing.T) {
	dir := t.TempDir()
	s, err := Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	now := time.Now().Truncate(time.Minute)
	if err := s.Add(Point{T: now.Add(-3 * time.Minute), CPM: 20, Src: SrcLive}); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(dir, "leituras", now.Add(-3*time.Minute).Format("2006-01")+".csv")
	// "arquivo travado": no lugar dele, uma pasta (falha até como root)
	if err := os.Rename(path, path+".x"); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(path, 0o755); err != nil {
		t.Fatal(err)
	}
	err = s.Add(Point{T: now.Add(-2 * time.Minute), CPM: 21, Src: SrcLive})
	if err == nil || !errorsIs(err, ErrNotSaved) {
		t.Fatalf("esperava ErrNotSaved, veio %v", err)
	}
	if n, _ := s.Pending(); n != 1 {
		t.Fatalf("pendentes %d", n)
	}
	if got, _ := s.Range(now.Add(-150*time.Second), now); len(got) != 1 {
		t.Fatal("o minuto pendente sumiu da tela")
	}
	_ = os.Remove(path)
	_ = os.Rename(path+".x", path)
	if err := s.Add(Point{T: now.Add(-1 * time.Minute), CPM: 22, Src: SrcLive}); err != nil {
		t.Fatal(err)
	}
	if n, _ := s.Pending(); n != 0 {
		t.Fatalf("ainda pendentes: %d", n)
	}
	pts, _, err := readFileBad(path)
	if err != nil || len(pts) != 3 {
		t.Fatalf("arquivo com %d minutos (%v)", len(pts), err)
	}
	for i := 1; i < len(pts); i++ {
		if !pts[i-1].T.Before(pts[i].T) {
			t.Fatal("fora de ordem")
		}
	}
}

func errorsIs(err, target error) bool {
	for err != nil {
		if err == target {
			return true
		}
		u, ok := err.(interface{ Unwrap() error })
		if !ok {
			return false
		}
		err = u.Unwrap()
	}
	return false
}
