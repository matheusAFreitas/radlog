package pdf

import (
	"bytes"
	"strings"
	"testing"
	"time"
)

func TestDoc(t *testing.T) {
	d := New()
	d.Text(40, 60, Sans, 12, Hex("#15191C"), "Dose: 1,2 µSv · 3σ ≈ ok")
	d.Line(40, 70, 200, 70, 1, Hex("#DDE3E7"), true)
	d.AddPage()
	d.TextRight(500, 60, Mono, 9, Hex("#000000"), "página 2")
	b := d.Bytes("teste", "radlog")
	if !bytes.HasPrefix(b, []byte("%PDF-1.4")) || !bytes.Contains(b, []byte("/Count 2")) || !bytes.HasSuffix(b, []byte("%%EOF\n")) {
		t.Fatal("PDF inválido")
	}
	if TextWidth("µ", Sans, 10) <= 0 || Fit("texto bem comprido mesmo", Sans, 10, 40) == "texto bem comprido mesmo" {
		t.Fatal("largura/corte")
	}
}

// fitSlow é o Fit de antes (uma letra por vez), como referência.
func fitSlow(s string, f Font, size, w float64) string {
	if rawWidth(s, f, size) <= w {
		return s
	}
	r := []rune(s)
	for len(r) > 1 && rawWidth(string(r)+"...", f, size) > w {
		r = r[:len(r)-1]
	}
	return string(r) + "..."
}

// Fit com busca binária dá o mesmo corte que o de antes.
func TestFitSameAsLinear(t *testing.T) {
	texts := []string{
		"texto bem comprido mesmo",
		"alterado em 03/11 14:20 de 50 para 60 CPM (app) · τ = 120 µs → ligado ≈ 0,3 µSv/h ±1σ",
		"ÀÁÂÃ çãõ — “aspas” … fim",
		"x",
	}
	for _, s := range texts {
		for _, f := range []Font{Sans, SansBold, Mono} {
			for w := 1.0; w < 400; w += 7 {
				if a, b := Fit(s, f, 9, w), fitSlow(s, f, 9, w); a != b {
					t.Fatalf("%q f=%v w=%v: %q × %q", s, f, w, a, b)
				}
			}
		}
	}
}

// Um texto enorme cabe numa linha rápido (antes era quadrático).
func TestFitLongIsFast(t *testing.T) {
	s := strings.Repeat("alterado em 03/11 14:20 de 50 para 60 CPM · ", 2000)
	t0 := time.Now()
	out := Fit(s, Sans, 7.5, 300)
	if d := time.Since(t0); d > time.Second {
		t.Fatalf("Fit levou %v", d)
	}
	if TextWidth(out, Sans, 7.5) > 300 || !strings.HasSuffix(out, "...") {
		t.Fatalf("não coube: %q", out)
	}
}
