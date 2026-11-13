package app

import (
	"bytes"
	"image"
	"image/jpeg"
	"image/png"
	"os"
	"path/filepath"
	"testing"
)

// S2: a foto anexada vai sem os metadados; um "jpg" que não é foto não entra.
func TestAttachStripsPhoto(t *testing.T) {
	dir := t.TempDir()
	var enc bytes.Buffer
	_ = jpeg.Encode(&enc, image.NewGray(image.Rect(0, 0, 4, 4)), nil)
	b := enc.Bytes()
	com := []byte{0xFF, 0xFE, 0, 8, 'G', 'P', 'S', '!', '!', '!'}
	in := append(append(append([]byte(nil), b[:2]...), com...), b[2:]...)
	src := filepath.Join(dir, "foto.jpg")
	_ = os.WriteFile(src, in, 0o644)
	dst := filepath.Join(dir, "anexo.jpg")
	n, err := copyAttachment(src, dst, ".jpg")
	if err != nil {
		t.Fatal(err)
	}
	got, _ := os.ReadFile(dst)
	if bytes.Contains(got, []byte("GPS")) || int64(len(got)) != n {
		t.Fatal("a localização ficou")
	}
	bad := filepath.Join(dir, "nao.jpg")
	_ = os.WriteFile(bad, []byte("texto"), 0o644)
	if _, err := copyAttachment(bad, filepath.Join(dir, "x.jpg"), ".jpg"); err != errPhotoMeta {
		t.Fatalf("aceitou: %v", err)
	}
	// texto vai como veio
	txt := filepath.Join(dir, "nota.txt")
	_ = os.WriteFile(txt, []byte("GPS"), 0o644)
	if _, err := copyAttachment(txt, filepath.Join(dir, "n.txt"), ".txt"); err != nil {
		t.Fatal(err)
	}
}

// testPhoto é uma foto de verdade, pequena (o anexo passa pelo photometa).
func testPhoto(kind string) []byte {
	var b bytes.Buffer
	m := image.NewGray(image.Rect(0, 0, 4, 4))
	if kind == "png" {
		_ = png.Encode(&b, m)
	} else {
		_ = jpeg.Encode(&b, m, nil)
	}
	return b.Bytes()
}
