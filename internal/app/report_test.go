package app

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestRenderReport(t *testing.T) {
	base := time.Now().Truncate(time.Hour).Add(-48 * time.Hour)
	a, _ := testApp(t)
	fillApp(t, a, base, 0, 600, "A")
	T := base.Add(2 * time.Hour).UnixMilli()
	a.events = append(a.events,
		Event{ID: 1, Time: T, End: T + 600000, Kind: kindAlert, Level: "atencao", Peak: 70},
		Event{ID: 2, Time: T + 300000, End: T + 500000, Kind: kindAlert, Level: "alarme", Peak: 134},
	)
	r, err := a.buildReport(base.UnixMilli(), base.Add(24*time.Hour).UnixMilli(), "teste")
	if err != nil {
		t.Fatal(err)
	}
	if r.Alerts != "1 alarme" || len(r.AlertList) != 1 || r.AlertList[0].Peak[:3] != "134" {
		t.Fatalf("alertas: %q %+v", r.Alerts, r.AlertList)
	}
	b := renderReportPDF(r)
	if !strings.HasPrefix(string(b), "%PDF-1.4") || !strings.Contains(string(b), "%%EOF") {
		t.Fatal("não parece um PDF")
	}
}

func TestUniquePath(t *testing.T) {
	dir := t.TempDir()
	p := filepath.Join(dir, "r.pdf")
	if uniquePath(p) != p {
		t.Fatal("arquivo novo mudou de nome")
	}
	_ = os.WriteFile(p, []byte("x"), 0o644)
	if q := uniquePath(p); q != filepath.Join(dir, "r-2.pdf") {
		t.Fatalf("esperava r-2.pdf, veio %s", q)
	}
}
