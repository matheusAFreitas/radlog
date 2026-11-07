package app

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestLogFile(t *testing.T) {
	a, dir := testApp(t)
	a.logStart()
	a.logf("conexão: abrindo %s", "COM13")
	a.logEvent("Contador conectado")
	b, err := os.ReadFile(filepath.Join(dir, "log.txt"))
	if err != nil {
		t.Fatal(err)
	}
	s := string(b)
	if !strings.Contains(s, "RadLog "+AppVersion) || !strings.Contains(s, "abrindo COM13") || !strings.Contains(s, "evento: Contador conectado") {
		t.Fatalf("log:\n%s", s)
	}
	// passou de 2 MB: vira log-anterior.txt
	big := strings.Repeat("x", logMax+10)
	_ = os.WriteFile(filepath.Join(dir, "log.txt"), []byte(big), 0o644)
	a.logf("depois de girar")
	if _, err := os.Stat(filepath.Join(dir, "log-anterior.txt")); err != nil {
		t.Fatal("não girou o log")
	}
	if b, _ := os.ReadFile(filepath.Join(dir, "log.txt")); !strings.Contains(string(b), "depois de girar") || len(b) > 200 {
		t.Fatalf("log novo: %d bytes", len(b))
	}
}

func TestWithTimeout(t *testing.T) {
	v, err := withTimeout(time.Second, func() (string, error) { return "GMC-800", nil })
	if v != "GMC-800" || err != nil {
		t.Fatal(v, err)
	}
	block := make(chan struct{})
	defer close(block)
	_, err = withTimeout(30*time.Millisecond, func() (int, error) { <-block; return 1, nil })
	if !errors.Is(err, errStuck) {
		t.Fatal(err)
	}
}
