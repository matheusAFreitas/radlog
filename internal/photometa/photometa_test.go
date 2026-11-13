package photometa

import (
	"bytes"
	"encoding/binary"
	"hash/crc32"
	"image"
	"image/color"
	"image/jpeg"
	"image/png"
	"testing"
)

func img() image.Image {
	m := image.NewRGBA(image.Rect(0, 0, 16, 8))
	for x := 0; x < 16; x++ {
		m.Set(x, x%8, color.RGBA{200, 50, 50, 255})
	}
	return m
}

// exifWithGPS é um EXIF com rotação 6 e um texto de "localização".
func exifWithGPS() []byte {
	var t bytes.Buffer
	t.WriteString("II*\x00")
	_ = binary.Write(&t, binary.LittleEndian, uint32(8))
	_ = binary.Write(&t, binary.LittleEndian, uint16(1))
	_ = binary.Write(&t, binary.LittleEndian, []uint16{0x0112, 3})
	_ = binary.Write(&t, binary.LittleEndian, uint32(1))
	_ = binary.Write(&t, binary.LittleEndian, []uint16{6, 0})
	_ = binary.Write(&t, binary.LittleEndian, uint32(0))
	t.WriteString("GPS -23.5505 -46.6333")
	p := append([]byte("Exif\x00\x00"), t.Bytes()...)
	seg := []byte{0xFF, 0xE1, 0, 0}
	binary.BigEndian.PutUint16(seg[2:], uint16(len(p)+2))
	return append(seg, p...)
}

func TestJPEG(t *testing.T) {
	var enc bytes.Buffer
	_ = jpeg.Encode(&enc, img(), nil)
	b := enc.Bytes()
	com := []byte{0xFF, 0xFE, 0, 9, 'G', 'P', 'S', ' ', 'c', 'o', 'm'}
	in := append([]byte{0xFF, 0xD8}, exifWithGPS()...)
	in = append(in, com...)
	in = append(in, b[2:]...)
	in = append(in, []byte("rabicho GPS do fabricante")...)
	out, err := Strip(in, ".JPG")
	if err != nil {
		t.Fatal(err)
	}
	if bytes.Contains(out, []byte("GPS")) {
		t.Fatal("a localização ficou")
	}
	if _, err := jpeg.Decode(bytes.NewReader(out)); err != nil {
		t.Fatalf("a foto estragou: %v", err)
	}
	// a rotação continua
	i := bytes.Index(out, []byte("Exif\x00\x00"))
	if i < 0 || exifOrientation(out[i+6:]) != 6 {
		t.Fatal("perdeu a rotação")
	}
	// sem rotação no original: sem EXIF nenhum
	plain, _ := Strip(b, "jpeg")
	if bytes.Contains(plain, []byte("Exif")) || !bytes.Equal(plain[len(plain)-2:], []byte{0xFF, 0xD9}) {
		t.Fatal("jpeg simples")
	}
	if _, err := Strip([]byte("não é jpeg"), "jpg"); err == nil {
		t.Fatal("aceitou lixo")
	}
}

func pngChunk(typ string, data []byte) []byte {
	c := make([]byte, 8, 12+len(data))
	binary.BigEndian.PutUint32(c, uint32(len(data)))
	copy(c[4:], typ)
	c = append(c, data...)
	return binary.BigEndian.AppendUint32(c, crc32.ChecksumIEEE(c[4:]))
}

func TestPNG(t *testing.T) {
	var enc bytes.Buffer
	_ = png.Encode(&enc, img())
	b := enc.Bytes()
	in := append(append(append([]byte(nil), b[:33]...), pngChunk("tEXt", []byte("GPS\x00-23.55"))...), b[33:]...)
	in = append(in, pngChunk("eXIf", []byte("GPS"))...) // depois do IEND também sai
	out, err := Strip(in, "png")
	if err != nil {
		t.Fatal(err)
	}
	if bytes.Contains(out, []byte("GPS")) {
		t.Fatal("a localização ficou")
	}
	if _, err := png.Decode(bytes.NewReader(out)); err != nil {
		t.Fatalf("a foto estragou: %v", err)
	}
}

