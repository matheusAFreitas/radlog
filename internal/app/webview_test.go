package app

import (
	"os"
	"path/filepath"
	"testing"
)

func TestOldWebviewDirs(t *testing.T) {
	base := t.TempDir()
	mk := func(p string) { _ = os.MkdirAll(filepath.Join(base, p), 0o755) }
	mk("RadLog-v2.1.0.exe/EBWebView")
	mk("RadLog(1).exe/EBWebView")
	mk("RadLog/leituras")                  // pasta de dados: nunca
	mk("RadLog-v2.0.0.exe/outra")          // sem EBWebView: não mexe
	mk("Outro.exe/EBWebView")              // outro programa: não mexe
	mk("GMC-Monitor-v1.6.2.exe/EBWebView") // 3.0.0: os da 1.x ficam
	got := oldWebviewDirs(base)
	if len(got) != 2 {
		t.Fatalf("esperava 2, veio %v", got)
	}
}
