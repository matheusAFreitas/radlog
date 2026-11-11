package app

import "testing"

func TestUIScale(t *testing.T) {
	for in, want := range map[float64]float64{0: 1, 1: 1, 0.5: 0.9, 1.1: 1.08, 1.2: 1.15, 1.4: 1.3, 3: 1.5} {
		if got := normUIScale(in); got != want {
			t.Errorf("normUIScale(%v) = %v, quero %v", in, got, want)
		}
	}
	if normUIDensity("compacto") != "compacto" || normUIDensity("x") != "" {
		t.Error("densidade")
	}
}
