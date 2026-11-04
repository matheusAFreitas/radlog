package main

import (
	"bytes"
	"compress/zlib"
	"fmt"
	"strings"
)

// ---------- gerador de PDF mínimo (sem dependências) ----------
//
// Só o que o relatório precisa: páginas A4, texto nas fontes padrão do PDF
// (Helvetica e Courier, que todo leitor tem) com acentos do português
// (WinAnsiEncoding), linhas e retângulos. Coordenadas em pontos, com y
// crescendo para BAIXO a partir do topo da página (mais fácil de montar).

const (
	pdfW = 595.28 // A4
	pdfH = 841.89
)

type pdfFont string

const (
	fSans     pdfFont = "F1" // Helvetica
	fSansBold pdfFont = "F2" // Helvetica-Bold
	fMono     pdfFont = "F3" // Courier
	fMonoBold pdfFont = "F4" // Courier-Bold
)

type rgb struct{ r, g, b float64 }

func hexRGB(h string) rgb {
	var r, g, b int
	fmt.Sscanf(strings.TrimPrefix(h, "#"), "%02x%02x%02x", &r, &g, &b)
	return rgb{float64(r) / 255, float64(g) / 255, float64(b) / 255}
}

type pdfDoc struct {
	pages []*bytes.Buffer
	cur   *bytes.Buffer
}

func newPDF() *pdfDoc {
	d := &pdfDoc{}
	d.addPage()
	return d
}

func (d *pdfDoc) addPage() {
	d.cur = &bytes.Buffer{}
	d.pages = append(d.pages, d.cur)
}

// caracteres que vêm da fonte Symbol (todo leitor de PDF tem): código na
// Symbol e largura (1/1000)
var symChars = map[rune]struct {
	c byte
	w int
}{'µ': {'m', 576}, 'σ': {'s', 603}, '≈': {0xBB, 549}, '→': {0xAE, 987}, 'τ': {'t', 439}, '×': {0xB4, 549}, '≥': {0xB3, 549}, '≤': {0xA3, 549}, '−': {'-', 549}, '±': {0xB1, 549}}

type pdfRun struct {
	sym  bool
	text string
}

func splitSym(s string) []pdfRun {
	var out []pdfRun
	var cur strings.Builder
	curSym := false
	flush := func() {
		if cur.Len() > 0 {
			out = append(out, pdfRun{curSym, cur.String()})
			cur.Reset()
		}
	}
	for _, r := range s {
		_, isSym := symChars[r]
		if r == '±' {
			isSym = false // existe na WinAnsi
		}
		if isSym != curSym {
			flush()
			curSym = isSym
		}
		cur.WriteRune(r)
	}
	flush()
	return out
}

// winAnsi converte UTF-8 para os bytes da WinAnsiEncoding (o que não existe
// nela vira um equivalente simples).
func winAnsi(s string) []byte {
	repl := map[rune]string{'σ': "s", '≈': "~", '→': "->", '−': "-", 'τ': "t", '×': "x", '…': "...", '≥': ">=", '≤': "<="}
	special := map[rune]byte{'€': 0x80, '‚': 0x82, '„': 0x84, '…': 0x85, '–': 0x96, '—': 0x97, '‘': 0x91, '’': 0x92, '“': 0x93, '”': 0x94, '•': 0x95}
	var out []byte
	for _, r := range s {
		if t, ok := repl[r]; ok && r != '…' {
			out = append(out, t...)
			continue
		}
		if b, ok := special[r]; ok {
			out = append(out, b)
			continue
		}
		if r < 256 {
			out = append(out, byte(r))
			continue
		}
		out = append(out, '?')
	}
	return out
}

func pdfEscape(b []byte) string {
	var sb strings.Builder
	for _, c := range b {
		switch c {
		case '(', ')', '\\':
			sb.WriteByte('\\')
			sb.WriteByte(c)
		default:
			sb.WriteByte(c)
		}
	}
	return sb.String()
}

