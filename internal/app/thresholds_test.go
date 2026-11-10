package app

import (
	"testing"

	"radlog/internal/gmc"
)

// uT escreve um CPM em µSv/h como os textos dos alertas (calibração em uso).
func uT(a *App, cpm float64) string {
	a.mu.Lock()
	defer a.mu.Unlock()
	return fmtUSv(a.convCalLocked().USv(cpm)) + " µSv/h"
}

func uN(a *App, cpm float64) string {
	a.mu.Lock()
	defer a.mu.Unlock()
	return fmtUSv(a.convCalLocked().USv(cpm))
}

func TestCpmForUSv(t *testing.T) {
	cal := gmc.DefaultCalibration()
	for _, cpm := range []int{5, 50, 100, 300, 1000, 2000, 50000} {
		if got := cpmForUSv(cal, cal.USv(float64(cpm))); got != cpm {
			t.Errorf("cpmForUSv(USv(%d)) = %d", cpm, got)
		}
	}
}

func TestFmtUSv(t *testing.T) {
	for in, want := range map[float64]string{0.1234: "0,123", 1.2: "1,20", 10.14: "10,1", 152.4: "152", 0: "0"} {
		if got := fmtUSv(in); got != want {
			t.Errorf("fmtUSv(%v) = %q, quer %q", in, got, want)
		}
	}
}

func TestMigrateThresholdsKeepsCPM(t *testing.T) {
	a := newTestApp(t)
	a.mu.Lock()
	a.settings.AlarmUSv, a.settings.Levels.AttentionUSv, a.settings.Levels.DangerUSv = 0, 0, 0
	a.settings.AlarmCPM, a.settings.Levels.AttentionCPM, a.settings.Levels.DangerCPM = 123, 61, 987
	a.migrateThrLocked()
	eff := a.effLocked()
	a.mu.Unlock()
	if eff.AlarmCPM != 123 || eff.Levels.AttentionCPM != 61 || eff.Levels.DangerCPM != 987 {
		t.Fatalf("migração mudou o CPM: %d %d %d", eff.AlarmCPM, eff.Levels.AttentionCPM, eff.Levels.DangerCPM)
	}
}
