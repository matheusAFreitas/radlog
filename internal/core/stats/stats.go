// Package stats tem a estatística simples que o app usa em várias telas
// (mediana, desvio de Poisson). Não depende da interface nem do Wails: o app
// Android vai usar o mesmo pacote.
package stats

import (
	"math"
	"sort"
)

// Median devolve a mediana de v (0 se vazio). Não mexe em v.
func Median(v []float64) float64 {
	s := append([]float64(nil), v...)
	sort.Float64s(s)
	n := len(s)
	if n == 0 {
		return 0
	}
	if n%2 == 1 {
		return s[n/2]
	}
	return (s[n/2-1] + s[n/2]) / 2
}

// Sigma diz quantos desvios de Poisson (√fundo) o CPM está acima (ou abaixo)
// do fundo. nil se o fundo não for positivo.
func Sigma(cpm, bg float64) *float64 {
	if bg <= 0 {
		return nil
	}
	z := (cpm - bg) / math.Sqrt(bg)
	return &z
}
