package app

import (
	"bytes"
	"image"
	"image/png"
	"net/http"
	"net/http/httptest"
	"testing"
)

// B7: a imagem do radar tem limite de tamanho
func TestRadarImageLimit(t *testing.T) {
	small := new(bytes.Buffer)
	_ = png.Encode(small, image.NewGray(image.Rect(0, 0, 256, 256)))
	big := new(bytes.Buffer)
	_ = png.Encode(big, image.NewGray(image.Rect(0, 0, 5000, 10)))
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/ok.png":
			w.Write(small.Bytes())
		case "/largo.png":
			w.Write(big.Bytes())
		default:
			w.Write(make([]byte, radarMaxBytes+10))
		}
	}))
	defer srv.Close()
	if img, err := radarImage(srv.URL + "/ok.png"); err != nil || img.Bounds().Dx() != 256 {
		t.Fatalf("normal: %v", err)
	}
	if _, err := radarImage(srv.URL + "/largo.png"); err == nil {
		t.Fatal("aceitou imagem larga demais")
	}
	if _, err := radarImage(srv.URL + "/pesado.png"); err == nil {
		t.Fatal("aceitou arquivo grande demais")
	}
}
