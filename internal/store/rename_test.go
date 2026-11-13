package store

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestRenameRetry(t *testing.T) {
	dir := t.TempDir()
	a, b := filepath.Join(dir, "a.tmp"), filepath.Join(dir, "b")
	_ = os.WriteFile(a, []byte("x"), 0o644)
	if err := RenameRetry(a, b); err != nil {
		t.Fatal(err)
	}
	t0 := time.Now()
	if err := RenameRetry(a, b); err == nil || time.Since(t0) > 200*time.Millisecond {
		t.Fatalf("sem o temporário devia falhar logo: %v em %v", err, time.Since(t0))
	}
}
