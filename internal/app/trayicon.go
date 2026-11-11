package app

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"image"
	"image/color"
	"image/png"
	"math"
	"strings"
)

// ---------- número no ícone da bandeja (2.47) ----------
//
// Com a janela fechada, o ícone perto do relógio vira a leitura em µSv/h, na
// cor do nível (spec ux/bandeja.html). O ícone é desenhado em memória, pixel
// a pixel, em 16, 20, 24 e 32 px num .ico só (o Windows escolhe pela escala
// da tela). A fonte é em bitmap: antisserrilhado em 16 px borra o número.
// Até 3 caracteres; o valor completo fica na dica.

// trayDev é o que o ícone sabe de um aparelho.
type trayDev struct {
	Name     string  // GMC-800, RC102
	Key      string  // gmc, rc
	State    string  // ok, wait (conectando), off (desconectado)
	USv      float64 // µSv/h
	Level    string  // "", atencao, alarme, perigo
	NoCount  bool
	Sat      bool
	SatAbove float64 // RC: limite de saturação (µSv/h); 0 = "o valor real é maior"
	Away     bool    // RC102 fora de casa
}

// trayRank ordena a gravidade para o automático.
func trayRank(d trayDev) int {
	if d.State != "ok" {
		return -1
	}
	switch {
	case d.Sat || d.Level == "perigo":
		return 4
	case d.Level == "alarme" || d.NoCount:
		return 3
	case d.Level == "atencao":
		return 2
	}
	return 1
}

// trayPick escolhe o aparelho do ícone: fixo pela configuração, ou o GMC-800
// (monitor da casa) a menos que o RC102 esteja mais grave; com um só
// conectado, esse.
func trayPick(choice string, g, r trayDev) trayDev {
	switch choice {
	case "gmc":
		return g
	case "radiacode":
		return r
	}
	gOK, rOK := g.State == "ok", r.State == "ok"
	if rOK && !gOK {
		return r
	}
	if gOK && rOK && trayRank(r) > trayRank(g) {
		return r
	}
	return g
}

// trayNum escreve o valor em até 3 caracteres (0,12 → ,12; 1,4; 48; 1k).
func trayNum(v float64, sep string) string {
	switch {
	case v >= 999.5:
		return "1k"
	case v >= 9.95:
		return fmt.Sprint(int(math.Round(v)))
	case v >= 0.995:
		r := math.Round(v*10) / 10
		if r >= 10 {
			return "10"
		}
		i := math.Floor(r)
		return fmt.Sprintf("%d%s%d", int(i), sep, int(math.Round((r-i)*10)))
	}
	c := int(math.Round(v * 100))
	if c < 0 {
		c = 0
	}
	if c >= 100 {
		return "1" + sep + "0"
	}
	return fmt.Sprintf("%s%02d", sep, c)
}

// trayFull é o valor completo da dica.
func trayFull(v float64, sep string) string {
	d := 0
	switch {
	case v < 10:
		d = 2
	case v < 100:
		d = 1
	}
	return strings.Replace(fmt.Sprintf("%.*f", d, v), ".", sep, 1)
}

// trayLine é a linha de um aparelho na dica (em português; T traduz).
func trayLine(d trayDev, sep string) string {
	switch {
	case d.State == "off":
		return d.Name + " desconectado"
	case d.State == "wait":
		return d.Name + " conectando…"
	case d.Sat && d.SatAbove > 0:
		return fmt.Sprintf("%s saturado · acima de %s µSv/h", d.Name, trayFull(d.SatAbove, sep))
	case d.Sat:
		return d.Name + " saturado · o valor real é maior"
	case d.NoCount:
		return d.Name + " sem contagens · confira o aparelho"
	}
	v := trayFull(d.USv, sep)
	switch d.Level {
	case "atencao":
		return fmt.Sprintf("ATENÇÃO no %s · %s µSv/h", d.Name, v)
	case "alarme":
		return fmt.Sprintf("ALARME no %s · %s µSv/h", d.Name, v)
	case "perigo":
		return fmt.Sprintf("PERIGO no %s · %s µSv/h", d.Name, v)
	}
	w := "seguro"
	if d.Away {
		w = "fora de casa"
	}
	return fmt.Sprintf("%s · %s µSv/h · %s", d.Name, v, w)
}

// trayText é o que vai escrito no ícone e o nível (cor do quadradinho).
func trayText(d trayDev, sep string) (txt, lvl string) {
	switch {
	case d.Sat:
		return "MAX", "perigo"
	case d.NoCount:
		return "0", "alarme"
	}
	return trayNum(d.USv, sep), d.Level
}

// ---------- desenho ----------

