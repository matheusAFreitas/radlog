package app

import (
	"math"
	"math/rand"
	"sort"
	"testing"
)

// fundo no nível real do RC102 (~13 cps no total, K-40 das paredes ~0,03 cps),
// com a escala de energia que o aparelho manda g vezes a verdadeira
// (o RC102 do ACE: ~0,969, o K-40 aparece em ~1.415 keV)
func gainFake(rng *rand.Rand, sec, salt, g float64) actSpec {
	pk := map[float64]float64{actK40: (0.03 + salt) * sec, 609.3: 0.05 * sec, 1764.5: 0.012 * sec, 1120.3: 0.015 * sec, 351.9: 0.04 * sec, 2614.5: 0.006 * sec, 583.2: 0.01 * sec, 238.6: 0.03 * sec}
	s := actFake(rng, sec, pk, 0.0325)
	return s.scaled(g)
}

// 2.103.0: com a escala 3% baixa, o app tem que achar o K-40 em ~1.415 keV e
// recuperar a área inteira (antes pegava 40–60%)
func TestActAlignShiftedK40(t *testing.T) {
	const g, salt = 1 / 1.0324, 0.047 // a tigela do ACE em 21/11
	rng := rand.New(rand.NewSource(3))
	var old, now, peaks []float64
	cover := 0
	n := 150
	for i := 0; i < n; i++ {
		bg := gainFake(rng, 3600, 0, g)
		smp := gainFake(rng, 3600, salt, g)
		if _, _, a := actCalKCl(smp, bg, 750, 51); a > 0 {
			old = append(old, a/(salt*3600))
		} else {
			old = append(old, 0)
		}
		s2, b2, gn := actAlign(smp, bg, true, 0)
		if !gn.Found {
			t.Fatalf("não achou o K-40 (rodada %d)", i)
		}
		peaks = append(peaks, gn.PeakKeV)
		k, _, area := actCalKCl(s2, b2, 750, 51)
		if k == nil {
			now = append(now, 0)
			continue
		}
		tru := salt * 3600
		now = append(now, area/tru)
		if math.Abs(area-tru) <= k.EffErr*area {
			cover++
		}
	}
	med := func(x []float64) float64 { y := append([]float64(nil), x...); sort.Float64s(y); return y[len(y)/2] }
	if p := med(peaks); math.Abs(p-actK40*g) > 4 {
		t.Fatalf("pico achado em %.1f keV, esperado %.1f", p, actK40*g)
	}
	if m := med(now); m < 0.9 || m > 1.1 {
		t.Fatalf("área/verdade com a escala corrigida: mediana %.2f", m)
	}
	if c := float64(cover) / float64(n); c < 0.55 || c > 0.8 {
		t.Fatalf("±1σ cobre %.0f%% (esperado ~68%%)", c*100)
	}
	// sem corrigir, a área fica bem menor (o bug de antes)
	if m := med(old); m > 0.8 {
		t.Fatalf("sem corrigir a escala a área deveria sair baixa: mediana %.2f", m)
	}
	t.Logf("pico %.1f keV; área/verdade %.2f (sem corrigir %.2f; cobertura %d/%d)", med(peaks), med(now), med(old), cover, n)
}

// a escala certa (ganho 1) não pode ser "corrigida" para longe
func TestActAlignNoShift(t *testing.T) {
	rng := rand.New(rand.NewSource(4))
	var gs []float64
	for i := 0; i < 60; i++ {
		_, _, gn := actAlign(gainFake(rng, 3600, 0.047, 1), gainFake(rng, 3600, 0, 1), true, 0)
		if !gn.Found {
			t.Fatal("não achou o K-40 na escala certa")
		}
		gs = append(gs, gn.Gain)
	}
	sort.Float64s(gs)
	if m := gs[len(gs)/2]; math.Abs(m-1) > 0.003 {
		t.Fatalf("ganho mediano %.4f na escala certa", m)
	}
}

// sem pico (só ruído) não inventa ganho
func TestActAlignNoPeak(t *testing.T) {
	rng := rand.New(rand.NewSource(5))
	s := actFake(rng, 60, map[float64]float64{}, 0.0325)
	if _, _, gn := actAlign(s, actSpec{}, false, 0); gn.Found || gn.Gain != 1 {
		t.Fatalf("achou pico no ruído: %+v", gn)
	}
}

// reamostrar não cria nem perde contagens
func TestActResample(t *testing.T) {
	rng := rand.New(rand.NewSource(6))
	src := actFake(rng, 3600, map[float64]float64{actK40: 500}, 0.5)
	dst := src.scaled(0.97)
	out := actResample(src, dst)
	var a, b float64
	for i := range src.C {
		a += src.C[i]
		b += out.C[i]
	}
	// a parte que sai da grade (acima do último canal) se perde; o resto fica
	if b > a || b < 0.98*a {
		t.Fatalf("total %v → %v", a, b)
	}
}

// amostra com fundo curto (o K-40 das paredes quase não aparece): vale o
// ganho da calibração
func TestActAlignFallback(t *testing.T) {
	rng := rand.New(rand.NewSource(8))
	const g = 1 / 1.0324
	_, _, gn := actAlign(gainFake(rng, 600, 0, g), gainFake(rng, 600, 0, g), false, 1.0324)
	if gn.Found || math.Abs(gn.Gain-1.0324) > 1e-9 {
		t.Fatalf("sem pico no fundo curto devia usar o ganho da calibração: %+v", gn)
	}
	if c := (ActCal{KCl: &ActCalKCl{Eff: 1e-4}}); c.gain() != 0 {
		t.Fatal("calibração de antes da correção não tem ganho")
	}
}
