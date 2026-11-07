package app

import (
	"math"
	"math/rand"
	"testing"
)

// espectro de mentira: contínuo + picos (área em contagens), com ruído de Poisson
func actFake(rng *rand.Rand, sec float64, peaks map[float64]float64, cont float64) actSpec {
	s := actSpec{C: make([]float64, 1024), Sec: sec, A: [3]float64{0, 3, 0}}
	for i := range s.C {
		e0, e1 := s.energy(float64(i)), s.energy(float64(i+1))
		ec := (e0 + e1) / 2
		lam := cont * sec * math.Exp(-ec/400) * (e1 - e0)
		for kev, area := range peaks {
			sg := specFWHM(kev) / 2.3548
			lam += area * 0.5 * (math.Erf((e1-kev)/(sg*math.Sqrt2)) - math.Erf((e0-kev)/(sg*math.Sqrt2)))
		}
		s.C[i] = poissonF(rng, lam)
	}
	return s
}

func poissonF(rng *rand.Rand, lam float64) float64 {
	if lam > 50 {
		return math.Max(0, math.Round(lam+math.Sqrt(lam)*rng.NormFloat64()))
	}
	l, k, p := math.Exp(-lam), 0.0, 1.0
	for {
		p *= rng.Float64()
		if p <= l {
			return k
		}
		k++
	}
}

func TestActFit(t *testing.T) {
	rng := rand.New(rand.NewSource(1))
	// fundo de 2 h com K-40 e Bi-214; amostra de 1 h com mais K-40 (5000 no pico) e tório
	bg := actFake(rng, 7200, map[float64]float64{1460.8: 4000, 609.3: 3000, 351.9: 2400}, 0.4)
	smp := actFake(rng, 3600, map[float64]float64{1460.8: 2000 + 5000, 609.3: 1500, 351.9: 1200, 583.2: 6000, 238.6: 12000, 2614.5: 1500}, 0.6)
	for kev, want := range map[float64]float64{1460.8: 5000, 583.2: 6000, 238.6: 12000, 2614.5: 1500, 609.3: 0} {
		pk := actFit(smp, bg, kev)
		if !pk.ok || math.Abs(pk.Area-want) > 3.5*pk.Err+0.03*want {
			t.Errorf("%v keV: %.0f ± %.0f (queria %v)", kev, pk.Area, pk.Err, want)
		}
	}
	// calibração com KCl: 500 g de KCl puro → 8,3 kBq; ε = 5000/(A·0,1066·3600)
	cal, aK, _ := actCalKCl(smp, bg, 500, 100)
	wantA := 500 * actKPerKCl * actBqPerGK
	if math.Abs(aK-wantA) > 1 || cal == nil || math.Abs(cal.Eff-5000/(wantA*0.1066*3600)) > 3*cal.EffErr*cal.Eff {
		t.Fatalf("KCl: %+v %v", cal, aK)
	}
	// e a mesma amostra medida como K-40 dá a atividade de volta
	res := actAnalyze(smp, bg, []string{"K-40", "Cs-137"}, ActCal{KCl: cal}, 500)
	if !res[0].Detected || math.Abs(res[0].Bq-wantA) > 2*res[0].StatErr || res[0].BqKg <= 0 {
		t.Fatalf("K-40: %+v", res[0])
	}
	if res[1].Detected || res[1].MDA <= 0 {
		t.Fatalf("Cs-137 não tem: %+v", res[1])
	}
}

func TestActCalTh(t *testing.T) {
	rng := rand.New(rand.NewSource(2))
	// eficiência de verdade: ε(E) = 0,004·(E/583)^-1,2; eletrodo de 2% com 10 anos
	eff := func(e float64) float64 { return 0.004 * math.Pow(e/583.2, -1.2) }
	_, f228, _ := actThFactors(10)
	aTh := 1.0 * 0.02 * actThPerTO2 * actBqPerGTh * f228 // 1 g de eletrodo
	sec := 3600.0
	peaks := map[float64]float64{}
	for _, l := range []actLine{{238.6, 0.436, ""}, {583.2, 0.304, ""}, {2614.5, 0.356, ""}} {
		peaks[l.KeV] = aTh * l.Yield * eff(l.KeV) * sec * 40 // eletrodo forte (40 g)
	}
	bg := actFake(rng, 7200, map[float64]float64{1460.8: 4000}, 0.4)
	smp := actFake(rng, sec, peaks, 0.4)
	cal, lines := actCalTh(smp, bg, 40, 2, 10)
	if cal == nil || len(lines) != 3 {
		t.Fatalf("tório: %+v %+v", cal, lines)
	}
	if math.Abs(cal.B+1.2) > 3*cal.BErr+0.05 {
		t.Fatalf("inclinação %v ± %v", cal.B, cal.BErr)
	}
	got, _, src := ActCal{Th: cal}.eff(1000)
	if src != "torio" || math.Abs(got/eff(1000)-1) > 0.15 {
		t.Fatalf("ε(1000) %v, queria %v", got, eff(1000))
	}
	// fatores do tório: recém-purificado = 0 de rádio e 1 de Th-228; muito velho = 1 e 1
	if ra, th, _ := actThFactors(0.001); ra > 0.01 || math.Abs(th-1) > 0.01 {
		t.Fatalf("novo: %v %v", ra, th)
	}
	if ra, th, _ := actThFactors(80); math.Abs(ra-1) > 0.01 || math.Abs(th-1) > 0.02 {
		t.Fatalf("velho: %v %v", ra, th)
	}
}
