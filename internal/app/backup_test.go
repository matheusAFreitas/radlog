package app

import (
	"os"
	"path/filepath"
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
