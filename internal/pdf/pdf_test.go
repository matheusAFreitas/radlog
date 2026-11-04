package pdf

import (
	"bytes"
	"testing"
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
