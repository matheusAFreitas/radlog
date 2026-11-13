// Package photometa tira os metadados das fotos anexadas (localização do
// celular, modelo, data, miniatura, XMP), mantendo a imagem e a rotação.
// Auditoria 2.76.0, S2: a localização não pode sair do PC num backup ou
// na nuvem.
package photometa

import (
	"bytes"
	"encoding/binary"
	"errors"
	"strings"
)

// ErrFormat: o arquivo diz ser uma foto, mas não deu para entender.
var ErrFormat = errors.New("formato de imagem não reconhecido")

// Handles diz se a extensão é de uma foto que pode levar metadados.
func Handles(ext string) bool {
	switch strings.ToLower(strings.TrimPrefix(ext, ".")) {
	case "jpg", "jpeg", "png", "webp", "heic", "heif", "avif":
		return true
	}
	return false
}

// Strip devolve a imagem sem os metadados. Para extensões que não são de
// foto devolve b como veio.
func Strip(b []byte, ext string) ([]byte, error) {
	switch strings.ToLower(strings.TrimPrefix(ext, ".")) {
	case "jpg", "jpeg":
		return stripJPEG(b)
	case "png":
		return stripPNG(b)
	case "webp":
		return stripWebP(b)
	case "heic", "heif", "avif":
		return stripHEIF(b)
	}
	return b, nil
}

// ---------- JPEG ----------

func stripJPEG(b []byte) ([]byte, error) {
	if len(b) < 4 || b[0] != 0xFF || b[1] != 0xD8 {
		return nil, ErrFormat
	}
	var head, rest bytes.Buffer // APP0 (JFIF) primeiro, depois o resto
	orient := 0
	i := 2
	for {
		if i+1 >= len(b) || b[i] != 0xFF {
			return nil, ErrFormat
		}
		for i+1 < len(b) && b[i+1] == 0xFF { // bytes de enchimento
			i++
		}
		if i+1 >= len(b) {
			return nil, ErrFormat
		}
		m := b[i+1]
		if m == 0xD9 { // fim sem imagem
			return nil, ErrFormat
		}
		if m == 0x01 || (m >= 0xD0 && m <= 0xD7) {
			rest.Write(b[i : i+2])
			i += 2
			continue
		}
		if i+4 > len(b) {
			return nil, ErrFormat
		}
		n := int(binary.BigEndian.Uint16(b[i+2:]))
		if n < 2 || i+2+n > len(b) {
			return nil, ErrFormat
		}
		seg, pay := b[i:i+2+n], b[i+4:i+2+n]
		if m == 0xDA { // começo da imagem: vai até o primeiro fim (EOI)
			end, ok := jpegEOI(b, i+2+n)
			if !ok {
				return nil, ErrFormat
			}
			rest.Write(b[i:end])
			break
		}
		switch {
		case m == 0xE0:
			head.Write(seg) // JFIF
		case m == 0xE1:
			if bytes.HasPrefix(pay, []byte("Exif\x00\x00")) && orient == 0 {
				orient = exifOrientation(pay[6:])
			}
		case m == 0xE2:
			if bytes.HasPrefix(pay, []byte("ICC_PROFILE\x00")) {
				rest.Write(seg) // cores
			}
		case m == 0xEE:
			rest.Write(seg) // Adobe: transformação de cor
		case m >= 0xE3 && m <= 0xEF, m == 0xFE:
			// outros APPn (IPTC, fabricante) e comentários: fora
		default:
			rest.Write(seg) // tabelas e cabeçalho da imagem
		}
		i += 2 + n
	}
	var out bytes.Buffer
	out.Write([]byte{0xFF, 0xD8})
	out.Write(head.Bytes())
	if orient > 1 && orient <= 8 {
		out.Write(orientationAPP1(orient))
	}
	out.Write(rest.Bytes())
	return out.Bytes(), nil
}

// jpegEOI acha o fim da imagem (FF D9) a partir de i, pulando os dados
// comprimidos e os segmentos entre as varreduras (JPEG progressivo). O que
// vem depois (imagens extras, rabicho do fabricante) fica de fora.
func jpegEOI(b []byte, i int) (int, bool) {
	for i+1 < len(b) {
		if b[i] != 0xFF {
			i++
			continue
		}
		m := b[i+1]
		switch {
		case m == 0x00 || m == 0xFF || (m >= 0xD0 && m <= 0xD7):
			i++
			if m != 0xFF {
				i++
			}
		case m == 0xD9:
			return i + 2, true
		default: // DHT, SOS etc. entre varreduras
			if i+4 > len(b) {
				return 0, false
			}
			i += 2 + int(binary.BigEndian.Uint16(b[i+2:]))
		}
	}
	return 0, false
}

