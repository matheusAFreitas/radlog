package app

import (
	"fmt"
	"math"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"radlog/internal/store"
)

func TestRainImpact(t *testing.T) {
	a, _ := testApp(t)
	a.settings.RainEnabled, a.settings.RainLat, a.settings.RainLon = true, -15.79, -47.88
	now := time.Now().Truncate(time.Hour)
	start := now.Add(-10 * 24 * time.Hour)
	a.rain = &rainCache{V: rainCacheVersion, Lat: -15.79, Lon: -47.88, Hours: map[int64]float64{}}
	for h := start; h.Before(now); h = h.Add(time.Hour) {
		a.rain.Hours[h.Unix()] = 0
	}
	rains := []time.Time{now.Add(-8 * 24 * time.Hour), now.Add(-5*24*time.Hour + 6*time.Hour), now.Add(-2 * 24 * time.Hour)}
	for _, r := range rains {
		a.rain.Hours[r.Unix()] = 3 // 3 mm numa hora
	}
	// radar numa delas (deve virar "radar+modelo"/"radar")
	a.radar = &radarStore{V: radarVersion, Lat: -15.79, Lon: -47.88, Points: map[int64]RadarPoint{}}
	r2 := rains[2]
	for i := 0; i < 6; i++ {
		k := r2.Add(time.Duration(i) * 10 * time.Minute).Unix()
		a.radar.Points[k] = RadarPoint{Time: k * 1000, DBZ: 35, MMh: 5}
	}

	var pts []store.Point
	i := 0
	for m := start; m.Before(now); m = m.Add(time.Minute) {
		v := 25 + float64((i*7)%5-2) // ruído pequeno e fixo
		for _, r := range rains {
			d := m.Sub(r)
			if d >= 30*time.Minute && d < 2*time.Hour {
				v += 6
			}
		}
		pts = append(pts, store.Point{T: m, CPM: v})
		i++
	}
	if _, err := a.store.ImportNew(pts); err != nil {
		t.Fatal(err)
	}
	res, err := a.GetRainImpact()
	if err != nil {
		t.Fatal(err)
	}
	if res.N != 3 {
		t.Fatalf("episódios: %d %+v", res.N, res.Episodes)
	}
	if res.AvgRise < 5 || res.DryRise > 2.5 || res.Effect < 3 || res.DryN < 10 {
		t.Fatalf("subida %.1f, sem chuva %.1f (%d), efeito %.1f", res.AvgRise, res.DryRise, res.DryN, res.Effect)
	}
	if res.PeakMin < 30 || res.PeakMin > 120 || res.BackN != 3 || res.BackMin < 90 || res.BackMin > 180 {
		t.Fatalf("pico %d min, volta %d min (%d)", res.PeakMin, res.BackMin, res.BackN)
	}
	if res.Episodes[0].Source != "radar" || res.Episodes[1].Source != "modelo" {
		t.Fatalf("fontes: %s %s", res.Episodes[0].Source, res.Episodes[1].Source)
	}
	if len(res.Curve) != 24 || res.Curve[12].Rain == nil || res.Curve[0].Dry == nil {
		t.Fatalf("curva: %d pontos", len(res.Curve))
	}

	t.Logf("N=%d subida=%.2f seco=%.2f(%d) efeito=%.2f pico=%dmin volta=%dmin", res.N, res.AvgRise, res.DryRise, res.DryN, res.Effect, res.PeakMin, res.BackMin)
	// chuva desligada: vazio
	a.settings.RainEnabled = false
	if r, _ := a.GetRainImpact(); r.Enabled || r.N != 0 {
		t.Fatal("devia vir vazio")
	}
}

