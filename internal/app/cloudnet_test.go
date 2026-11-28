package app

import (
	"testing"
	"time"
)

// no celular, na rede móvel, a nuvem segue Settings.CloudCell; no Wi-Fi e no
// PC, sincroniza sempre
func TestCloudHoldForNet(t *testing.T) {
	a := &App{mobile: true}
	recent, old := time.Now().Add(-10*time.Minute), time.Now().Add(-2*time.Hour)
	for _, c := range []struct {
		cell    string
		metered bool
		last    time.Time
		hold    bool
	}{
		{"", true, recent, false},
		{"wifi", false, recent, false},
		{"wifi", true, old, true},
		{"hora", true, recent, true},
		{"hora", true, old, false},
	} {
		a.settings.CloudCell = c.cell
		a.netMetered.Store(c.metered)
		a.cloudWaitNet.Store(false)
		if got := a.cloudHoldForNet(c.last); got != c.hold || a.cloudWaitNet.Load() != c.hold {
			t.Errorf("%+v: espera = %v", c, got)
		}
	}
	a.mobile = false
	a.settings.CloudCell = "wifi"
	a.netMetered.Store(true)
	if a.cloudHoldForNet(recent) {
		t.Error("no PC não espera")
	}
}
