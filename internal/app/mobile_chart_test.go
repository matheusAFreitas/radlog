package app

import (
	"encoding/json"
	"testing"
	"time"
)

func TestMobileChartRC(t *testing.T) {
	a := NewMobile(t.TempDir(), func(string, any) {})
	a.settings = a.loadSettings()
	a.store = a.openStore()
	now := time.Now()
	for i := 1; i <= 30; i++ {
		m := truncLocal(now, time.Minute).Add(-time.Duration(i) * time.Minute)
		a.rcWriteMinute(rcMinute{t: m, cps: 600, usv: 9.6, n: 60})
	}
	c, err := a.GetChart("6h")
	if err != nil {
		t.Fatal(err)
	}
	k := 0
	for _, v := range c.RC {
		if v != nil {
			k++
		}
	}
	if _, err := json.Marshal(c); err != nil {
		t.Fatal("JSON:", err)
	}
	t.Logf("times=%d rc=%d", len(c.Times), k)
	if k == 0 {
		t.Fatal("sem minutos do RC no gráfico")
	}
}
