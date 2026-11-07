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
	if ld := detectionLimitCPM(m.Bg, m.Obj.Seconds); ld < 4 || ld > 20 {
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
	en := pdfWithLang("en", func() []byte { return renderMeasurePDF(m, "GMC-800Re1.23", "0,0101 µSv/h por CPM", "desligada") })
	if out := os.Getenv("MEAS_OUT"); out != "" {
		_ = os.WriteFile(strings.TrimSuffix(out, ".pdf")+"-en.pdf", en, 0o644)
	}
	// nome comprido não pode ser cortado
	long := m
	long.Name = "Eletrodo Toriado (Mesa Granito, encostado no tubo, lado esquerdo, sem embalagem plástica)"
	lb := renderMeasurePDF(long, "", "x", "y")
	if out := os.Getenv("MEAS_OUT"); out != "" {
		_ = os.WriteFile(strings.TrimSuffix(out, ".pdf")+"-longo.pdf", lb, 0o644)
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

func pdfWithLang(lang string, f func() []byte) []byte {
	a := &App{lang: lang}
	return a.inLang(f)
}

// fakeSpectrum faz um espectro de 1024 canais (3 keV cada): contínuo que cai
// com a energia, mais os picos dados (keV → contagens/s), em sec segundos.
func fakeSpectrum(rng *rand.Rand, sec float64, cont float64, peaks map[float64]float64) []uint32 {
	out := make([]uint32, 1024)
	for i := range out {
		e := (float64(i) + 0.5) * 3
		rate := cont * math.Exp(-e/350) / 3
		for pe, r := range peaks {
			sg := specFWHM(pe) / 2.355
			rate += r * 3 / (sg * math.Sqrt(2*math.Pi)) * math.Exp(-math.Pow((e-pe)/sg, 2)/2)
		}
		l := rate * sec
		if l > 500 {
			out[i] = uint32(math.Max(0, l+math.Sqrt(l)*rng.NormFloat64()))
			continue
		}
		L, k, p := math.Exp(-l), uint32(0), 1.0
		for {
			p *= rng.Float64()
			if p <= L {
				break
			}
			k++
		}
		out[i] = k
	}
	return out
}

func TestMeasureReportRadiacode(t *testing.T) {
	rng := rand.New(rand.NewSource(5))
	m := fakeMeasure(90, 720, 2400)
	m.Device = "radiacode"
	m.Name = "Camisa de lampião antiga"
	m.Bg.Source = "casa"
	m.Bg.Seconds, m.Bg.Counts = 3600, 3600*12
	m.Bg.USv, m.Obj.USv = 0.09, 0.31
	m.Result = compareMeasure(m.Bg, m.Obj, nil)
	bgPeaks := map[float64]float64{1460.8: 0.08, 609.3: 0.05, 351.9: 0.05}
	thPeaks := map[float64]float64{238.6: 3, 583.2: 1, 911.2: 0.5, 2614.5: 0.4, 1460.8: 0.08, 609.3: 0.05, 351.9: 0.05}
	sp := &MeasureSpectrum{A: [3]float64{0, 3, 0}, BgSrc: "acumulado", BgSec: 50 * 3600, ObjSec: 90}
	sp.Bg = fakeSpectrum(rng, sp.BgSec, 0.3, bgPeaks)
	sp.Obj = fakeSpectrum(rng, sp.ObjSec, 0.8, thPeaks)
	sp.Peaks = sp.findPeaks()
	m.Spectrum = sp
	groups := map[string]bool{}
	for _, p := range sp.Peaks {
		groups[p.Group] = true
	}
	if !groups["Tório-232 (série)"] || groups["Césio-137"] || groups["Amerício-241"] {
		t.Fatalf("picos do tório: %+v", sp.Peaks)
	}
	b := renderMeasurePDF(m, "RadiaCode-102 RC-102-000001", "o µSv/h é o que o próprio RadiaCode calcula (calibração de fábrica, por energia)", "")
	if !strings.HasPrefix(string(b), "%PDF-1.4") {
		t.Fatal("não é PDF")
	}
	if out := os.Getenv("MEAS_OUT"); out != "" {
		_ = os.WriteFile(strings.TrimSuffix(out, ".pdf")+"-rc.pdf", b, 0o644)
		en := pdfWithLang("en", func() []byte {
			return renderMeasurePDF(m, "RadiaCode-102 RC-102-000001", "o µSv/h é o que o próprio RadiaCode calcula (calibração de fábrica, por energia)", "")
		})
		_ = os.WriteFile(strings.TrimSuffix(out, ".pdf")+"-rc-en.pdf", en, 0o644)
	}
}

func TestMeasureReportDual(t *testing.T) {
	m := fakeMeasure(90, 720, 760)
	m.Device, m.Mode = "radiacode", "ambos"
	gb := phaseResult(1500, 3600, func(c float64) float64 { return c * 0.0065 })
	gb.Source = "casa"
	gobj := phaseResult(120, 90, func(c float64) float64 { return c * 0.0065 })
	r := compareMeasure(gb, gobj, nil)
	m.Other = &MeasureSide{Device: "gmc", Role: "objeto", Bg: &gb, Obj: &gobj, Result: &r, Hint: dualHint(r, m.Result)}
	b := renderMeasurePDF(m, "RadiaCode-102", "x", "")
	v := m
	v.Mode = "voce"
	v.Other = &MeasureSide{Device: "radiacode", Role: "voce", Dose: 0.0115, AvgUSv: 0.46, MaxUSv: 0.61, BgUSv: 0.10, Excess: 0.009}
	b2 := renderMeasurePDF(v, "GMC-800", "x", "y")
	if out := os.Getenv("MEAS_OUT"); out != "" {
		_ = os.WriteFile(strings.TrimSuffix(out, ".pdf")+"-ambos.pdf", b, 0o644)
		_ = os.WriteFile(strings.TrimSuffix(out, ".pdf")+"-voce.pdf", b2, 0o644)
	}
	if !strings.Contains(m.Other.Hint, "beta") {
		t.Fatalf("dica: %q", m.Other.Hint)
	}
}
