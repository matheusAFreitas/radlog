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

// whyFixture: 30 dias de fundo nos dois aparelhos; na última hora o GMC vai
// a gmcUp× e o RC102 a rcUp×.
func whyFixture(t *testing.T, gmcUp, rcUp float64) (*App, time.Time) {
	a, _ := testApp(t)
	r := rand.New(rand.NewSource(3))
	end := time.Now().Truncate(time.Minute)
	start := end.Add(-30 * 24 * time.Hour)
	var pts []store.Point
	files := map[string]*strings.Builder{}
	for m := start; m.Before(end); m = m.Add(time.Minute) {
		up := m.After(end.Add(-time.Hour))
		g := poissonR(r, 20*map[bool]float64{true: gmcUp, false: 1}[up])
		pts = append(pts, store.Point{T: m, CPM: g, Src: store.SrcDevice})
		cps := 10 * map[bool]float64{true: rcUp, false: 1}[up]
		k := m.Format("2006-01")
		if files[k] == nil {
			files[k] = &strings.Builder{}
		}
		c := poissonR(r, cps*60) / 60
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
	return a, end
}

func whyState(w WhyRise, key string) string {
	for _, c := range w.Causes {
		if c.Key == key {
			return c.State
		}
	}
	return "-"
}

func TestWhyRise(t *testing.T) {
	// só o GMC subiu 50%: coisa do aparelho
	a, end := whyFixture(t, 1.5, 1)
	w, err := a.GetWhyRise("gmc", 0)
	if err != nil {
		t.Fatal(err)
	}
	if w.NoData || w.Acaso || w.Pct < 30 || w.Pct > 70 || w.Sigma < 5 {
		t.Fatalf("subida: %+v", w)
	}
	if d := time.UnixMilli(w.From); d.Before(end.Add(-80*time.Minute)) || d.After(end.Add(-40*time.Minute)) {
		t.Fatalf("começo da subida %s (fim %s)", d, end)
	}
	if whyState(w, "ambos") != "no" || whyState(w, "aparelho") != "yes" || whyState(w, "chuva") != "na" || whyState(w, "acaso") != "no" {
		t.Fatalf("causas: %+v", w.Causes)
	}
	if !strings.Contains(w.Verdict, "Só o GMC-800 subiu") || len(w.Bins) != 72 || w.Causes[0].State != "yes" {
		t.Fatalf("veredito %q, %d blocos", w.Verdict, len(w.Bins))
	}
	// uma anotação na hora: objeto perto, e o veredito cita a nota
	a.notes = append(a.notes, Note{ID: 1, Time: end.Add(-50 * time.Minute).UnixMilli(), Text: "pedra"})
	w, _ = a.GetWhyRise("gmc", 0)
	if whyState(w, "objeto") != "yes" || whyState(w, "aparelho") != "no" || !strings.Contains(w.Verdict, "pedra") {
		t.Fatalf("objeto: %q %+v", w.Verdict, w.Causes)
	}

	// os dois subiram
	a, _ = whyFixture(t, 1.5, 1.5)
	w, _ = a.GetWhyRise("radiacode", 0)
	if whyState(w, "ambos") != "yes" || !strings.Contains(w.Verdict, "radiação de verdade") || w.Device != devRC {
		t.Fatalf("ambos: %q %+v", w.Verdict, w.Causes)
	}

	// nada subiu: acaso
	a, _ = whyFixture(t, 1, 1)
	w, _ = a.GetWhyRise("gmc", 0)
	if !w.Acaso || whyState(w, "acaso") != "yes" || !strings.HasPrefix(w.Verdict, "Provavelmente é só o acaso") {
		t.Fatalf("acaso: %+v", w)
	}
}
