package app

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

// B8: toda goroutine do app passa pelo goSafe/goLoop (panics.go), para um
// erro interno não fechar o app. As exceções têm o próprio recover ou só
// devolvem um resultado por canal.
func TestNoBareGoroutines(t *testing.T) {
	allowed := map[string]bool{
		"app.go:go a.poll(d, stop, port)":              true, // recover próprio (lostConnection)
		"applog.go:go func() {":                        true, // withTimeout
		"watchdog.go:go func() {":                      true, // withTimeout
		"panics.go:go func() {":                        true, // o próprio goSafe
		"panics.go:go a.runLoop(name, fn, time.Sleep)": true,
	}
	bare := regexp.MustCompile(`^\s*go\s`)
	files, _ := filepath.Glob("*.go")
	for _, f := range files {
		if strings.HasSuffix(f, "_test.go") {
			continue
		}
		b, err := os.ReadFile(f)
		if err != nil {
			t.Fatal(err)
		}
		for i, line := range strings.Split(string(b), "\n") {
			if !bare.MatchString(line) {
				continue
			}
			if k := f + ":" + strings.TrimSpace(line); !allowed[k] {
				t.Errorf("%s:%d: goroutine sem goSafe: %s", f, i+1, strings.TrimSpace(line))
			}
		}
	}
}
