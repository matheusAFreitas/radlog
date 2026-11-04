package app

import (
	"math"
	"testing"
	"time"

	"radlog/internal/gmc"
	"radlog/internal/store"
)

func TestChartUSv(t *testing.T) {
	dir := t.TempDir()
	st, _ := store.Open(dir)
	a := NewApp()
	a.dataDir, a.store, a.cal = dir, st, gmc.DefaultCalibration()
	end := time.Now().Truncate(time.Minute)
	for i := 30; i >= 1; i-- {
		_ = st.Add(store.Point{T: end.Add(-time.Duration(i) * time.Minute), CPM: float64(10 + i), Src: store.SrcLive})
	}
	for _, period := range []string{"15m", "30m", "1h", "24h"} {
		c, err := a.GetChart(period)
		if err != nil {
			t.Fatal(err)
		}
		if len(c.USv) != len(c.Values) {
			t.Fatalf("%s: %d valores e %d em µSv", period, len(c.Values), len(c.USv))
		}
		for i, v := range c.Values {
			if (v == nil) != (c.USv[i] == nil) || (v != nil && math.Abs(*c.USv[i]-a.cal.USv(*v)) > 1e-12) {
				t.Fatalf("%s: ponto %d não confere", period, i)
			}
		}
		if c.Max == nil || math.Abs(*c.MaxUSv-a.cal.USv(*c.Max)) > 1e-12 || c.AvgUSv == nil {
			t.Fatalf("%s: resumo em µSv errado", period)
		}
		if c.DangerCPM != 1000 || math.Abs(c.DangerUSv-gmc.DefaultCalibration().USv(1000)) > 1e-9 || c.AlarmCPM <= 0 || c.AlarmUSv <= 0 {
			t.Fatalf("limites dos alarmes: %+v", c)
		}
		if math.Abs(c.AttentionUSv-50*10.0/986) > 1e-12 {
			t.Fatalf("limiar de atenção em µSv: %v", c.AttentionUSv)
		}
	}
}

// O gráfico de 15/30 min começa no meio de um minuto: sem leituras por
// segundo (app reaberto), o minuto gravado tem que cobrir esse começo.
func TestShortChartFirstMinute(t *testing.T) {
	a := NewApp()
	a.dataDir = t.TempDir()
	a.settings = defaultSettings()
	st, err := store.Open(a.dataDir)
	if err != nil {
		t.Fatal(err)
	}
	a.store = st
	base := time.Now().Truncate(time.Minute).Add(-20 * time.Minute)
	var pts []store.Point
	for i := 0; i < 21; i++ {
		pts = append(pts, store.Point{T: base.Add(time.Duration(i) * time.Minute), CPM: float64(20 + i)})
	}
	if _, err := st.ImportNew(pts); err != nil {
		t.Fatal(err)
	}
	from := base.Add(5*time.Minute + 20*time.Second)
	c, err := a.buildChart(from, from.Add(15*time.Minute), 5*time.Second)
	if err != nil {
		t.Fatal(err)
	}
	if c.Values[0] == nil || *c.Values[0] != 25 {
		t.Fatalf("começo do gráfico vazio ou errado: %v", c.Values[0])
	}
	for i := 0; i < 8; i++ { // os primeiros 40 s ainda são do minuto 5
		if c.Values[i] == nil {
			t.Fatalf("buraco no ponto %d", i)
		}
	}
}