func riffChunk(id string, data []byte) []byte {
	c := append([]byte(id), 0, 0, 0, 0)
	binary.LittleEndian.PutUint32(c[4:], uint32(len(data)))
	c = append(c, data...)
	if len(data)%2 == 1 {
		c = append(c, 0)
	}
	return c
}

func TestWebP(t *testing.T) {
	body := []byte("WEBP")
	body = append(body, riffChunk("VP8X", []byte{0x0C, 0, 0, 0, 1, 0, 0, 1, 0, 0})...)
	body = append(body, riffChunk("VP8L", []byte{1, 2, 3})...)
	body = append(body, riffChunk("EXIF", []byte("GPS -23"))...)
	body = append(body, riffChunk("XMP ", []byte("<gps/>"))...)
	in := append([]byte("RIFF\x00\x00\x00\x00"), body...)
	binary.LittleEndian.PutUint32(in[4:], uint32(len(body)))
	out, err := Strip(in, "webp")
	if err != nil {
		t.Fatal(err)
	}
	if bytes.Contains(out, []byte("GPS")) || bytes.Contains(out, []byte("gps")) {
		t.Fatal("a localização ficou")
	}
	if out[20]&0x0C != 0 {
		t.Fatalf("flags do VP8X: %x", out[20])
	}
	if int(binary.LittleEndian.Uint32(out[4:])) != len(out)-8 {
		t.Fatal("tamanho do RIFF")
	}
}

func bmff(typ string, data ...[]byte) []byte {
	body := bytes.Join(data, nil)
	b := make([]byte, 8, 8+len(body))
	binary.BigEndian.PutUint32(b, uint32(8+len(body)))
	copy(b[4:], typ)
	return append(b, body...)
}

func TestHEIF(t *testing.T) {
	payload := []byte("\x00\x00\x00\x06Exif\x00\x00GPS -23.5505")
	img := []byte("pixels")
	ftyp := bmff("ftyp", []byte("heic\x00\x00\x00\x00mif1heic"))
	infe := func(id uint16, typ string) []byte {
		d := []byte{2, 0, 0, 0, 0, 0, 0, 0}
		binary.BigEndian.PutUint16(d[4:], id)
		d = append(d, typ...)
		return bmff("infe", append(d, 0))
	}
	iinf := bmff("iinf", []byte{0, 0, 0, 0, 0, 2}, infe(1, "hvc1"), infe(2, "Exif"))
	// iloc v0, offset 4 bytes, tamanho 4 bytes, sem base; o endereço é
	// preenchido depois que o tamanho do arquivo é conhecido
	ilocBody := func(off1, off2 uint32) []byte {
		d := []byte{0, 0, 0, 0, 0x44, 0x00, 0, 2}
		for _, it := range []struct {
			id       uint16
			off, len uint32
		}{{1, off1, uint32(len(img))}, {2, off2, uint32(len(payload))}} {
			d = binary.BigEndian.AppendUint16(d, it.id)
			d = binary.BigEndian.AppendUint16(d, 0) // data_reference_index
			d = binary.BigEndian.AppendUint16(d, 1)
			d = binary.BigEndian.AppendUint32(d, it.off)
			d = binary.BigEndian.AppendUint32(d, it.len)
		}
		return d
	}
	build := func(o1, o2 uint32) []byte {
		meta := bmff("meta", []byte{0, 0, 0, 0}, iinf, bmff("iloc", ilocBody(o1, o2)))
		return append(append(ftyp, meta...), bmff("mdat", img, payload)...)
	}
	pre := build(0, 0)
	start := uint32(len(pre) - len(img) - len(payload))
	in := build(start, start+uint32(len(img)))
	out, err := Strip(in, "HEIC")
	if err != nil {
		t.Fatal(err)
	}
	if bytes.Contains(out, []byte("GPS")) || !bytes.Contains(out, img) || len(out) != len(in) {
		t.Fatal("HEIC: localização ficou ou imagem mudou")
	}
	if !bytes.Contains(in, []byte("GPS")) {
		t.Fatal("o original foi alterado")
	}
}

func TestOtherFiles(t *testing.T) {
	in := []byte("%PDF qualquer")
	if out, err := Strip(in, "pdf"); err != nil || !bytes.Equal(out, in) {
		t.Fatal("pdf mudou")
	}
	if Handles(".pdf") || !Handles(".HEIC") {
		t.Fatal("Handles")
	}
}
