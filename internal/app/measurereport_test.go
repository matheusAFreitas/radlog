package app

import (
	"math"
	"math/rand"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func fakeMeasure(seconds int, bgCPM, objCPM float64) SavedMeasure {
	rng := rand.New(rand.NewSource(3))
	pois := func(l float64) uint32 {
		L, k, p := math.Exp(-l), uint32(0), 1.0
		for {
			p *= rng.Float64()
			if p <= L {
				return k
			}
			k++
		}
	}
	t0 := time.Date(2026, 10, 31, 15, 20, 0, 0, time.Local)
	var samples []MeasureSample
	var bgN, objN int
	for i := 0; i < seconds; i++ {
		c := pois(bgCPM / 60)
		bgN += int(c)
		samples = append(samples, MeasureSample{T: int64(i+1) * 1000, Phase: "fundo", CPS: c})
	}
	gap := int64(seconds+25) * 1000 // tempo para encostar o objeto
	for i := 0; i < seconds; i++ {
		c := pois(objCPM / 60)
		objN += int(c)
		samples = append(samples, MeasureSample{T: gap + int64(i+1)*1000, Phase: "objeto", CPS: c})
	}
	usv := func(c float64) float64 { return c * 0.0101 }
	bg := phaseResult(bgN, seconds, usv)
	bg.Start, bg.End = t0.UnixMilli(), t0.Add(time.Duration(seconds)*time.Second).UnixMilli()
	obj := phaseResult(objN, seconds, usv)
	obj.Start, obj.End = t0.UnixMilli()+gap, t0.UnixMilli()+gap+int64(seconds)*1000
	return SavedMeasure{ID: 1, Name: "Eletrodo de tungstênio toriado", Time: t0.UnixMilli(), Bg: bg, Obj: obj,
		Result: compareMeasure(bg, obj, usv), Note: "encostado no lado do tubo, sem embalagem", Samples: samples}
}

func TestMeasureReport(t *testing.T) {
	m := fakeMeasure(120, 25, 105)
	if ld := detectionLimitCPM(m.Bg); ld < 4 || ld > 20 {
		t.Fatalf("limite de detecção %v", ld)
	}
	if s := secondsForPrecision(m.Bg, m.Obj, 0.10); s < 5 || s > 600 {
		t.Fatalf("tempo para 10%%: %v", s)
	}
	st := calcPhaseStats(phaseSeconds(m.Samples, "objeto"))
	if st.N != 120 || st.Dispersion < 0.6 || st.Dispersion > 1.5 {
		t.Fatalf("estatística: %+v", st)
	}
	b := renderMeasurePDF(m, "GMC-800Re1.23", "0,0101 µSv/h por CPM", "desligada")
	if !strings.HasPrefix(string(b), "%PDF-1.4") {
		t.Fatal("não é PDF")
	}
	if out := os.Getenv("MEAS_OUT"); out != "" {
		_ = os.WriteFile(out, b, 0o644)
	}
	// medição antiga, sem os segundos
	old := m
	old.Samples = nil
	if b := renderMeasurePDF(old, "", "x", "y"); !strings.HasPrefix(string(b), "%PDF") {
		t.Fatal("sem segundos")
	}
}

func TestAutoMeasureReport(t *testing.T) {
	a, _ := testApp(t)
	dir := t.TempDir()
	a.settings.ReportDir = dir
	m := fakeMeasure(30, 25, 60)
	id, err := a.addMeasurement(m)
	if err != nil {
		t.Fatal(err)
	}
	a.measure.SavedID = id
	a.autoMeasureReport(id)
	if p := a.GetMeasure().ReportPath; filepath.Dir(p) != dir || !strings.Contains(filepath.Base(p), "gmc-medicao-eletrodo-de-tungstenio-toriado-2026-10-31_15h20") {
		t.Fatalf("caminho: %q (%s)", p, a.GetMeasure().ReportError)
	}
	if err := a.OpenReportFile(filepath.Join(t.TempDir(), "x.pdf")); err == nil {
		t.Fatal("abriu arquivo fora da pasta de relatórios")
	}
}