// exifOrientation lê a etiqueta Orientation (0x0112) do IFD0.
func exifOrientation(t []byte) int {
	if len(t) < 8 {
		return 0
	}
	var bo binary.ByteOrder
	switch string(t[:2]) {
	case "II":
		bo = binary.LittleEndian
	case "MM":
		bo = binary.BigEndian
	default:
		return 0
	}
	off := int(bo.Uint32(t[4:]))
	if off < 8 || off+2 > len(t) {
		return 0
	}
	n := int(bo.Uint16(t[off:]))
	for k := 0; k < n; k++ {
		e := off + 2 + 12*k
		if e+12 > len(t) {
			return 0
		}
		if bo.Uint16(t[e:]) == 0x0112 && bo.Uint16(t[e+2:]) == 3 {
			return int(bo.Uint16(t[e+8:]))
		}
	}
	return 0
}

// orientationAPP1 é um EXIF só com a rotação, para a foto não virar.
func orientationAPP1(o int) []byte {
	var p bytes.Buffer
	p.WriteString("Exif\x00\x00MM\x00\x2A\x00\x00\x00\x08")
	_ = binary.Write(&p, binary.BigEndian, []uint16{1, 0x0112, 3})
	_ = binary.Write(&p, binary.BigEndian, uint32(1))
	_ = binary.Write(&p, binary.BigEndian, []uint16{uint16(o), 0})
	_ = binary.Write(&p, binary.BigEndian, uint32(0))
	seg := []byte{0xFF, 0xE1, 0, 0}
	binary.BigEndian.PutUint16(seg[2:], uint16(p.Len()+2))
	return append(seg, p.Bytes()...)
}

// ---------- PNG ----------

var pngSig = []byte("\x89PNG\r\n\x1a\n")

func stripPNG(b []byte) ([]byte, error) {
	if !bytes.HasPrefix(b, pngSig) {
		return nil, ErrFormat
	}
	out := bytes.NewBuffer(append([]byte(nil), pngSig...))
	i := len(pngSig)
	for {
		if i+12 > len(b) {
			return nil, ErrFormat
		}
		n := int(binary.BigEndian.Uint32(b[i:]))
		if n < 0 || i+12+n > len(b) {
			return nil, ErrFormat
		}
		typ := string(b[i+4 : i+8])
		switch typ {
		case "eXIf", "tEXt", "zTXt", "iTXt", "tIME":
		default:
			out.Write(b[i : i+12+n])
		}
		i += 12 + n
		if typ == "IEND" {
			return out.Bytes(), nil
		}
	}
}

// ---------- WebP ----------

func stripWebP(b []byte) ([]byte, error) {
	if len(b) < 12 || string(b[:4]) != "RIFF" || string(b[8:12]) != "WEBP" {
		return nil, ErrFormat
	}
	var body bytes.Buffer
	body.WriteString("WEBP")
	i := 12
	for i+8 <= len(b) {
		n := int(binary.LittleEndian.Uint32(b[i+4:]))
		end := i + 8 + n + n%2
		if n < 0 || i+8+n > len(b) {
			return nil, ErrFormat
		}
		if end > len(b) {
			end = len(b)
		}
		ch := append([]byte(nil), b[i:end]...)
		switch string(ch[:4]) {
		case "EXIF", "XMP ":
		case "VP8X":
			if n > 0 {
				ch[8] &^= 0x08 | 0x04 // sem EXIF nem XMP
			}
			body.Write(ch)
		default:
			body.Write(ch)
		}
		i = end
	}
	out := []byte("RIFF\x00\x00\x00\x00")
	binary.LittleEndian.PutUint32(out[4:], uint32(body.Len()))
	return append(out, body.Bytes()...), nil
}

// ---------- HEIC / HEIF / AVIF ----------
//
// O EXIF e o XMP são "itens" guardados em algum ponto do arquivo, achados
// pela tabela iloc. Mexer na estrutura obrigaria a refazer os endereços de
// tudo; então o conteúdo desses itens é zerado no lugar. O tamanho fica
// igual e a imagem não muda.

type box struct {
	typ        string
	start, end int // a caixa inteira
	body       int // começo do conteúdo
}

func boxes(b []byte, from, to int) ([]box, error) {
	var out []box
	for i := from; i < to; {
		if i+8 > to {
			return nil, ErrFormat
		}
		size := int64(binary.BigEndian.Uint32(b[i:]))
		typ := string(b[i+4 : i+8])
		body := i + 8
		switch size {
		case 1:
			if i+16 > to {
				return nil, ErrFormat
			}
			size = int64(binary.BigEndian.Uint64(b[i+8:]))
			body = i + 16
		case 0:
			size = int64(to - i)
		}
		if size < int64(body-i) || int64(i)+size > int64(to) {
			return nil, ErrFormat
		}
		out = append(out, box{typ: typ, start: i, end: i + int(size), body: body})
		i += int(size)
	}
	return out, nil
}

func find(bs []box, typ string) (box, bool) {
	for _, x := range bs {
		if x.typ == typ {
			return x, true
		}
	}
	return box{}, false
}

