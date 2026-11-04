package app

import (
	"bytes"
	"fmt"
	"image"
	"image/color"
	"image/png"
	"math"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

func pngTile(t *testing.T, px, py int, c color.NRGBA) []byte {
	t.Helper()
	img := image.NewNRGBA(image.Rect(0, 0, 256, 256))
	if c.A > 0 {
		img.SetNRGBA(px, py, c)
	}
	var b bytes.Buffer
	if err := png.Encode(&b, img); err != nil {
		t.Fatal(err)
	}
	return b.Bytes()
}

func TestRadarColorAndRate(t *testing.T) {
	if d := colorDBZ(color.NRGBA{0x00, 0x77, 0xaa, 0xff}.RGBA()); d != 25 {
		t.Fatalf("0077aa = %v dBZ, quero 25", d)
	}
	if d := colorDBZ(color.NRGBA{0xff, 0xaa, 0x00, 0xff}.RGBA()); d != 40 {
		t.Fatalf("ffaa00 = %v dBZ, quero 40", d)
	}
	if d := colorDBZ(color.NRGBA{0x63, 0x61, 0x59, 0x14}.RGBA()); d != 0 {
		t.Fatalf("cinza fraco virou %v dBZ", d)
	}
	// 40 dBZ ≈ 11,5 mm/h (chuva forte); 20 dBZ ≈ 0,65 mm/h
	if r := dbzToMMh(40); math.Abs(r-11.53) > 0.1 {
		t.Fatalf("40 dBZ = %.2f mm/h", r)
	}
	if r := dbzToMMh(20); math.Abs(r-0.65) > 0.05 {
		t.Fatalf("20 dBZ = %.2f mm/h", r)
	}
	if dbzToMMh(10) != 0 {
		t.Fatal("garoa/ruído deveria ser 0")
	}
	x, y, _, _ := tileOf(-15.79, -47.88, 7, 256) // Brasília
	if x != 46 || y != 69 {
		t.Fatalf("tile %d/%d", x, y)
	}
}

func TestRadarFetch(t *testing.T) {
	lat, lon := -15.79, -47.88
	x, y, px, py := tileOf(lat, lon, radarZoom, 256)
	now := time.Now().Truncate(10 * time.Minute)
	t1, t2 := now.Add(-20*time.Minute).Unix(), now.Add(-10*time.Minute).Unix()
	var hits int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&hits, 1)
		host := "http://" + r.Host
		switch {
		case r.URL.Path == "/index.json":
			fmt.Fprintf(w, `{"host":%q,"radar":{"past":[{"time":%d,"path":"/v2/radar/a"},{"time":%d,"path":"/v2/radar/b"}]}}`, host, t1, t2)
		case strings.HasPrefix(r.URL.Path, "/v2/coverage/"):
			w.Write(pngTile(t, px, py, color.NRGBA{})) // transparente = coberto
		case r.URL.Path == fmt.Sprintf("/v2/radar/a/256/7/%d/%d/2/0_0.png", x, y):
			w.Write(pngTile(t, px, py, color.NRGBA{})) // sem chuva
		case r.URL.Path == fmt.Sprintf("/v2/radar/b/256/7/%d/%d/2/0_0.png", x, y):
			// chuva forte do lado do ponto (dentro do quadradinho de 3×3)
			w.Write(pngTile(t, px+1, py, color.NRGBA{0xff, 0xaa, 0x00, 0xff}))
		default:
			http.NotFound(w, r)
		}
	}))
	defer srv.Close()
	oldURL := radarIndexURL
	radarIndexURL = srv.URL + "/index.json"
	defer func() { radarIndexURL = oldURL }()

	a := NewApp()
	a.dataDir = t.TempDir()
	a.settings = defaultSettings()
	a.settings.RainEnabled, a.settings.RainLat, a.settings.RainLon = true, lat, lon

	if err := a.fetchRadar(lat, lon); err != nil {
		t.Fatal(err)
	}
	pts := a.GetRainRadar(now.Add(-time.Hour).UnixMilli(), now.Add(time.Hour).UnixMilli())
	if len(pts) != 2 {
		t.Fatalf("leituras: %+v", pts)
	}
	if pts[0].MMh != 0 || pts[1].DBZ != 40 || pts[1].MMh < 11 {
		t.Fatalf("valores: %+v", pts)
	}
	st := a.GetRainStatus()
	if st.RadarCovered == nil || !*st.RadarCovered || st.RadarNow == nil || st.RadarNow.DBZ != 40 || st.RadarFetched == 0 {
		t.Fatalf("status: %+v", st)
	}

	// segunda busca: não baixa de novo os quadros que já tem
	before := atomic.LoadInt32(&hits)
	if err := a.fetchRadar(lat, lon); err != nil {
		t.Fatal(err)
	}
	if n := atomic.LoadInt32(&hits) - before; n != 1 {
		t.Fatalf("baixou %d coisas, queria só o índice", n)
	}

	// grava e lê de volta
	b := NewApp()
	b.dataDir = a.dataDir
	b.settings = a.settings
	b.loadRadar()
	if len(b.GetRainRadar(0, now.Add(time.Hour).UnixMilli())) != 2 {
		t.Fatal("não leu chuva-radar.json")
	}
	// outro lugar: não mistura
	b.settings.RainLat = -22.9
	if len(b.GetRainRadar(0, now.Add(time.Hour).UnixMilli())) != 0 {
		t.Fatal("mostrou radar de outro lugar")
	}
}

func TestRadarNoCoverage(t *testing.T) {
	lat, lon := -10.0, -60.0
	_, _, px, py := tileOf(lat, lon, radarZoom, 256)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.URL.Path == "/index.json":
			fmt.Fprintf(w, `{"host":"http://%s","radar":{"past":[{"time":%d,"path":"/v2/radar/a"}]}}`, r.Host, time.Now().Unix())
		case strings.HasPrefix(r.URL.Path, "/v2/coverage/"):
			w.Write(pngTile(t, px, py, color.NRGBA{0, 0, 0, 0xff})) // preto = sem radar
		default:
			w.Write(pngTile(t, px, py, color.NRGBA{}))
		}
	}))
	defer srv.Close()
	old := radarIndexURL
	radarIndexURL = srv.URL + "/index.json"
	defer func() { radarIndexURL = old }()

	a := NewApp()
	a.dataDir = t.TempDir()
	a.settings = defaultSettings()
	a.settings.RainEnabled, a.settings.RainLat, a.settings.RainLon = true, lat, lon
	if err := a.fetchRadar(lat, lon); err != nil {
		t.Fatal(err)
	}
	st := a.GetRainStatus()
	if st.RadarCovered == nil || *st.RadarCovered {
		t.Fatalf("deveria dizer sem cobertura: %+v", st)
	}
}
