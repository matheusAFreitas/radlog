package app

// Testes da conferência do QA da 2.101.8 (fator do PDF da medição salva).

import (
	"strings"
	"testing"

	"radlog/internal/gmc"
	"radlog/internal/pdf"
)

// linha "Calibração" e números do PDF de uma medição salva
func qaCalLine(t *testing.T, a *App, id int64) []string {
	t.Helper()
	var got []string
	pdf.SetTextHook(func(s string) {
		if strings.Contains(s, "por CPM") || strings.Contains(s, "hoje") || strings.Contains(s, "µSv/h") && strings.Contains(s, ",") && len(s) < 40 {
			got = append(got, s)
		}
	})
	_, err := a.saveMeasureReport(id)
	pdf.SetTextHook(ptTextHook)
	if err != nil {
		t.Fatal(err)
	}
	return got
}

func qaCalOnly(l []string) string {
	for _, s := range l {
		if strings.Contains(s, "por CPM") {
			return s
		}
	}
	return ""
}

// 2.101.8 (item 44): na faixa da curva, sem mudar nada, não aparece "hoje".
func TestQA2108CalTxtCurva(t *testing.T) {
	a, _ := testApp(t)
	a.settings.ReportDir = t.TempDir()
	a.settings.DateFormat = "iso"
	pts, err := suggestCounterCal(gmc.DefaultCalibration().Points, 0.00564)
	if err != nil {
		t.Fatal(err)
	}
	a.cal = gmc.Calibration{Points: pts, Source: "contador"}
	for i, objCPM := range []float64{105, 1500, 3000, 5000, 20000} {
		m := fakeMeasure(120, 25, objCPM)
		a.mu.Lock()
		cal := a.convCalLocked()
		a.mu.Unlock()
		m.Bg.USv = cal.USv(m.Bg.CPM)
		m.Obj.USv = cal.USv(m.Obj.CPM)
		m.Result = compareMeasure(m.Bg, m.Obj, cal.USv)
		m.ID = 0
		m.Name = "objeto " + string(rune('A'+i))
		id, err := a.addMeasurement(m)
		if err != nil {
			t.Fatal(err)
		}
		line := qaCalOnly(qaCalLine(t, a, id))
		t.Logf("objeto %5.0f CPM: %q", m.Obj.CPM, line)
		if strings.Contains(line, "hoje") || !strings.HasPrefix(line, "0,0056 ") {
			t.Errorf("curva: %q", line)
		}
	}
}

// 2.101.8 (item 44): calibração mudada depois da medição.
func TestQA2108CalTxtLater(t *testing.T) {
	for _, retro := range []bool{false, true} {
		a, _ := testApp(t)
		a.settings.ReportDir = t.TempDir()
		a.settings.DateFormat = "iso"
		a.loadCalHist() // como no app: a calibração de antes vale desde o começo
		m := fakeMeasure(120, 25, 300)
		m.Time -= 3600e3 // medida uma hora antes da mudança
		k0 := m.Obj.USv / m.Obj.CPM
		id, err := a.addMeasurement(m)
		if err != nil {
			t.Fatal(err)
		}
		if err := a.setGMCFactor(0.0065, "você", "intercalibração"); err != nil {
			t.Fatal(err)
		}
		if retro {
			a.mu.Lock()
			last := a.calHist.Versions[len(a.calHist.Versions)-1].ID
			a.mu.Unlock()
			if err := a.SetCalRetro(last); err != nil {
				t.Fatal(err)
			}
		}
		l := qaCalLine(t, a, id)
		line := qaCalOnly(l)
		t.Logf("retro=%v: números feitos com %.4f (objeto %.0f CPM = %.3f µSv/h); linha: %q", retro, k0, m.Obj.CPM, m.Obj.USv, line)
		if !strings.HasPrefix(line, "0,0101 ") || !strings.Contains(line, "hoje: 0,0065") {
			t.Errorf("retro=%v: esperava 0,0101 (a que valia) e hoje 0,0065: %q", retro, line)
		}
	}
}