func stripHEIF(in []byte) ([]byte, error) {
	b := append([]byte(nil), in...)
	top, err := boxes(b, 0, len(b))
	if err != nil {
		return nil, err
	}
	if _, ok := find(top, "ftyp"); !ok {
		return nil, ErrFormat
	}
	meta, ok := find(top, "meta")
	if !ok || meta.body+4 > meta.end {
		return nil, ErrFormat
	}
	kids, err := boxes(b, meta.body+4, meta.end) // meta é "full box"
	if err != nil {
		return nil, err
	}
	iinf, ok1 := find(kids, "iinf")
	iloc, ok2 := find(kids, "iloc")
	if !ok1 || !ok2 {
		return nil, ErrFormat
	}
	targets, err := metaItems(b, iinf)
	if err != nil {
		return nil, err
	}
	if len(targets) == 0 {
		return b, nil
	}
	idat, hasIdat := find(kids, "idat")
	r := &reader{b: b, i: iloc.body, end: iloc.end}
	ver := r.u8()
	r.skip(3)
	sz := r.u8()
	offSize, lenSize := int(sz>>4), int(sz&15)
	sz = r.u8()
	baseSize, idxSize := int(sz>>4), int(sz&15)
	var count int
	if ver < 2 {
		count = r.n(2)
	} else {
		count = r.n(4)
	}
	done := 0
	for k := 0; k < count && r.ok(); k++ {
		var id int
		if ver < 2 {
			id = r.n(2)
		} else {
			id = r.n(4)
		}
		method := 0
		if ver == 1 || ver == 2 {
			method = r.n(2) & 15
		}
		dref := r.n(2)
		base := r.n(baseSize)
		ext := r.n(2)
		for e := 0; e < ext && r.ok(); e++ {
			if (ver == 1 || ver == 2) && idxSize > 0 {
				r.skip(idxSize)
			}
			off, ln := r.n(offSize), r.n(lenSize)
			if !targets[id] {
				continue
			}
			start := base + off
			switch {
			case method == 0 && dref == 0:
			case method == 1 && hasIdat:
				start += idat.body
			default:
				return nil, ErrFormat
			}
			if ln == 0 || start < 0 || start+ln > len(b) {
				return nil, ErrFormat
			}
			clear(b[start : start+ln])
			done++
		}
	}
	if !r.ok() || done == 0 {
		return nil, ErrFormat
	}
	return b, nil
}

// metaItems devolve os itens de EXIF e XMP listados em iinf.
func metaItems(b []byte, iinf box) (map[int]bool, error) {
	r := &reader{b: b, i: iinf.body, end: iinf.end}
	ver := r.u8()
	r.skip(3)
	if ver == 0 {
		r.n(2)
	} else {
		r.n(4)
	}
	if !r.ok() {
		return nil, ErrFormat
	}
	kids, err := boxes(b, r.i, iinf.end)
	if err != nil {
		return nil, err
	}
	out := map[int]bool{}
	for _, k := range kids {
		if k.typ != "infe" {
			continue
		}
		e := &reader{b: b, i: k.body, end: k.end}
		v := e.u8()
		e.skip(3)
		if v < 2 {
			continue // sem item_type
		}
		var id int
		if v == 2 {
			id = e.n(2)
		} else {
			id = e.n(4)
		}
		e.n(2) // proteção
		typ := e.str(4)
		e.cstr() // nome
		if !e.ok() {
			return nil, ErrFormat
		}
		switch {
		case typ == "Exif":
			out[id] = true
		case typ == "mime":
			ct := strings.ToLower(e.cstr())
			if strings.Contains(ct, "xmp") || strings.Contains(ct, "rdf") {
				out[id] = true
			}
		}
	}
	return out, nil
}

type reader struct {
	b      []byte
	i, end int
	bad    bool
}

func (r *reader) ok() bool { return !r.bad }

func (r *reader) skip(n int) {
	if r.i+n > r.end {
		r.bad = true
		return
	}
	r.i += n
}

func (r *reader) u8() byte {
	if r.i >= r.end {
		r.bad = true
		return 0
	}
	r.i++
	return r.b[r.i-1]
}

// n lê um inteiro de size bytes (0, 2, 4 ou 8).
func (r *reader) n(size int) int {
	if size == 0 {
		return 0
	}
	if r.i+size > r.end {
		r.bad = true
		return 0
	}
	var v uint64
	for k := 0; k < size; k++ {
		v = v<<8 | uint64(r.b[r.i+k])
	}
	r.i += size
	if v > 1<<40 {
		r.bad = true
		return 0
	}
	return int(v)
}

func (r *reader) str(n int) string {
	if r.i+n > r.end {
		r.bad = true
		return ""
	}
	r.i += n
	return string(r.b[r.i-n : r.i])
}

func (r *reader) cstr() string {
	k := bytes.IndexByte(r.b[r.i:r.end], 0)
	if k < 0 {
		r.i = r.end
		return ""
	}
	s := string(r.b[r.i : r.i+k])
	r.i += k + 1
	return s
}
