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

// T4/B3: no Windows a troca falha enquanto o arquivo está aberto para
// leitura (outra parte do app lendo o mês); o RenameRetry espera ele soltar.
// No Linux a troca passa de primeira; o teste vale nos dois.
func TestRenameRetryWhileOpen(t *testing.T) {
	dir := t.TempDir()
	dst, tmp := filepath.Join(dir, "2026-11.csv"), filepath.Join(dir, "2026-11.csv.tmp")
	_ = os.WriteFile(dst, []byte("velho"), 0o644)
	_ = os.WriteFile(tmp, []byte("novo"), 0o644)
	f, err := os.Open(dst)
	if err != nil {
		t.Fatal(err)
	}
	done := make(chan struct{})
	go func() {
		time.Sleep(200 * time.Millisecond)
		f.Close()
		close(done)
	}()
	if err := RenameRetry(tmp, dst); err != nil {
		t.Fatal(err)
	}
	<-done
	if b, _ := os.ReadFile(dst); string(b) != "novo" {
		t.Fatalf("ficou %q", b)
	}
}