var (
	trayBg     = color.NRGBA{0x0B, 0x0E, 0x10, 0xFF}
	trayLine_  = color.NRGBA{0x34, 0x40, 0x4A, 0xFF}
	trayAcc    = color.NRGBA{0x6E, 0xE7, 0xA0, 0xFF}
	trayRc     = color.NRGBA{0xF4, 0x72, 0xB6, 0xFF}
	trayAtt    = color.NRGBA{0xF5, 0xB8, 0x4A, 0xFF}
	trayAlarm  = color.NRGBA{0xF2, 0x8C, 0x38, 0xFF}
	trayDanger = color.NRGBA{0x8E, 0x1D, 0x24, 0xFF}
	trayDim    = color.NRGBA{0x8C, 0x96, 0x9E, 0xFF}
	trayDark   = color.NRGBA{0x1C, 0x0F, 0x04, 0xFF}
	trayWhite  = color.NRGBA{0xFF, 0xF4, 0xF4, 0xFF}
)

// fonte 3×5 (16 e 32 px) e 5×7 (20 e 24 px); a vírgula desce uma linha
var trayF3 = map[byte][]string{
	'0': {"111", "101", "101", "101", "111"}, '1': {"010", "110", "010", "010", "111"},
	'2': {"111", "001", "111", "100", "111"}, '3': {"111", "001", "111", "001", "111"},
	'4': {"101", "101", "111", "001", "001"}, '5': {"111", "100", "111", "001", "111"},
	'6': {"111", "100", "111", "101", "111"}, '7': {"111", "001", "010", "010", "010"},
	'8': {"111", "101", "111", "101", "111"}, '9': {"111", "101", "111", "001", "111"},
	'k': {"100", "101", "110", "101", "101"}, 'M': {"101", "111", "111", "101", "101"},
	'A': {"010", "101", "111", "101", "101"}, 'X': {"101", "101", "010", "101", "101"},
	',': {"0", "0", "0", "1", "1", "1"}, '.': {"0", "0", "0", "0", "1"},
}

var trayF5 = map[byte][]string{
	'0': {"01110", "10001", "10011", "10101", "11001", "10001", "01110"},
	'1': {"00100", "01100", "00100", "00100", "00100", "00100", "01110"},
	'2': {"01110", "10001", "00001", "00010", "00100", "01000", "11111"},
	'3': {"11111", "00010", "00100", "00010", "00001", "10001", "01110"},
	'4': {"00010", "00110", "01010", "10010", "11111", "00010", "00010"},
	'5': {"11111", "10000", "11110", "00001", "00001", "10001", "01110"},
	'6': {"00110", "01000", "10000", "11110", "10001", "10001", "01110"},
	'7': {"11111", "00001", "00010", "00100", "01000", "01000", "01000"},
	'8': {"01110", "10001", "10001", "01110", "10001", "10001", "01110"},
	'9': {"01110", "10001", "10001", "01111", "00001", "00010", "01100"},
	'k': {"10000", "10000", "10010", "10100", "11000", "10100", "10010"},
	'M': {"10001", "11011", "10101", "10101", "10001", "10001", "10001"},
	'A': {"01110", "10001", "10001", "11111", "10001", "10001", "10001"},
	'X': {"10001", "10001", "01010", "00100", "01010", "10001", "10001"},
	',': {"00", "00", "00", "00", "00", "11", "01", "10"},
	'.': {"0", "0", "0", "0", "0", "0", "1"},
}

// inRRect diz se o ponto (x, y) está no quadrado de lado n com cantos de raio r.
func inRRect(x, y, n, r float64) bool {
	if x < 0 || y < 0 || x > n || y > n {
		return false
	}
	cx, cy := x, y
	switch {
	case x < r:
		cx = r
	case x > n-r:
		cx = n - r
	}
	switch {
	case y < r:
		cy = r
	case y > n-r:
		cy = n - r
	}
	dx, dy := x-cx, y-cy
	return dx*dx+dy*dy <= r*r
}

// blend mistura c sobre o pixel com cobertura a (0–1).
func blend(img *image.NRGBA, x, y int, c color.NRGBA, a float64) {
	if a <= 0 {
		return
	}
	o := img.NRGBAAt(x, y)
	oa := float64(o.A) / 255
	na := a + oa*(1-a)
	if na <= 0 {
		return
	}
	mix := func(cv, ov uint8) uint8 {
		return uint8(math.Round((float64(cv)*a + float64(ov)*oa*(1-a)) / na))
	}
	img.SetNRGBA(x, y, color.NRGBA{mix(c.R, o.R), mix(c.G, o.G), mix(c.B, o.B), uint8(math.Round(na * 255))})
}

// supersample cobre cada pixel com 4×4 amostras de inside.
func supersample(img *image.NRGBA, n int, c color.NRGBA, inside func(x, y float64) bool) {
	for y := 0; y < n; y++ {
		for x := 0; x < n; x++ {
			k := 0
			for sy := 0; sy < 4; sy++ {
				for sx := 0; sx < 4; sx++ {
					if inside(float64(x)+(float64(sx)+.5)/4, float64(y)+(float64(sy)+.5)/4) {
						k++
					}
				}
			}
			blend(img, x, y, c, float64(k)/16)
		}
	}
}

