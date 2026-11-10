package app

import (
	"archive/zip"
	"encoding/json"
	"io"
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
	if res.Minutes != 60 || res.Events != 1 || res.Periods != 1 || res.Measurements != 1 || res.AlertFiles != 1 || res.From == "" {
		t.Fatalf("mesclagem: %+v", res)
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

// o lugar da chuva não vai para o backup, mas a chuva ainda mescla no mesmo lugar
func TestBackupRainHidesPlace(t *testing.T) {
	a, _ := testApp(t)
	b, _ := testApp(t)
	c, _ := testApp(t)
	h := time.Now().Truncate(time.Hour).Add(-5 * time.Hour).Unix()
	a.rain = &rainCache{V: rainCacheVersion, Lat: -15.7939, Lon: -47.8828, Hours: map[int64]float64{h: 1.5}, Press: map[int64]float64{h: 1012.3}}
	rb, _ := json.Marshal(a.rain)
	_ = os.WriteFile(a.rainPath(), rb, 0o644)
	zp := filepath.Join(t.TempDir(), "a.zip")
	if err := a.writeBackupFile(zp); err != nil {
		t.Fatal(err)
	}
	zr, err := zip.OpenReader(zp)
	if err != nil {
		t.Fatal(err)
	}
	for _, f := range zr.File {
		if f.Name != "chuva.json" {
			continue
		}
		r, _ := f.Open()
		raw, _ := io.ReadAll(r)
		r.Close()
		if strings.Contains(string(raw), "15.79") || strings.Contains(string(raw), "47.88") || strings.Contains(string(raw), `"lat"`) || !strings.Contains(string(raw), `"place"`) {
			t.Fatalf("lugar no backup: %s", raw)
		}
	}
	zr.Close()
	b.rain = &rainCache{V: rainCacheVersion, Lat: -15.7939, Lon: -47.8828, Hours: map[int64]float64{}}
	c.rain = &rainCache{V: rainCacheVersion, Lat: -23.55, Lon: -46.63, Hours: map[int64]float64{}}
	_, _ = b.mergeBackup(zp)
	_, _ = c.mergeBackup(zp)
	if b.rain.Hours[h] != 1.5 || b.rain.Press[h] != 1012.3 {
		t.Fatalf("mesmo lugar não mesclou: %+v", b.rain)
	}
	if len(c.rain.Hours) != 0 {
		t.Fatalf("outro lugar mesclou: %+v", c.rain)
	}
}
