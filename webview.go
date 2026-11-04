package main

import (
	"os"
	"path/filepath"
	"strings"
)

// webviewDir é a pasta fixa do WebView2 (cache e localStorage da tela),
// dentro da pasta de dados do app.
func webviewDir() string {
	base, err := os.UserConfigDir()
	if err != nil || base == "" {
		return ""
	}
	return filepath.Join(base, "GMC Monitor", "webview")
}

// oldWebviewDirs acha as pastas que o WebView2 criava em %APPDATA% com o
// nome de cada .exe (GMC-Monitor-v1.6.2.exe, GMC-Monitor(1).exe...).
// Só entram pastas com o nome do app terminando em .exe e com EBWebView
// dentro: é certeza de que são só cache da tela.
func oldWebviewDirs(base string) []string {
	ents, err := os.ReadDir(base)
	if err != nil {
		return nil
	}
	var out []string
	for _, e := range ents {
		n := e.Name()
		low := strings.ToLower(n)
		if !e.IsDir() || !strings.HasPrefix(low, "gmc-monitor") || !strings.HasSuffix(low, ".exe") {
			continue
		}
		dir := filepath.Join(base, n)
		if st, err := os.Stat(filepath.Join(dir, "EBWebView")); err != nil || !st.IsDir() {
			continue
		}
		out = append(out, dir)
	}
	return out
}

// cleanOldWebviews apaga essas pastas antigas. Uma que esteja em uso
// (versão antiga ainda aberta) falha e fica para a próxima vez.
func (a *App) cleanOldWebviews() {
	base, err := os.UserConfigDir()
	if err != nil || base == "" {
		return
	}
	dirs := oldWebviewDirs(base)
	if len(dirs) == 0 {
		return
	}
	ok, fail := 0, 0
	for _, d := range dirs {
		if err := os.RemoveAll(d); err != nil {
			fail++
		} else {
			ok++
		}
	}
	a.logf("limpeza: %d pastas antigas do WebView apagadas em %s (%d ficaram para depois)", ok, base, fail)
}