// larguras da Helvetica (1/1000 do tamanho), ASCII 32..126
var helvW = [...]int{278, 278, 355, 556, 556, 889, 667, 191, 333, 333, 389, 584, 278, 333, 278, 278,
	556, 556, 556, 556, 556, 556, 556, 556, 556, 556, 278, 278, 584, 584, 584, 556,
	1015, 667, 667, 722, 722, 667, 611, 778, 722, 278, 500, 667, 556, 833, 722, 778,
	667, 778, 722, 667, 611, 722, 667, 944, 667, 667, 611, 278, 278, 278, 469, 556,
	333, 556, 556, 500, 556, 556, 278, 556, 556, 222, 222, 500, 222, 833, 556, 556,
	556, 556, 333, 500, 278, 556, 500, 722, 500, 500, 500, 334, 260, 334, 584}

// letra base dos acentuados (para medir a largura)
func baseLetter(c byte) byte {
	const from = "ÀÁÂÃÄÇÈÉÊËÌÍÎÏÑÒÓÔÕÖÙÚÛÜàáâãäçèéêëìíîïñòóôõöùúûü"
	const to = "AAAAACEEEEIIIINOOOOOUUUUaaaaaceeeeiiiinooooouuuu"
	fr := []rune(from)
	for i, r := range fr {
		if byte(r) == c {
			return to[i]
		}
	}
	return 0
}

func textWidth(s string, f pdfFont, size float64) float64 {
	total := 0.0
	for _, run := range splitSym(s) {
		if run.sym {
			for _, r := range run.text {
				total += float64(symChars[r].w) / 1000 * size
			}
			continue
		}
		total += plainWidth(run.text, f, size)
	}
	return total
}

func plainWidth(s string, f pdfFont, size float64) float64 {
	b := winAnsi(s)
	if f == fMono || f == fMonoBold {
		return float64(len(b)) * 0.6 * size
	}
	w := 0
	for _, c := range b {
		if bl := baseLetter(c); bl != 0 {
			c = bl
		}
		switch {
		case c >= 32 && c <= 126:
			w += helvW[c-32]
		case c == 0xB5: // µ
			w += 556
		case c == 0xB7: // ·
			w += 278
		case c == 0x96:
			w += 556
		case c == 0x97:
			w += 1000
		default:
			w += 556
		}
	}
	k := 1.0
	if f == fSansBold {
		k = 1.06
	}
	return float64(w) / 1000 * size * k
}

func (d *pdfDoc) color(c rgb, fill bool) {
	op := "RG"
	if fill {
		op = "rg"
	}
	fmt.Fprintf(d.cur, "%.3f %.3f %.3f %s\n", c.r, c.g, c.b, op)
}

// text escreve com a linha de base em y (medido do topo).
func (d *pdfDoc) text(x, y float64, f pdfFont, size float64, c rgb, s string) {
	d.color(c, true)
	fmt.Fprintf(d.cur, "BT %.2f %.2f Td", x, pdfH-y)
	for _, run := range splitSym(s) {
		if run.sym {
			var b []byte
			for _, r := range run.text {
				b = append(b, symChars[r].c)
			}
			fmt.Fprintf(d.cur, " /F5 %.1f Tf (%s) Tj", size, pdfEscape(b))
			continue
		}
		fmt.Fprintf(d.cur, " /%s %.1f Tf (%s) Tj", f, size, pdfEscape(winAnsi(run.text)))
	}
	d.cur.WriteString(" ET\n")
}

func (d *pdfDoc) textRight(xr, y float64, f pdfFont, size float64, c rgb, s string) {
	d.text(xr-textWidth(s, f, size), y, f, size, c, s)
}

// fit corta o texto com "..." para caber na largura.
func fit(s string, f pdfFont, size, w float64) string {
	if textWidth(s, f, size) <= w {
		return s
	}
	r := []rune(s)
	for len(r) > 1 && textWidth(string(r)+"...", f, size) > w {
		r = r[:len(r)-1]
	}
	return string(r) + "..."
}

func (d *pdfDoc) line(x1, y1, x2, y2, width float64, c rgb, dash bool) {
	d.color(c, false)
	if dash {
		d.cur.WriteString("[3 3] 0 d\n")
	}
	fmt.Fprintf(d.cur, "%.2f w %.2f %.2f m %.2f %.2f l S\n", width, x1, pdfH-y1, x2, pdfH-y2)
	if dash {
		d.cur.WriteString("[] 0 d\n")
	}
}

