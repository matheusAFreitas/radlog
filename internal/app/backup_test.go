package app

import (
	"archive/zip"
	"encoding/json"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"radlog/internal/store"
)

func fillApp(t *testing.T, a *App, base time.Time, from, n int, tag string) {
	t.Helper()
	var pts []store.Point
	for m := from; m < from+n; m++ {
		pts = append(pts, store.Point{T: base.Add(time.Duration(m) * time.Minute), CPM: 25, Src: store.SrcLive})
	}
	if _, err := a.store.ImportNew(pts); err != nil {
		t.Fatal(err)
	}
	a.addEvent(Event{Time: base.Add(time.Duration(from) * time.Minute).UnixMilli(), Kind: kindEvent, Text: "evento " + tag}, true)
}

func TestBackupMerge(t *testing.T) {
	base := time.Now().Truncate(time.Hour).Add(-48 * time.Hour)
	a, _ := testApp(t)
	b, _ := testApp(t)
	fillApp(t, a, base, 0, 120, "A")  // 2 h
	fillApp(t, b, base, 60, 120, "B") // 1 h em comum
	_, _ = a.SavePeriod(Period{From: base.UnixMilli(), To: base.Add(time.Hour).UnixMilli(), Name: "parque"})
	a.measurements = []SavedMeasure{{ID: 1, Name: "banana", Time: base.UnixMilli()}}
	_ = a.saveMeasurementsLocked()
	_ = os.MkdirAll(a.alertDir(), 0o755)
	_ = os.WriteFile(filepath.Join(a.alertDir(), "123-atencao.json"), []byte(`[{"t":1,"cpm":2}]`), 0o644)

	zip := filepath.Join(t.TempDir(), "b.zip")
	if err := a.writeBackupFile(zip); err != nil {
		t.Fatal(err)
	}
	res, err := b.mergeBackup(zip)
	if err != nil {
		t.Fatal(err)
	}
	if res.Minutes != 60 || res.Events != 1 || res.Periods != 1 || res.Measurements != 1 || res.AlertFiles != 1 || res.From != "" || res.Created == "" {
		t.Fatalf("mesclagem (sem o nome do PC desde a 2.78.0): %+v", res)
	}
	pts, _ := b.store.Range(base, base.Add(4*time.Hour))
	if len(pts) != 180 {
		t.Fatalf("B ficou com %d minutos", len(pts))
	}
	// de novo: nada novo
	if res2, _ := b.mergeBackup(zip); res2.total() != 0 {
		t.Fatalf("segunda vez: %+v", res2)
	}
	// arquivo que não é backup
	bad := filepath.Join(t.TempDir(), "x.zip")
	_ = os.WriteFile(bad, []byte("nada"), 0o644)
	if _, err := b.mergeBackup(bad); err == nil {
		t.Fatal("aceitou arquivo inválido")
	}
}

// o lugar da chuva não vai para o backup (nem marca, nem erro com o endereço),
// mas a chuva ainda mescla no mesmo lugar, reconhecido pela pressão
func TestBackupRainHidesPlace(t *testing.T) {
	a, _ := testApp(t)
	b, _ := testApp(t)
	c, _ := testApp(t)
	h := time.Now().Truncate(time.Hour).Add(-5 * time.Hour).Unix()
	press := func(base float64) map[int64]float64 {
		m := map[int64]float64{}
		for i := 0; i < 24; i++ {
			m[h-int64(i)*3600] = base + float64(i%5)*0.3
		}
		return m
	}
	a.rain = &rainCache{V: rainCacheVersion, Lat: -15.7939, Lon: -47.8828, Hours: map[int64]float64{h: 1.5}, Press: press(886.0)}
	rb, _ := json.Marshal(a.rain)
	_ = os.WriteFile(a.rainPath(), rb, 0o644)
	a.radar = &radarStore{V: radarVersion, Lat: -15.7939, Lon: -47.8828, Points: map[int64]RadarPoint{h: {Time: h * 1000, DBZ: 30, MMh: 2.7}},
		Err: `sem acesso ao radar: Get "https://tilecache.rainviewer.com/v2/radar/x/256/7/46/68/2/0_0.png": timeout`}
	a.saveRadarLocked()
	zp := filepath.Join(t.TempDir(), "a.zip")
	if err := a.writeBackupFile(zp); err != nil {
		t.Fatal(err)
	}
	zr, err := zip.OpenReader(zp)
	if err != nil {
		t.Fatal(err)
	}
	for _, f := range zr.File {
		if f.Name != "chuva.json" && f.Name != "chuva-radar.json" {
			continue
		}
		r, _ := f.Open()
		raw, _ := io.ReadAll(r)
		r.Close()
		for _, bad := range []string{"15.79", "47.88", `"lat"`, `"lon"`, `"place"`, "rainviewer", `"err"`} {
			if strings.Contains(string(raw), bad) {
				t.Fatalf("%s: lugar no backup (%s): %s", f.Name, bad, raw)
			}
		}
	}
	zr.Close()
	// b: mesma cidade (mesma pressão nas horas em comum); c: outra cidade
	b.rain = &rainCache{V: rainCacheVersion, Lat: -15.7939, Lon: -47.8828, Hours: map[int64]float64{}, Press: press(886.0)}
	delete(b.rain.Press, h)
	b.radar = &radarStore{V: radarVersion, Lat: -15.7939, Lon: -47.8828, Points: map[int64]RadarPoint{}}
	c.rain = &rainCache{V: rainCacheVersion, Lat: -23.55, Lon: -46.63, Hours: map[int64]float64{}, Press: press(925.0)}
	c.radar = &radarStore{V: radarVersion, Lat: -23.55, Lon: -46.63, Points: map[int64]RadarPoint{}}
	_, _ = b.mergeBackup(zp)
	_, _ = c.mergeBackup(zp)
	if b.rain.Hours[h] != 1.5 || b.rain.Press[h] != 886.0 || len(b.radar.Points) != 1 {
		t.Fatalf("mesmo lugar não mesclou: %+v %+v", b.rain, b.radar)
	}
	if len(c.rain.Hours) != 0 || len(c.radar.Points) != 0 {
		t.Fatalf("outro lugar mesclou: %+v %+v", c.rain, c.radar)
	}
}

// backups da 2.44.0 (marca) e de antes (coordenadas) continuam reconhecidos
func TestSamePlaceOld(t *testing.T) {
	if s, ok := samePlace(-15.7939, -47.8828, 0, 0, placeKey(-15.7939, -47.8828)); !s || !ok {
		t.Fatal("marca da 2.44.0")
	}
	if s, ok := samePlace(-15.7939, -47.8828, -23.55, -46.63, ""); s || !ok {
		t.Fatal("coordenadas antigas, outro lugar")
	}
	if _, ok := samePlace(-15.7939, -47.8828, 0, 0, ""); ok {
		t.Fatal("backup sem lugar deveria cair na pressão")
	}
	if samePressure(map[int64]float64{1: 900}, map[int64]float64{1: 900}) {
		t.Fatal("uma hora só não basta")
	}
}

func TestNoURL(t *testing.T) {
	_, err := http.Get("http://127.0.0.1:1/forecast?latitude=-15.7939&longitude=-47.8828")
	if err == nil || strings.Contains(noURL(err).Error(), "15.79") {
		t.Fatalf("erro com o endereço: %v", noURL(err))
	}
}
