package stats

import (
	"math"
	"math/rand"
	"testing"
)

func TestMedian(t *testing.T) {
	cases := []struct {
		in   []float64
		want float64
	}{
		{nil, 0},
		{[]float64{5}, 5},
		{[]float64{3, 1, 2}, 2},
		{[]float64{4, 1, 3, 2}, 2.5},
	}
	for _, c := range cases {
		if got := Median(c.in); got != c.want {
			t.Errorf("Median(%v) = %v, quero %v", c.in, got, c.want)
		}
	}
	v := []float64{3, 1, 2}
	Median(v)
	if v[0] != 3 || v[1] != 1 {
		t.Fatal("Median mexeu na entrada")
	}
}

func TestSigma(t *testing.T) {
	if Sigma(30, 0) != nil {
		t.Fatal("fundo zero tem que dar nil")
	}
	if z := Sigma(30, 25); z == nil || math.Abs(*z-1) > 1e-9 {
		t.Fatalf("Sigma(30, 25) = %v, quero 1", z)
	}
	if z := Sigma(20, 25); z == nil || math.Abs(*z+1) > 1e-9 {
		t.Fatalf("Sigma(20, 25) = %v, quero -1", z)
	}
}

func TestMedianCounts(t *testing.T) {
	if MedianCounts(nil) != 0 {
		t.Fatal("vazio")
	}
	// 20 e 21 meio a meio: 20,5 (a comum também dá 20,5)
	if m := MedianCounts([]float64{20, 21, 20, 21}); math.Abs(m-20.5) > 1e-9 {
		t.Fatalf("%v", m)
	}
	// um valor só repetido: o próprio valor
	if m := MedianCounts([]float64{7, 7, 7}); m != 7 {
		t.Fatalf("%v", m)
	}
	// Poisson: a interpolada acompanha a média de perto, a comum anda de 1 em 1
	r := rand.New(rand.NewSource(3))
	for _, lam := range []float64{18, 20.4, 22.03, 25} {
		v := make([]float64, 20000)
		for i := range v {
			v[i] = float64(poissonT(r, lam))
		}
		// a interpolada fica a uma distância quase fixa de λ (sem degraus)
		if m := MedianCounts(v); math.Abs(m-(lam-1.0/6)) > 0.12 {
			t.Fatalf("λ %.2f: %.3f", lam, m)
		}
	}
}

func poissonT(r *rand.Rand, l float64) int {
	L, k, p := math.Exp(-l), 0, 1.0
	for {
		p *= r.Float64()
		if p <= L {
			return k
		}
		k++
	}
}
