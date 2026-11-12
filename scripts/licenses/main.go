// Gera frontend/dist/licencas.txt: os avisos de licença das bibliotecas que
// vão dentro do .exe e do AppImage (MIT, BSD e afins pedem que o aviso
// acompanhe o programa). A tela mostra o arquivo em Configurações ›
// Atualizações e sobre.
//
// Uso: go run ./scripts/licenses          (regrava o arquivo)
//
//	go run ./scripts/licenses -check   (falha se estiver desatualizado; CI)
package main

import (
	"bytes"
	"flag"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
)

const out = "frontend/dist/licencas.txt"

// os dois builds publicados
var builds = []struct{ goos, tags string }{
	{"linux", "desktop,production,webkit2_41"},
	{"windows", "desktop,production"},
}

func main() {
	check := flag.Bool("check", false, "só confere")
	flag.Parse()
	mods := map[string]string{} // caminho@versão → pasta
	for _, b := range builds {
		cmd := exec.Command("go", "list", "-deps", "-tags", b.tags,
			"-f", "{{with .Module}}{{if not .Main}}{{.Path}}@{{.Version}}\t{{.Dir}}{{end}}{{end}}", ".")
		cgo := "0"
		if b.goos == "linux" {
			cgo = "1" // a janela do Linux (WebKitGTK) é por cgo
		}
		cmd.Env = append(os.Environ(), "GOOS="+b.goos, "CGO_ENABLED="+cgo)
		cmd.Stderr = os.Stderr
		o, err := cmd.Output()
		if err != nil {
			fail("go list (%s): %v", b.goos, err)
		}
		for _, ln := range strings.Split(strings.TrimSpace(string(o)), "\n") {
			if k, dir, ok := strings.Cut(ln, "\t"); ok {
				mods[k] = dir
			}
		}
	}
	names := make([]string, 0, len(mods))
	for k := range mods {
		names = append(names, k)
	}
	sort.Strings(names)

	var w bytes.Buffer
	w.WriteString("RadLog: avisos de licença das bibliotecas de terceiros\n")
	w.WriteString("Third-party license notices\n")
	w.WriteString("(gerado por scripts/licenses; não edite à mão)\n\n")
	root, err := exec.Command("go", "env", "GOROOT").Output()
	if err != nil {
		fail("go env GOROOT: %v", err)
	}
	section(&w, "Go (biblioteca padrão e compilador)", filepath.Join(strings.TrimSpace(string(root)), "LICENSE"))
	for _, k := range names {
		lic := findLicense(mods[k])
		if lic == "" {
			fail("%s: sem arquivo de licença em %s", k, mods[k])
		}
		section(&w, k, lic)
	}
	section(&w, "IBM Plex (fontes da tela)", "frontend/dist/fonts/OFL-IBM-Plex.txt")
	if *check {
		cur, _ := os.ReadFile(out)
		if !bytes.Equal(cur, w.Bytes()) {
			fail("%s está desatualizado: rode go run ./scripts/licenses", out)
		}
		fmt.Printf("licenças ok (%d bibliotecas)\n", len(names))
		return
	}
	if err := os.WriteFile(out, w.Bytes(), 0o644); err != nil {
		fail("%v", err)
	}
	fmt.Printf("%s: %d bibliotecas\n", out, len(names))
}

func section(w *bytes.Buffer, name, path string) {
	b, err := os.ReadFile(path)
	if err != nil {
		fail("%s: %v", name, err)
	}
	fmt.Fprintf(w, "==============================================================================\n%s\n==============================================================================\n\n%s\n\n", name, bytes.TrimSpace(b))
}

func findLicense(dir string) string {
	ents, _ := os.ReadDir(dir)
	for _, pre := range []string{"LICENSE", "LICENCE", "COPYING", "UNLICENSE"} {
		for _, e := range ents {
			if !e.IsDir() && strings.HasPrefix(strings.ToUpper(e.Name()), pre) {
				return filepath.Join(dir, e.Name())
			}
		}
	}
	return ""
}

func fail(f string, a ...any) {
	fmt.Fprintf(os.Stderr, f+"\n", a...)
	os.Exit(1)
}
