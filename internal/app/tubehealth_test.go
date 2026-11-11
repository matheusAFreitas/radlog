package app

import (
	"fmt"
	"math/rand"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"radlog/internal/store"
)

// tubeFixture: 40 dias dos dois aparelhos em casa. gmcK multiplica o GMC nos
// últimos 10 dias; noisy junta ruído além do acaso; zeros grava 5 min em 0.
func tubeFixture(t *testing.T, gmcK float64, noisy, zeros bool) *App {
	a, _ := testApp(t)
	r := rand.New(rand.NewSource(7))
	end := time.Now().Truncate(time.Minute)
	start := end.Add(-40 * 24 * time.Hour)
	var pts []store.Point
	files := map[string]*strings.Builder{}
	for m := start; m.Before(end); m = m.Add(time.Minute) {
		mean := 20.0
		if m.After(end.Add(-10 * 24 * time.Hour)) {
			mean *= gmcK
		}
		if noisy {
			mean *= 1 + 0.5*r.NormFloat64()*0.6
			if mean < 1 {
				mean = 1
			}
		}
		g := poissonR(r, mean)
		if zeros && !m.Before(end.Add(-3*24*time.Hour)) && m.Before(end.Add(-3*24*time.Hour+5*time.Minute)) {
			g = 0
		}
		pts = append(pts, store.Point{T: m, CPM: g, Src: store.SrcLive, Has: true, Raw: g, Corr: g})
		k := m.Format("2006-01")
		if files[k] == nil {
			files[k] = &strings.Builder{}
		}
		c := poissonR(r, 560) / 60 // cps: 28 contagens do RC102 por contagem do GMC
		fmt.Fprintf(files[k], "%s,%.3f,%.5f,%d\n", m.Format(time.RFC3339), c, c*0.01, 60)
	}
	if _, err := a.store.Import(pts); err != nil {
		t.Fatal(err)
	}
	_ = os.MkdirAll(a.rcDir(), 0o755)
	for k, b := range files {
		if err := os.WriteFile(filepath.Join(a.rcDir(), k+".csv"), []byte(b.String()), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	// o RC102 ficou em casa junto com o GMC-800
	if _, err := a.SavePeriod(Period{From: start.UnixMilli(), To: end.Add(time.Minute).UnixMilli(), GMC: WhereHome, RC: WhereHome, Name: "casa"}); err != nil {
		t.Fatal(err)
	}
	return a
}

func TestTubeHealth(t *testing.T) {
	a := tubeFixture(t, 1, false, false)
	h, err := a.GetTubeHealth(true)
	if err != nil {
		t.Fatal(err)
	}
	if h.GapsOK != "ok" || h.DispState != "ok" || h.BurstState != "ok" || h.RatioState != "ok" {
		t.Fatalf("saudável: gaps %s disp %s (%.2f) rajadas %s razão %s %s (%.1f→%.1f)", h.GapsOK, h.DispState, h.Disp, h.BurstState, h.RatioState, h.RatioWhy, h.R0, h.R1)
	}
	if h.Disp < .9 || h.Disp > 1.1 || len(h.Hist) < 10 {
		t.Fatalf("dispersão %.2f, %d barras", h.Disp, len(h.Hist))
	}
	// cache: sem force devolve o mesmo
	if h2, _ := a.GetTubeHealth(false); h2.At != h.At {
		t.Fatal("deveria usar o guardado")
	}

	if h, _ := tubeFixture(t, 0.85, false, false).GetTubeHealth(true); h.RatioState != "warn" || h.RatioChg < .1 {
		t.Fatalf("sensibilidade caindo: %s %.3f", h.RatioState, h.RatioChg)
	}
	if h, _ := tubeFixture(t, 1, true, false).GetTubeHealth(true); h.DispState != "warn" {
		t.Fatalf("ruído: %s %.2f", h.DispState, h.Disp)
	}
	if h, _ := tubeFixture(t, 1, false, true).GetTubeHealth(true); h.GapsOK != "warn" || len(h.Gaps) != 1 || h.Gaps[0].Min != 5 {
		t.Fatalf("parou: %+v", h.Gaps)
	}
}

func TestTubeDue(t *testing.T) {
	loc := time.Local
	at := time.Date(2026, 11, 10, 7, 0, 0, 0, loc)
	if tubeDue(at, time.Date(2026, 11, 11, 5, 0, 0, 0, loc)) {
		t.Fatal("antes das 6 ainda vale o de ontem")
	}
	if !tubeDue(at, time.Date(2026, 11, 11, 6, 1, 0, 0, loc)) {
		t.Fatal("depois das 6 vence")
	}
}
