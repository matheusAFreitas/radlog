package app

import (
	"os"
	"path/filepath"
	"testing"
)

func TestOldWebviewDirs(t *testing.T) {
	base := t.TempDir()
	mk := func(p string) { _ = os.MkdirAll(filepath.Join(base, p), 0o755) }
	mk("GMC-Monitor-v1.6.2.exe/EBWebView")
	mk("GMC-Monitor(1).exe/EBWebView")
	mk("GMC Monitor/leituras")       // pasta de dados: nunca
	mk("GMC-Monitor-v1.0.exe/outra") // sem EBWebView: não mexe
	mk("Outro.exe/EBWebView")        // outro programa: não mexe
	got := oldWebviewDirs(base)
	if len(got) != 2 {
		t.Fatalf("esperava 2, veio %v", got)
	}
}