func trayGlyphs(s string, f map[byte][]string) [][]string {
	var out [][]string
	for i := 0; i < len(s); i++ {
		if g, ok := f[s[i]]; ok {
			out = append(out, g)
		}
	}
	return out
}

// drawTrayText escreve o texto no meio, sem antisserrilhado.
func drawTrayText(img *image.NRGBA, n int, s string, ink color.NRGBA) {
	f, sc := trayF3, 1
	switch {
	case n >= 32:
		sc = 2
	case n >= 20:
		f = trayF5
	}
	gs := trayGlyphs(s, f)
	w := 0
	for i, g := range gs {
		w += len(g[0]) * sc
		if i < len(gs)-1 {
			w += sc
		}
	}
	gh := len(f['0']) * sc
	ox := int(math.Round(float64(n-w) / 2))
	oy := int(math.Round(float64(n-gh) / 2))
	for _, g := range gs {
		for r, row := range g {
			for c := 0; c < len(row); c++ {
				if row[c] != '1' {
					continue
				}
				for dy := 0; dy < sc; dy++ {
					for dx := 0; dx < sc; dx++ {
						x, y := ox+c*sc+dx, oy+r*sc+dy
						if x >= 0 && y >= 0 && x < n && y < n {
							img.SetNRGBA(x, y, ink)
						}
					}
				}
			}
		}
		ox += len(g[0])*sc + sc
	}
}

// drawTrayIcon desenha um tamanho do ícone.
func drawTrayIcon(n int, d trayDev, sep string) *image.NRGBA {
	img := image.NewNRGBA(image.Rect(0, 0, n, n))
	N := float64(n)
	r := 6.0
	switch {
	case n <= 16:
		r = 3
	case n <= 24:
		r = 4
	}
	txt, lvl := trayText(d, sep)
	if d.State != "ok" {
		lvl = ""
	}
	fill := trayBg
	switch lvl {
	case "atencao":
		fill = trayAtt
	case "alarme":
		fill = trayAlarm
	case "perigo":
		fill = trayDanger
	}
	if fill == trayBg {
		// fundo escuro com borda de 1 px, para aparecer na barra escura
		supersample(img, n, trayLine_, func(x, y float64) bool { return inRRect(x, y, N, r) })
		supersample(img, n, trayBg, func(x, y float64) bool { return inRRect(x-1, y-1, N-2, math.Max(0, r-1)) })
	} else {
		supersample(img, n, fill, func(x, y float64) bool { return inRRect(x, y, N, r) })
	}
	switch d.State {
	case "off": // tracinho cinza
		w := int(math.Round(N * .42))
		h := int(math.Max(2, math.Round(N/9)))
		x0, y0 := int(math.Round(float64(n-w)/2)), int(math.Round(float64(n-h)/2))
		for y := y0; y < y0+h; y++ {
			for x := x0; x < x0+w; x++ {
				img.SetNRGBA(x, y, trayDim)
			}
		}
		return img
	case "wait": // anel tracejado âmbar
		lw := math.Max(1.5, N/11)
		rad := N * .3
		dash := N * .16
		per := dash * 1.8
		supersample(img, n, trayAtt, func(x, y float64) bool {
			dx, dy := x-N/2, y-N/2
			dist := math.Hypot(dx, dy)
			if math.Abs(dist-rad) > lw/2 {
				return false
			}
			ang := math.Atan2(dy, dx)
			if ang < 0 {
				ang += 2 * math.Pi
			}
			return math.Mod(ang*rad, per) < dash
		})
		return img
	}
	ink := trayAcc
	switch {
	case lvl == "perigo":
		ink = trayWhite
	case lvl == "atencao" || lvl == "alarme":
		ink = trayDark
	case d.Key == "rc":
		ink = trayRc
	}
	drawTrayText(img, n, txt, ink)
	return img
}

// trayICO junta os tamanhos num .ico (entradas em PNG, aceitas desde o Vista).
func trayICO(d trayDev, sep string) []byte {
	sizes := []int{16, 20, 24, 32}
	var imgs [][]byte
	for _, n := range sizes {
		var b bytes.Buffer
		_ = png.Encode(&b, drawTrayIcon(n, d, sep))
		imgs = append(imgs, b.Bytes())
	}
	var out bytes.Buffer
	w16 := func(v uint16) { _ = binary.Write(&out, binary.LittleEndian, v) }
	w32 := func(v uint32) { _ = binary.Write(&out, binary.LittleEndian, v) }
	w16(0)
	w16(1)
	w16(uint16(len(sizes)))
	off := 6 + 16*len(sizes)
	for i, n := range sizes {
		out.WriteByte(byte(n))
		out.WriteByte(byte(n))
		out.WriteByte(0)
		out.WriteByte(0)
		w16(1)
		w16(32)
		w32(uint32(len(imgs[i])))
		w32(uint32(off))
		off += len(imgs[i])
	}
	for _, b := range imgs {
		out.Write(b)
	}
	return out.Bytes()
}
