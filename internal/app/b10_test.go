package app

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"

	"radlog/internal/store"
)

// B10: o dia dos gráficos de 1 ano começa à meia-noite local.
func TestTruncLocal(t *testing.T) {
	sp := time.FixedZone("BRT", -3*3600)
	at := time.Date(2026, 11, 13, 1, 30, 0, 0, sp)
	if got := truncLocal(at, 24*time.Hour); !got.Equal(time.Date(2026, 11, 13, 0, 0, 0, 0, sp)) {
		t.Fatalf("dia: %v", got)
	}
	if got := truncLocal(at, 6*time.Hour); !got.Equal(time.Date(2026, 11, 13, 0, 0, 0, 0, sp)) {
		t.Fatalf("6 h: %v", got)
	}
	if got := ceilTo(at, 24*time.Hour); !got.Equal(time.Date(2026, 11, 14, 0, 0, 0, 0, sp)) {
		t.Fatalf("teto: %v", got)
	}
	if got := truncLocal(at, 5*time.Minute); !got.Equal(at) {
		t.Fatalf("5 min: %v", got)
	}
}

// D2: só as 5 cópias mais novas da memória do contador ficam.
func TestPruneHistoryCopies(t *testing.T) {
	dir := t.TempDir()
	for d := 1; d <= 8; d++ {
		_ = os.WriteFile(filepath.Join(dir, fmt.Sprintf("historico-2026-11-%02d_120000.bin", d)), []byte("x"), 0o644)
	}
	_ = os.WriteFile(filepath.Join(dir, "outro.bin"), []byte("x"), 0o644)
	if n := pruneHistoryCopies(dir, historyKeep); n != 3 {
		t.Fatalf("apagou %d", n)
	}
	left, _ := filepath.Glob(filepath.Join(dir, "historico-*.bin"))
	if len(left) != 5 || filepath.Base(left[0]) != "historico-2026-11-04_120000.bin" {
		t.Fatalf("ficaram: %v", left)
	}
	if _, err := os.Stat(filepath.Join(dir, "outro.bin")); err != nil {
		t.Fatal("apagou outro arquivo")
	}
}

// QA 13/11: minutos de uma sessão na pasta temporária voltam.
func TestRecoverTempReadings(t *testing.T) {
	a := newTestApp(t)
	tmp := t.TempDir()
	ts, _ := store.Open(tmp)
	now := time.Now().Add(-time.Hour).Truncate(time.Minute)
	_ = ts.Add(store.Point{T: now, CPM: 21, Src: store.SrcLive})
	_ = ts.Add(store.Point{T: now.Add(time.Minute), CPM: 22, Src: store.SrcLive})
	a.recoverTempReadings(a.store, tmp)
	pts, _ := a.store.Range(now.Add(-time.Minute), now.Add(time.Hour))
	if len(pts) != 2 {
		t.Fatalf("voltaram %d", len(pts))
	}
	if left, _ := filepath.Glob(filepath.Join(tmp, "leituras", "*.csv")); len(left) != 0 {
		t.Fatalf("ficou na temporária: %v", left)
	}
}
