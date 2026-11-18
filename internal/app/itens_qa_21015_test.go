package app

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// 2.101.15 (item 133): no RC102 com o ± da dose, o evento e o PDF usam o ±,
// o σ, a razão e a precisão da dose mesmo sem DoseUp; o GMC-800 não muda.
func TestRCDoseEverywhereItem133(t *testing.T) {
	bg := MeasurePhase{Counts: 36000, Seconds: 3600, CPM: 600, ErrCPM: 3.2, USv: 0.169, ErrUSv: 0.002}
	obj := MeasurePhase{Counts: 19260, Seconds: 1800, CPM: 642, ErrCPM: 4.6, USv: 0.195, ErrUSv: 0.003}
	r := compareMeasure(bg, obj, nil)
	if r.DoseUp || r.NetErrUSv <= 0 || r.Level != "sim" {
		t.Fatalf("caso errado: %+v", r)
	}
	k := obj.USv / obj.CPM
	ev := measEventText("objeto", " (RC102)", r, k, true)
	want := "± " + usvTxt(r.NetErrUSv) + " µSv/h sobre o fundo (" + br(r.DoseSigma, false) + "σ na dose)"
	if !strings.Contains(ev, want) {
		t.Fatalf("evento %q sem %q", ev, want)
	}
	if g := measEventText("objeto", "", r, k, false); strings.Contains(g, "na dose") {
		t.Fatalf("GMC-800 com a dose: %q", g)
	}
	if _, err := exec.LookPath("pdftotext"); err != nil {
		t.Skip("sem pdftotext")
	}
	m := fakeMeasure(1800, 600, 642)
	m.Device, m.Bg, m.Obj, m.Result = "radiacode", bg, obj, r
	f := filepath.Join(t.TempDir(), "m.pdf")
	if err := os.WriteFile(f, renderMeasurePDF(m, "RadiaCode-102", "x", ""), 0o644); err != nil {
		t.Fatal(err)
	}
	out, err := exec.Command("pdftotext", "-layout", f, "-").Output()
	if err != nil {
		t.Fatal(err)
	}
	txt := strings.Join(strings.Fields(string(out)), " ")
	// só ASCII: o pdftotext do Windows troca o ÷
	for _, s := range []string{"fundo (dose)", "pela dose do RC102"} {
		if !strings.Contains(txt, s) {
			t.Fatalf("PDF sem %q", s)
		}
	}
}
