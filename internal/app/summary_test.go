package app

import (
	"testing"
	"time"

	"radlog/internal/store"
)

func TestSummary(t *testing.T) {
	a, _ := testApp(t)
	day := time.Now().Truncate(24 * time.Hour).Add(-48 * time.Hour)
	var pts []store.Point
	for m := 0; m < 24*60; m++ {
		v := 25.0
		if m == 600 {
			v = 60
		}
		if m >= 1200 { // 4 h finais sem dado
			break
		}
		pts = append(pts, store.Point{T: day.Add(time.Duration(m) * time.Minute), CPM: v})
	}
	if _, err := a.store.ImportNew(pts); err != nil {
		t.Fatal(err)
	}
	a.addEvent(Event{Time: day.Add(600 * time.Minute).UnixMilli(), End: day.Add(601 * time.Minute).UnixMilli(), Kind: kindAlert, Level: "atencao",
		CPM: 55, Peak: 63, PeakAt: day.Add(600*time.Minute + 20*time.Second).UnixMilli(), Text: "Atenção: 55 CPM (a partir de 50)"}, true)
	_, _ = a.SavePeriod(Period{From: day.Add(60 * time.Minute).UnixMilli(), To: day.Add(120 * time.Minute).UnixMilli(), Name: "mercado"})
	a.settings.RainEnabled, a.settings.RainLat, a.settings.RainLon = true, 1, 1
	a.rain = &rainCache{V: rainCacheVersion, Lat: 1, Lon: 1, Hours: map[int64]float64{
		day.Add(5 * time.Hour).Unix(): 2.5, day.Add(6 * time.Hour).Unix(): 0.1}}

	s, err := a.GetSummary(day.UnixMilli(), day.Add(24*time.Hour).UnixMilli())
	if err != nil {
		t.Fatal(err)
	}
	if s.Minutes != 1200 || s.Coverage < 0.83 || s.Coverage > 0.84 || s.AwayMinutes != 60 {
		t.Fatalf("minutos %d cobertura %.2f fora %d", s.Minutes, s.Coverage, s.AwayMinutes)
	}
	if *s.Max != 63 || s.MaxAt != day.Add(600*time.Minute+20*time.Second).UnixMilli() || *s.Min != 25 || s.Attention != 1 {
		t.Fatalf("máx %v às %v, mín %v, atenção %d", *s.Max, s.MaxAt, *s.Min, s.Attention)
	}
	if s.RainMM < 2.59 || s.RainMM > 2.61 || s.RainHours != 1 || !s.RainKnown {
		t.Fatalf("chuva %.2f mm em %d h", s.RainMM, s.RainHours)
	}
	if s.StepMs != (15*time.Minute).Milliseconds() || len(s.Curve) != 96 || s.Curve[95].CPM != nil || s.Curve[0].CPM == nil {
		t.Fatalf("curva: passo %d, %d pontos", s.StepMs, len(s.Curve))
	}
	if s.Dose <= 0 {
		t.Fatal("sem dose")
	}
	// semana: passo de 1 h
	w, _ := a.GetSummary(day.Add(-5*24*time.Hour).UnixMilli(), day.Add(2*24*time.Hour).UnixMilli())
	if w.StepMs != time.Hour.Milliseconds() || len(w.Curve) != 168 {
		t.Fatalf("semana: passo %d, %d pontos", w.StepMs, len(w.Curve))
	}
}
