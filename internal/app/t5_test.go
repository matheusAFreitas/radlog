package app

import (
	"os"
	"path/filepath"
	"reflect"
	"regexp"
	"sort"
	"strings"
	"testing"
)

// T5: toda chamada api.X da tela existe no Go (a Wails liga os métodos
// exportados do App). Uma chamada com nome errado só quebrava na hora.
func TestFrontendCallsExist(t *testing.T) {
	have := map[string]bool{}
	rt := reflect.TypeOf(&App{})
	for i := 0; i < rt.NumMethod(); i++ {
		have[rt.Method(i).Name] = true
	}
	call := regexp.MustCompile(`\bapi\.([A-Z][A-Za-z0-9]*)`)
	files, _ := filepath.Glob(filepath.Join("..", "..", "frontend", "dist", "*.js"))
	if len(files) < 10 {
		t.Fatalf("não achei a tela: %v", files)
	}
	missing := map[string][]string{}
	n := 0
	for _, f := range files {
		if filepath.Base(f) == "demo.js" {
			continue // o demo imita o Go e pode ter coisa a mais
		}
		b, err := os.ReadFile(f)
		if err != nil {
			t.Fatal(err)
		}
		for _, m := range call.FindAllStringSubmatch(string(b), -1) {
			n++
			if !have[m[1]] {
				missing[m[1]] = append(missing[m[1]], filepath.Base(f))
			}
		}
	}
	if n < 100 {
		t.Fatalf("só %d chamadas: o padrão mudou?", n)
	}
	var out []string
	for k, v := range missing {
		out = append(out, k+" ("+strings.Join(v, ", ")+")")
	}
	sort.Strings(out)
	for _, s := range out {
		t.Errorf("a tela chama api.%s, que não existe no Go", s)
	}
}