func (d *pdfDoc) rect(x, y, w, h float64, fill *rgb, stroke *rgb) {
	if fill != nil {
		d.color(*fill, true)
	}
	if stroke != nil {
		d.color(*stroke, false)
		d.cur.WriteString("0.6 w\n")
	}
	op := "S"
	switch {
	case fill != nil && stroke != nil:
		op = "B"
	case fill != nil:
		op = "f"
	}
	fmt.Fprintf(d.cur, "%.2f %.2f %.2f %.2f re %s\n", x, pdfH-y-h, w, h, op)
}

func (d *pdfDoc) polyline(pts [][2]float64, width float64, c rgb) {
	if len(pts) < 2 {
		return
	}
	d.color(c, false)
	fmt.Fprintf(d.cur, "%.2f w 1 J 1 j %.2f %.2f m\n", width, pts[0][0], pdfH-pts[0][1])
	for _, p := range pts[1:] {
		fmt.Fprintf(d.cur, "%.2f %.2f l\n", p[0], pdfH-p[1])
	}
	d.cur.WriteString("S 0 J 0 j\n")
}

// bytes monta o arquivo PDF.
func (d *pdfDoc) bytes(title string) []byte {
	var out bytes.Buffer
	var offs []int
	obj := func(body string) int {
		offs = append(offs, out.Len())
		n := len(offs)
		fmt.Fprintf(&out, "%d 0 obj\n%s\nendobj\n", n, body)
		return n
	}
	out.WriteString("%PDF-1.4\n%\xe2\xe3\xcf\xd3\n")
	// 1 catálogo, 2 páginas: números fixos, preenchidos já
	obj("<< /Type /Catalog /Pages 2 0 R >>")
	pagesAt := out.Len()
	offs = append(offs, 0) // reserva o 2
	fonts := []string{"Helvetica", "Helvetica-Bold", "Courier", "Courier-Bold"}
	var fontRefs []int
	for _, f := range fonts {
		fontRefs = append(fontRefs, obj(fmt.Sprintf("<< /Type /Font /Subtype /Type1 /BaseFont /%s /Encoding /WinAnsiEncoding >>", f)))
	}
	fontRefs = append(fontRefs, obj("<< /Type /Font /Subtype /Type1 /BaseFont /Symbol >>"))
	res := fmt.Sprintf("<< /Font << /F1 %d 0 R /F2 %d 0 R /F3 %d 0 R /F4 %d 0 R /F5 %d 0 R >> >>", fontRefs[0], fontRefs[1], fontRefs[2], fontRefs[3], fontRefs[4])
	var kids []string
	for _, p := range d.pages {
		var z bytes.Buffer
		w := zlib.NewWriter(&z)
		_, _ = w.Write(p.Bytes())
		w.Close()
		offs = append(offs, out.Len())
		cn := len(offs)
		fmt.Fprintf(&out, "%d 0 obj\n<< /Length %d /Filter /FlateDecode >>\nstream\n", cn, z.Len())
		out.Write(z.Bytes())
		out.WriteString("\nendstream\nendobj\n")
		pn := obj(fmt.Sprintf("<< /Type /Page /Parent 2 0 R /MediaBox [0 0 %.2f %.2f] /Resources %s /Contents %d 0 R >>", pdfW, pdfH, res, cn))
		kids = append(kids, fmt.Sprintf("%d 0 R", pn))
	}
	info := obj(fmt.Sprintf("<< /Title (%s) /Producer (GMC Monitor %s) >>", pdfEscape(winAnsi(title)), AppVersion))
	// objeto 2 no fim (o xref aponta para onde ele estiver)
	offs[1] = out.Len()
	fmt.Fprintf(&out, "2 0 obj\n<< /Type /Pages /Kids [%s] /Count %d >>\nendobj\n", strings.Join(kids, " "), len(kids))
	_ = pagesAt
	xref := out.Len()
	fmt.Fprintf(&out, "xref\n0 %d\n0000000000 65535 f \n", len(offs)+1)
	for _, o := range offs {
		fmt.Fprintf(&out, "%010d 00000 n \n", o)
	}
	fmt.Fprintf(&out, "trailer\n<< /Size %d /Root 1 0 R /Info %d 0 R >>\nstartxref\n%d\n%%%%EOF\n", len(offs)+1, info, xref)
	return out.Bytes()
}
