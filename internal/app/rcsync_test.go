package app

import (
	"archive/zip"
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"radlog/internal/gmc"
)

func writeRcMinutes(t *testing.T, a *App, from time.Time, n int, usv float64) {
	t.Helper()
	os.MkdirAll(a.rcDir(), 0o755)
	files := map[string]*strings.Builder{}
	for i := 0; i < n; i++ {
		m := from.Add(time.Duration(i) * time.Minute)
		k := m.Format("2006-01") + ".csv"
		if files[k] == nil {
			files[k] = &strings.Builder{}
			files[k].WriteString(rcHeader + "\n")
		}
		fmt.Fprintf(files[k], "%s,%.2f,%.4f,%d\n", m.Format(time.RFC3339), 2.0, usv, 30)
	}
	for k, b := range files {
		os.WriteFile(filepath.Join(a.rcDir(), k), []byte(b.String()), 0o644)
	}
}

// pull simula um "puxar" de b para a (o que o syncOnce faz).
func syncPull(t *testing.T, a, b *App) MergeResult {
	t.Helper()
	ra, rb := a.rcInventory(), b.rcInventory()
	days, files := rcWant(rb, ra)
	z, err := b.buildDelta(pullReq{Rc: true, RcDays: days, RcFiles: files, Anexos: attachWant(b.attachInventory(), a.attachInventory())})
	if err != nil {
		t.Fatal(err)
	}
	res, err := a.mergeDelta(z, "outro")
	if err != nil {
		t.Fatal(err)
	}
	return res
}

func TestSyncRadiacode(t *testing.T) {
	win, lin := newTestApp(t), newTestApp(t)
	cut := time.Now().Truncate(time.Minute).Add(-3 * time.Hour)
	// o RadiaCode esteve no Windows até "cut" e no Linux depois
	writeRcMinutes(t, win, cut.Add(-5*time.Hour), 300, 0.10)
	writeRcMinutes(t, lin, cut, 120, 0.12)
	// espectro fechado no Linux, dose do aparelho e um arquivo de log
	os.MkdirAll(lin.rcSpecPath(""), 0o755)
	spec := cut.Format("2006-01-02-150405") + ".jsonl.gz"
	os.WriteFile(lin.rcSpecPath(spec), []byte("gz"), 0o644)
	os.WriteFile(filepath.Join(lin.rcDir(), rcDoseFile), []byte(`{"at": 1}`), 0o644)

	res := syncPull(t, win, lin)
	if res.RcMinutes != 120 {
		t.Fatalf("minutos do RadiaCode no Windows: %d", res.RcMinutes)
	}
	if pts := win.rcRange(cut.Add(-5*time.Hour), cut.Add(3*time.Hour)); len(pts) != 420 {
		t.Fatalf("Windows: %d minutos", len(pts))
	}
	if _, err := os.Stat(win.rcSpecPath(spec)); err != nil {
		t.Fatal("espectro não veio")
	}
	// e o contrário
	syncPull(t, lin, win)
	if pts := lin.rcRange(cut.Add(-5*time.Hour), cut.Add(3*time.Hour)); len(pts) != 420 {
		t.Fatalf("Linux: %d minutos", len(pts))
	}
	// agora os resumos batem: nada mais a pedir
	d, f := rcWant(lin.rcInventory(), win.rcInventory())
	if len(d) != 0 || len(f) != 0 {
		t.Fatalf("ainda diferente: %v %v", d, f)
	}
	// arquivo sendo gravado não entra no inventário
	f2, _ := os.Create(lin.rcSpecPath("2099-01-01-000000.jsonl.gz"))
	lin.rc.spec.mu.Lock()
	lin.rc.spec.f = f2
	lin.rc.spec.mu.Unlock()
	for _, x := range lin.rcInventory().Files {
		if strings.Contains(x, "2099") {
			t.Fatal("arquivo aberto no inventário")
		}
	}
	lin.rc.spec.mu.Lock()
	lin.rc.spec.f = nil
	lin.rc.spec.mu.Unlock()
	f2.Close()
}

func TestSyncAttachmentsAndCalibration(t *testing.T) {
	win, lin := newTestApp(t), newTestApp(t)
	t0 := time.Now().Add(-time.Hour)
	addTestMeasure(win, 1, "Eletrodo", "encostado", t0)
	addTestMeasure(lin, 7, "Outra", "", t0.Add(-24*time.Hour))
	src := filepath.Join(t.TempDir(), "foto.jpg")
	os.WriteFile(src, []byte("jpg"), 0o644)
	if _, err := win.attachFiles(1, []string{src}); err != nil {
		t.Fatal(err)
	}
	// calibração: o Windows leu o contador; o Linux nunca
	win.loadCalHist()
	lin.loadCalHist()
	win.saveLastCal(gmc.Calibration{Points: []gmc.CalPoint{{CPM: 1500, USv: 10}}, Source: "contador"})
	if err := win.SetGMCFactor(0.0055, ""); err != nil {
		t.Fatal(err)
	}
	syncPull(t, lin, win)
	var got *SavedMeasure
	for i := range lin.measurements {
		if lin.measurements[i].Name == "Eletrodo" {
			got = &lin.measurements[i]
		}
	}
	if got == nil || len(got.Attachments) != 1 {
		t.Fatalf("medição: %+v", got)
	}
	if _, err := os.Stat(filepath.Join(lin.attachDir(got.ID), "foto.jpg")); err != nil {
		t.Fatalf("anexo: %v", err)
	}
	vs := lin.calHist.Versions
	if len(vs) != 2 || vs[0].Source != "contador" || vs[0].Points[0].CPM != 1500 || vs[1].Factor != 0.0055 {
		t.Fatalf("calibração no Linux: %+v", vs)
	}
	// o pacote de um PC antigo (sem Rc) não leva nada do RadiaCode
	z, _ := win.buildDelta(pullReq{})
	zr, _ := zip.NewReader(bytes.NewReader(z), int64(len(z)))
	for _, f := range zr.File {
		if strings.HasPrefix(f.Name, rcDirName+"/") || strings.HasPrefix(f.Name, attachDirName) {
			t.Fatalf("pacote antigo com %s", f.Name)
		}
	}
	_ = json.Marshal
}
