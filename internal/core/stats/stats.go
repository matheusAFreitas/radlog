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

// MedianCounts é a mediana para contagens inteiras (contagens por minuto
// do GMC-800), interpolada dentro do valor do meio como dado agrupado:
// L − 0,5 + (n/2 − abaixo)/no_valor. Com ~20 contagens por minuto a mediana
// comum só anda de 1 em 1 (degraus de ~5%); esta anda suave e continua
// resistente a picos. Valores não inteiros são arredondados. 0 se vazio.
func MedianCounts(v []float64) float64 {
	n := len(v)
	if n == 0 {
		return 0
	}
	s := make([]float64, n)
	for i, x := range v {
		s[i] = math.Round(x)
	}
	sort.Float64s(s)
	m := s[(n-1)/2] // valor que contém a metade (o de baixo, se n é par)
	lo := sort.SearchFloat64s(s, m)
	hi := sort.SearchFloat64s(s, m+0.5)
	return m - 0.5 + (float64(n)/2-float64(lo))/float64(hi-lo)
}

// MedianCountsScaled devolve a mediana de u (µSv/h dos mesmos minutos de
// cpm) corrigida na mesma proporção que MedianCounts corrige a de cpm.
func MedianCountsScaled(cpm, u []float64) float64 {
	m := Median(cpm)
	if m <= 0 {
		return Median(u)
	}
	return Median(u) * MedianCounts(cpm) / m
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