func TestRainNow(t *testing.T) {
	now := time.Now().Truncate(time.Minute)
	m := map[int64]float64{}
	for x := now.Add(-6 * time.Hour); x.Before(now); x = x.Add(time.Minute) {
		m[x.Unix()] = 25
	}
	all := func(t0, t1 time.Time) bool { return true }
	from := now.Add(-60 * 24 * time.Hour)
	cur := rainSpan{start: now.Add(-40 * time.Minute), end: now.Add(-5 * time.Minute), mm: 0.6, max: 1, radar: true}

	r := rainNow([]rainSpan{cur}, m, now, cur.end.Add(-10*time.Minute), from, all)
	if r == nil || r.Status != "medindo" || !r.Raining || !r.HasBase || r.ResultAt != cur.start.Add(2*time.Hour).UnixMilli() {
		t.Fatalf("medindo: %+v", r)
	}
	old := rainSpan{start: now.Add(-3 * time.Hour), end: now.Add(-2 * time.Hour)}
	if r := rainNow([]rainSpan{old, cur}, m, now, now, from, all); r == nil || r.Status != "ignorada" || r.PrevEnd == 0 {
		t.Fatalf("ignorada: %+v", r)
	}
	long := rainSpan{start: now.Add(-150 * time.Minute), end: now.Add(-100 * time.Minute)}
	// parou: o último bloco com dado já veio seco
	if r := rainNow([]rainSpan{cur}, m, now, cur.end, from, all); r == nil || r.Raining {
		t.Fatalf("parou: %+v", r)
	}
	if r := rainNow([]rainSpan{long}, m, now, now, from, all); r == nil || r.Status != "contando" || r.Raining {
		t.Fatalf("contando: %+v", r)
	}
	done := rainSpan{start: now.Add(-6 * time.Hour), end: now.Add(-5 * time.Hour)}
	if r := rainNow([]rainSpan{done}, m, now, now, from, all); r != nil {
		t.Fatalf("já fechada: %+v", r)
	}
}

func TestRainImpactRadiacode(t *testing.T) {
	a, _ := testApp(t)
	a.settings.RainEnabled, a.settings.RainLat, a.settings.RainLon = true, -15.79, -47.88
	now := time.Now().Truncate(time.Hour)
	start := now.Add(-10 * 24 * time.Hour)
	a.rain = &rainCache{V: rainCacheVersion, Lat: -15.79, Lon: -47.88, Hours: map[int64]float64{}}
	for h := start; h.Before(now); h = h.Add(time.Hour) {
		a.rain.Hours[h.Unix()] = 0
	}
	rains := []time.Time{now.Add(-8 * 24 * time.Hour), now.Add(-5*24*time.Hour + 6*time.Hour), now.Add(-2 * 24 * time.Hour)}
	for _, r := range rains {
		a.rain.Hours[r.Unix()] = 3
	}
	// Radiacode: 12 cps (0,10 µSv/h) e +10% por 1,5 h depois da chuva
	files := map[string]*strings.Builder{}
	i := 0
	for m := start; m.Before(now); m = m.Add(time.Minute) {
		cps := 12 + float64((i*7)%5-2)*0.05
		for _, r := range rains {
			if d := m.Sub(r); d >= 30*time.Minute && d < 2*time.Hour {
				cps *= 1.1
			}
		}
		k := m.Format("2006-01")
		if files[k] == nil {
			files[k] = &strings.Builder{}
			files[k].WriteString(rcHeader + "\n")
		}
		fmt.Fprintf(files[k], "%s,%g,%g,80\n", m.Format(time.RFC3339), cps, cps/120)
		i++
	}
	_ = os.MkdirAll(a.rcDir(), 0o755)
	for k, b := range files {
		_ = os.WriteFile(filepath.Join(a.rcDir(), k+".csv"), []byte(b.String()), 0o644)
	}
	res, err := a.GetRainImpactDev("radiacode")
	if err != nil {
		t.Fatal(err)
	}
	if res.Device != "radiacode" || res.N != 3 || math.Abs(res.USvPerCPM-1.0/7200) > 1e-6 {
		t.Fatalf("RC102: %+v", res)
	}
	// +10% de 720 CPM = +72 CPM
	if res.Effect < 50 || res.AvgBase < 700 || res.AvgBase > 740 {
		t.Fatalf("efeito %.1f base %.1f", res.Effect, res.AvgBase)
	}
	// o GMC não tem dado nenhum: nada medido
	if g, _ := a.GetRainImpactDev("gmc"); g.N != 0 || g.Device != "gmc" {
		t.Fatalf("GMC: %+v", g)
	}
}
