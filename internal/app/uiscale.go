package app

import "math"

// ---------- tamanho e densidade da janela (2.49) ----------
//
// Configurações › Janela: o tamanho multiplica os tokens de fonte do CSS
// (--ui-scale) e o espaço multiplica os de espaço (--ui-density). Só os
// passos da spec (ux/ideias-7-15-2.html); fora deles, o mais perto.

var uiScaleSteps = []float64{0.9, 1, 1.08, 1.15, 1.3, 1.5}

func normUIScale(v float64) float64 {
	if v <= 0 || math.IsNaN(v) {
		return 1
	}
	best := 1.0
	for _, s := range uiScaleSteps {
		if math.Abs(s-v) < math.Abs(best-v) {
			best = s
		}
	}
	return best
}

func normUIDensity(d string) string {
	switch d {
	case "compacto", "confortavel":
		return d
	}
	return ""
}
