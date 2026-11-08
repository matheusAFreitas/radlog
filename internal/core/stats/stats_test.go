package stats

import (
	"math"
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
