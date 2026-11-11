package app

import (
	"fmt"
	"math"
	"math/rand"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"radlog/internal/store"
)

// crossFixture: 3 h dos dois aparelhos, alerta às 2 h. gK e rK multiplicam o
// fundo (20 CPM do GMC-800, 560 do RC102) a partir do alerta; rcWhere é onde
// o RC102 estava.
func crossFixture(t *testing.T, gK, rK float64, rcWhere string, notHome bool) (*App, time.Time, time.Time) {
	a, _ := testApp(t)
	r := rand.New(rand.NewSource(5))
	end := time.Now().Truncate(time.Minute).Add(-30 * time.Minute)
	start := end.Add(-3 * time.Hour)
	tA := start.Add(2 * time.Hour)
	var pts []store.Point
	files := map[string]*strings.Builder{}
	for m := start; m.Before(end); m = m.Add(time.Minute) {
		g, c := 20.0, 560.0
		if !m.Before(tA) {
			g, c = g*gK, c*rK
		}
		gv := poissonR(r, g)
		pts = append(pts, store.Point{T: m, CPM: gv, Src: store.SrcLive, Has: true, Raw: gv, Corr: gv})
		k := m.Format("2006-01")
		if files[k] == nil {
			files[k] = &strings.Builder{}
		}
		cps := math.Round(c+r.NormFloat64()*math.Sqrt(c)) / 60 // Poisson grande ~ normal
		fmt.Fprintf(files[k], "%s,%.3f,%.5f,%d\n", m.Format(time.RFC3339), cps, cps*0.01, 60)
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
	if _, err := a.SavePeriod(Period{From: start.UnixMilli(), To: end.Add(time.Minute).UnixMilli(), GMC: WhereHome, RC: rcWhere, NotHome: notHome, Name: "teste"}); err != nil {
		t.Fatal(err)
	}
	return a, tA, end
}

func TestCrossCheck(t *testing.T) {
	// os dois sobem: confirmado
	a, tA, end := crossFixture(t, 2.5, 2.5, WhereHome, false)
	c := a.GetCrossCheck("gmc", tA.UnixMilli(), tA.Add(10*time.Minute).UnixMilli(), true)
	if c.State != "ok" || c.ORise < 1 || c.Tag != "confirmado pelo RC102" {
		t.Fatalf("confirmado: %+v", c)
	}
	if c.Series == nil || len(c.Series.Me) != 15 || c.Series.Me[12] == nil || *c.Series.Me[12] < 0.8 {
		t.Fatalf("gráfico: %+v", c.Series)
	}
	// o mesmo pedido de novo vem do guardado
	if c2 := a.GetCrossCheck("gmc", tA.UnixMilli(), tA.Add(10*time.Minute).UnixMilli(), true); c2.Text != c.Text {
		t.Fatal("guardado")
	}
	_ = end

	// só o GMC-800 sobe: não confirmado
	a, tA, _ = crossFixture(t, 2.5, 1, WhereHome, false)
	if c := a.GetCrossCheck("gmc", tA.UnixMilli(), tA.Add(10*time.Minute).UnixMilli(), false); c.State != "no" || c.Tag != "só no GMC-800" {
		t.Fatalf("não confirmado: %+v", c)
	}
	// alerta do RC102, o GMC-800 também subiu: confirmado pelo GMC-800
	a, tA, _ = crossFixture(t, 2.5, 2.5, WhereHome, false)
	if c := a.GetCrossCheck(devRC, tA.UnixMilli(), tA.Add(10*time.Minute).UnixMilli(), false); c.State != "ok" || c.Tag != "confirmado pelo GMC-800" {
		t.Fatalf("RC102: %+v", c)
	}
	// RC102 com você, fora de casa: sem como conferir
	a, tA, _ = crossFixture(t, 2.5, 2.5, WhereMe, true)
	if c := a.GetCrossCheck("gmc", tA.UnixMilli(), tA.Add(10*time.Minute).UnixMilli(), false); c.State != "na" || c.Why != "longe" {
		t.Fatalf("longe: %+v", c)
	}
}

func TestCrossCheckWait(t *testing.T) {
	// alerta do RC102 com subida pequena (+25%): o GMC-800 precisa de uns
	// 7 min; com 2 min de alerta ainda está conferindo
	a, tA, _ := crossFixture(t, 1.25, 1.25, WhereHome, false)
	now := tA.Add(2 * time.Minute)
	c := a.crossCheck(devRC, tA, now, false, now, false)
	if c.State != "wait" || c.Left < 60 || c.Frac <= 0 || c.Frac >= 1 {
		t.Fatalf("conferindo: %+v", c)
	}
	// acabou logo: sem como conferir
	end := tA.Add(time.Minute)
	if c := a.crossCheck(devRC, tA, end, true, tA.Add(30*time.Minute), false); c.State != "na" || c.Why != "curta" {
		t.Fatalf("curta: %+v", c)
	}
	// subida de +50%: com 12 min o GMC-800 já viu
	a, tA, _ = crossFixture(t, 1.5, 1.5, WhereHome, false)
	end = tA.Add(12 * time.Minute)
	if c := a.crossCheck(devRC, tA, end, true, tA.Add(30*time.Minute), false); c.State != "ok" {
		t.Fatalf("12 min: %+v", c)
	}
}
