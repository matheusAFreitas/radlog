package app

import (
	"math"
	"testing"
	"time"

	"radlog/internal/radiacode"
	"radlog/internal/store"
)

func TestRcMinutesAndCompare(t *testing.T) {
	a := newTestApp(t)
	now := time.Now().Truncate(time.Minute)
	var pts []store.Point
	for i := 30; i >= 2; i-- {
		m := now.Add(-time.Duration(i) * time.Minute)
		pts = append(pts, store.Point{T: m, CPM: 20 + float64(i%2)*2, Src: "live"})
		// RadiaCode: 2 leituras por minuto, µSv/h acompanhando o GMC
		usv := 0.10 + float64(i%2)*0.01
		a.rcAddMinute(m.Add(5*time.Second), 3.0, usv)
		a.rcAddMinute(m.Add(35*time.Second), 3.2, usv)
	}
	a.rcFlushMinute()
	if _, err := a.store.Import(pts); err != nil {
		t.Fatal(err)
	}
	got := a.rcRange(now.Add(-time.Hour), now)
	if len(got) != 29 || got[0].N != 2 || math.Abs(got[0].CPS-3.1) > 1e-9 {
		t.Fatalf("minutos gravados: %d %+v", len(got), got[0])
	}
	c, err := a.GetRcCompare(60)
	if err != nil {
		t.Fatal(err)
	}
	if c.Minutes != 29 || c.Ratio <= 0 || c.Corr < 0.99 {
		t.Fatalf("comparação: %+v", c)
	}
}

func TestRcRecordsState(t *testing.T) {
	a := newTestApp(t)
	a.rcRecords([]radiacode.Record{
		radiacode.RealTime{CPS: 5, DoseRate: 0.12, CPSErr: 10, DoseErr: 12},
		radiacode.Rare{Dose: 3.4, Duration: 7200, Battery: 87, TempC: 24.5},
	})
	st := a.GetRadiacode()
	if st.CPS != 5 || st.USvh != 0.12 || st.Dose != 3.4 || st.Battery != 87 || !st.HasRare {
		t.Fatalf("estado: %+v", st)
	}
}
