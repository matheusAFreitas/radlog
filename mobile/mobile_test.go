package mobile

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"regexp"
	"strings"
	"testing"
	"time"
)

type fake struct{ n int }

type pair struct {
	A string `json:"a"`
	B int    `json:"b"`
}

func (f *fake) Nothing()                      { f.n++ }
func (f *fake) Sum(a, b int) int              { return a + b }
func (f *fake) Make(s string) (pair, error)   { return pair{s, len(s)}, nil }
func (f *fake) Fail() error                   { return errors.New("não deu") }
func (f *fake) Struct(p pair) (string, error) { return p.A + "/" + string(rune('0'+p.B)), nil }
func (f *fake) StartMobile()                  {}

func got(t *testing.T, s string) reply {
	t.Helper()
	var r reply
	if err := json.Unmarshal([]byte(s), &r); err != nil {
		t.Fatalf("%q: %v", s, err)
	}
	return r
}

func TestCall(t *testing.T) {
	f := &fake{}
	v := reflect.ValueOf(f)
	if r := got(t, call(v, "Sum", "[2,3]")); r.E != "" || r.R.(float64) != 5 {
		t.Fatalf("Sum: %+v", r)
	}
	if r := got(t, call(v, "Make", `["abc"]`)); r.E != "" || r.R.(map[string]any)["b"].(float64) != 3 {
		t.Fatalf("Make: %+v", r)
	}
	if r := got(t, call(v, "Struct", `[{"a":"x","b":2}]`)); r.R != "x/2" {
		t.Fatalf("Struct: %+v", r)
	}
	if r := got(t, call(v, "Fail", "[]")); r.E != "não deu" {
		t.Fatalf("Fail: %+v", r)
	}
	if r := got(t, call(v, "Nothing", "")); r.E != "" || f.n != 1 {
		t.Fatalf("Nothing: %+v %d", r, f.n)
	}
	for _, c := range [][2]string{{"Sum", "[1]"}, {"Sum", `["a",1]`}, {"Nope", "[]"}, {"StartMobile", "[]"}, {"Sum", "{"}} {
		if r := got(t, call(v, c[0], c[1])); r.E == "" {
			t.Fatalf("%s %s passou: %+v", c[0], c[1], r)
		}
	}
	if !strings.Contains(Call("GetRadiacode", "[]"), "ainda não ligou") {
		t.Fatal("Call sem motor")
	}
}

type fakeHost struct{ events chan string }

func (h *fakeHost) Event(name, data string) {
	select {
	case h.events <- name:
	default:
	}
}
func (h *fakeHost) OpenURL(string)              {}
func (h *fakeHost) BLEScan(int) (string, error) { return "[]", nil }
func (h *fakeHost) BLEConnect(string) error     { return errors.New("sem rádio") }
func (h *fakeHost) BLEWrite([]byte) error       { return nil }
func (h *fakeHost) BLEClose()                   {}
func (h *fakeHost) BLERssi() int                { return 0 }

// O motor sobe no modo celular e responde aos métodos da tela.
func TestStartEngine(t *testing.T) {
	h := &fakeHost{events: make(chan string, 64)}
	if err := Start(t.TempDir(), "Teste", h); err != nil {
		t.Fatal(err)
	}
	defer Stop()
	if !Running() || Start("x", "", h) != nil {
		t.Fatal("Running/Start de novo")
	}
	for _, m := range []string{"GetRadiacode", "GetCloud", "GetSettings"} {
		if r := got(t, Call(m, "[]")); r.E != "" || r.R == nil {
			t.Fatalf("%s: %+v", m, r)
		}
	}
	if r := got(t, Call("SetRadiacode", `[false,""]`)); r.E != "" {
		t.Fatalf("SetRadiacode: %+v", r)
	}
	if r := got(t, Call("ScanRadiacodeBLE", "[]")); r.E != "" {
		// no PC o Bluetooth é o do Linux (BlueZ); sem D-Bus dá erro, e tudo bem
		t.Logf("ScanRadiacodeBLE no PC: %s", r.E)
	}
	Flush()
}

func TestSetTimeZone(t *testing.T) {
	old := time.Local
	defer func() { time.Local = old }()
	if err := SetTimeZone("America/Sao_Paulo"); err != nil {
		t.Fatal(err)
	}
	if time.Local.String() != "America/Sao_Paulo" {
		t.Fatalf("fuso %s", time.Local)
	}
	if SetTimeZone("") == nil || SetTimeZone("Nada/Nenhum") == nil {
		t.Fatal("fuso inválido passou")
	}
}

// A4 (auditoria 2.79): todo método que a tela do celular e o Kotlin chamam
// está na lista branca, e nada fora dela passa.
func TestMobileMethodsCoverUI(t *testing.T) {
	re := regexp.MustCompile(`call\("([A-Za-z]+)"`)
	files := []string{"../android/app/src/main/assets/www/app.js"}
	kt, _ := filepath.Glob("../android/app/src/main/java/app/radlog/*.kt")
	files = append(files, kt...)
	seen := 0
	for _, f := range files {
		b, err := os.ReadFile(f)
		if err != nil {
			t.Fatal(err)
		}
		for _, m := range re.FindAllSubmatch(b, -1) {
			seen++
			if !allowed(string(m[1])) {
				t.Errorf("%s chama %s, que não está em app.MobileMethods", filepath.Base(f), m[1])
			}
		}
	}
	if seen == 0 {
		t.Fatal("nenhum call(\"X\") achado: o caminho do app.js mudou?")
	}
	for _, m := range []string{"StartMobile", "StopMobile", "MobileFlush", "IsMobile", "CloudReset", "SaveCSV"} {
		if allowed(m) {
			t.Errorf("%s não devia passar", m)
		}
	}
}
