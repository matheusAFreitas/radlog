package app

import (
	"math"
	"math/rand"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"radlog/internal/gmc"
	"radlog/internal/store"
)

func TestRingWrapsAround(t *testing.T) {
	r := newRing()
	base := time.Now().Add(-2 * time.Hour)
	for i := 0; i < ringSize+100; i++ {
		r.add(base.Add(time.Duration(i)*time.Second), float64(i))
	}
	all := r.between(base, base.Add(3*time.Hour))
	if len(all) != ringSize || all[0].cpm != 100 || all[len(all)-1].cpm != ringSize+99 {
		t.Fatalf("%d amostras, primeira %v, última %v", len(all), all[0].cpm, all[len(all)-1].cpm)
	}
}

func TestPoissonMean(t *testing.T) {
	rng := rand.New(rand.NewSource(1))
	for _, lambda := range []float64{0.3, 5, 200} {
		sum := 0
		for i := 0; i < 20000; i++ {
			sum += poisson(rng, lambda)
		}
		if m := float64(sum) / 20000; math.Abs(m-lambda) > 0.05*lambda+0.02 {
			t.Fatalf("lambda %v: média %v", lambda, m)
		}
	}
}

func TestDemoDoesNotTouchFiles(t *testing.T) {
	dir := t.TempDir()
	st, _ := store.Open(dir)
	a := NewApp()
	a.dataDir, a.store, a.cal = dir, st, gmc.DefaultCalibration()
	a.settings = defaultSettings() // alarme em 100 CPM

	// um minuto real gravado há 10 min
	realT := time.Now().Truncate(time.Minute).Add(-10 * time.Minute)
	if err := st.Add(store.Point{T: realT, CPM: 20, Src: store.SrcLive}); err != nil {
		t.Fatal(err)
	}
	before := snapshotDir(t, dir)

	a.StartDemo(500, false)
	time.Sleep(2500 * time.Millisecond)

	c, err := a.GetChart("15m")
	if err != nil {
		t.Fatal(err)
	}
	var last *float64
	for _, v := range c.Values {
		if v != nil {
			last = v
		}
	}
	if last == nil || *last != 500 || len(c.Values) != 180 {
		t.Fatalf("gráfico de 15 min: último %v, %d pontos", last, len(c.Values))
	}

	// simula um minuto demo fechado sobre o minuto real
	a.mu.Lock()
	a.demo.minutes[realT.Unix()] = 500
	a.mu.Unlock()
	pts, _ := a.points(realT, realT.Add(time.Minute))
	if len(pts) != 1 || pts[0].CPM != 500 || pts[0].Src != "demo" {
		t.Fatalf("sobreposição: %+v", pts)
	}
	d, _ := a.GetDose("3h", "publico")
	if d.MeasuredMinutes != 1 || math.Abs(d.Total-500*10.0/1538/60) > 1e-9 {
		t.Fatalf("dose no demo: %+v", d.Total)
	}
	if d.BackgroundRate > 0.2 { // fundo continua vindo dos dados reais
		t.Fatalf("fundo contaminado pelo demo: %v", d.BackgroundRate)
	}

	evs := a.GetEvents("")
	joined := ""
	for _, e := range evs {
		joined += e.Text + " | "
	}
	if !strings.Contains(joined, "[demo] Alarme: 500 CPM") {
		t.Fatalf("eventos: %s", joined)
	}

	a.StopDemo()
	pts, _ = a.points(realT, realT.Add(time.Minute))
	if len(pts) != 1 || pts[0].CPM != 20 {
		t.Fatalf("depois do demo: %+v", pts)
	}
	for _, e := range a.GetEvents("") {
		if strings.HasPrefix(e.Text, demoPrefix) {
			t.Fatalf("evento demo ficou: %s", e.Text)
		}
	}
	if after := snapshotDir(t, dir); after != before {
		t.Fatalf("o demo mexeu nos arquivos:\nantes:\n%s\ndepois:\n%s", before, after)
	}
	if a.GetDemo().Active {
		t.Fatal("demo continuou ativo")
	}
}

// snapshotDir lista nome, tamanho e conteúdo de todos os arquivos.
func snapshotDir(t *testing.T, dir string) string {
	var b strings.Builder
	filepath.Walk(dir, func(p string, info os.FileInfo, err error) error {
		if err == nil && !info.IsDir() {
			data, _ := os.ReadFile(p)
			b.WriteString(p + "\n" + string(data) + "\n")
		}
		return nil
	})
	return b.String()
}
