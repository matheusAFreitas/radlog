package app

import (
	"math"
	"math/rand"
	"strings"
	"testing"
	"time"
)

func TestPoissonTail(t *testing.T) {
	// λ = 25/60: compara com 1 − Σ_{i<k} da fórmula
	l := 25.0 / 60
	direct := func(k int) float64 {
		sum, term := 0.0, math.Exp(-l)
		for i := 0; i < k; i++ {
			sum += term
			term *= l / float64(i+1)
		}
		return 1 - sum
	}
	for _, k := range []int{1, 3, 6, 7} {
		if v, w := poissonTail(float64(k), l), direct(k); math.Abs(v-w) > 1e-9*math.Max(1, w) && math.Abs(v-w)/w > 1e-6 {
			t.Fatalf("P(X≥%d) = %g, fórmula %g", k, v, w)
		}
	}
	if v := poissonTail(7, l); v > ifTailMax {
		t.Fatalf("7 contagens em 1 s no fundo deveria passar do limite: %g", v)
	}
	if v := poissonTail(0, 3); v != 1 {
		t.Fatal(v)
	}
	if v := poissonTail(2, 10); math.Abs(v-(1-math.Exp(-10)*11)) > 1e-12 {
		t.Fatalf("lado de baixo: %g", v)
	}
	if v := poissonTail(5000, 0.4); v != 0 {
		t.Fatalf("cauda enorme: %g", v)
	}
}

// fundo de 25 CPM com um segundo de ruído: marca; com uma fonte de verdade
// (vários segundos altos): não marca; fundo puro por horas: quase nunca marca.
func TestInterferenceDetector(t *testing.T) {
	rng := rand.New(rand.NewSource(7))
	pois := func(l float64) float64 {
		L, k, p := math.Exp(-l), 0.0, 1.0
		for {
			p *= rng.Float64()
			if p <= L {
				return k
			}
			k++
		}
	}
	t0 := time.Date(2026, 11, 4, 23, 0, 0, 0, time.Local)
	run := func(n int, f func(i int) float64) []*spikeSuspect {
		var d interferenceDetector
		var out []*spikeSuspect
		for i := 0; i < n; i++ {
			if s := d.step(t0.Add(time.Duration(i)*time.Second), f(i), 25); s != nil {
				out = append(out, s)
			}
		}
		return out
	}
	got := run(120, func(i int) float64 {
		if i == 60 {
			return 14
		}
		return pois(25.0 / 60)
	})
	if len(got) != 1 || !got[0].At.Equal(t0.Add(60*time.Second)) || got[0].Counts != 14 {
		t.Fatalf("ruído: %+v", got)
	}
	// fonte perto do contador por 30 s (≈ 600 CPM)
	got = run(120, func(i int) float64 {
		if i >= 50 && i < 80 {
			return pois(10)
		}
		return pois(25.0 / 60)
	})
	if len(got) != 0 {
		t.Fatalf("fonte de verdade marcada: %+v", got)
	}
	// 6 horas só de fundo
	got = run(6*3600, func(int) float64 { return pois(25.0 / 60) })
	if len(got) > 1 {
		t.Fatalf("falso positivo demais: %d", len(got))
	}
	// buraco na leitura recomeça a janela (não compara segundos distantes)
	var d interferenceDetector
	for i := 0; i < 15; i++ {
		d.step(t0.Add(time.Duration(i)*time.Second), 0, 25)
	}
	d.step(t0.Add(time.Minute), 0, 25)
	if len(d.win) != 1 {
		t.Fatalf("janela depois do buraco: %d", len(d.win))
	}
}

func TestInterferenceFlagsAlert(t *testing.T) {
	a, _ := testApp(t)
	now := time.Now().Truncate(time.Second).Add(-time.Minute)
	a.events = append(a.events, Event{ID: 9, Time: now.Add(3 * time.Second).UnixMilli(), Kind: kindAlert, Level: "atencao", Text: "Atenção: 55 CPM"})
	for i := 0; i <= 20; i++ {
		c := 0.0
		if i == 10 {
			c = 40
		}
		a.checkInterference(now.Add(time.Duration(i-10)*time.Second), c)
	}
	var alertText, logText string
	for _, e := range a.events {
		if e.ID == 9 {
			alertText = e.Text
		} else if e.Kind == kindEvent {
			logText = e.Text
		}
	}
	if !strings.Contains(alertText, "possível interferência (40 contagens") {
		t.Fatalf("alerta: %q", alertText)
	}
	if !strings.HasPrefix(logText, "Pico suspeito de interferência: 40 contagens") || classifyLog("evento: "+logText) != lvWarn {
		t.Fatalf("evento: %q", logText)
	}
}
