package app

import (
	"math"
	"strings"
)

// ---------- dose em coisas conhecidas (2.41) ----------
//
// "2,9 µSv" não diz muito. Cada dose ganha duas linhas: quanto tempo de fundo
// da casa isso é (a comparação mais honesta) e uma coisa conhecida da régua
// (voo, raio-X, tomografia; a banana saiu na auditoria de 12/11: o corpo
// mantém o potássio constante, comer banana não soma dose) que dê um número entre 1 e 30. Nunca
// compara com acidentes. É ordem de grandeza: no máximo dois dígitos. A tela
// faz a mesma conta em regua.js (doseKnown).

type knownRef struct {
	v         float64 // µSv
	one, many string
	half      string // "≈ meia hora de voo" (só a menor)
}

// da maior para a menor; a primeira que der entre 1 e 30 vale
var doseKnownRefs = []knownRef{
	{7000, "tomografia de tórax", "tomografias de tórax", ""},
	{20, "raio-X de tórax", "raios-X de tórax", ""},
	{4, "hora de voo", "horas de voo", "≈ meia hora de voo"},
}

// knownNum arredonda: meio a meio até 10, depois dois dígitos.
func knownNum(x float64) float64 {
	if x < 10 {
		return math.Max(1, math.Round(x*2)/2)
	}
	if x < 100 {
		return math.Round(x)
	}
	p := math.Pow(10, math.Floor(math.Log10(x))-1)
	return math.Round(x/p) * p
}

func knownTxt(n float64) string {
	if n != math.Trunc(n) {
		return fmtBR(n, 1)
	}
	return fmtBR(n, 0)
}

// doseHomeTime: "≈ 6 h do fundo da sua casa" (bg em µSv/h).
func doseHomeTime(uSv, bg float64) string {
	if !(uSv > 0) || !(bg > 0) {
		return ""
	}
	h := uSv / bg
	one := func(n float64, a, b string) string {
		w := b
		if n == 1 {
			w = a
		}
		return "≈ " + knownTxt(n) + " " + w + " do fundo da sua casa"
	}
	d := h / 24
	switch {
	case h < 1.0/60:
		return "menos de 1 min do fundo da sua casa"
	case h < 1:
		return "≈ " + knownTxt(knownNum(h*60)) + " min do fundo da sua casa"
	case h < 24:
		return "≈ " + knownTxt(knownNum(h)) + " h do fundo da sua casa"
	case d < 14:
		return one(knownNum(d), "dia", "dias")
	case d < 61:
		return one(knownNum(d/7), "semana", "semanas")
	case d < 335:
		return one(knownNum(d/30.44), "mês", "meses")
	}
	return one(knownNum(d/365.25), "ano", "anos")
}

// doseKnownThing: "≈ 3 horas de voo", "≈ 1 raio-X de tórax"; "" se nenhuma der.
func doseKnownThing(uSv float64) string {
	if !(uSv > 0) {
		return ""
	}
	for _, k := range doseKnownRefs {
		if x := uSv / k.v; x >= 0.75 && x <= 30.5 {
			n := knownNum(x)
			w := k.many
			if n == 1 {
				w = k.one
			}
			return "≈ " + knownTxt(n) + " " + w
		}
	}
	last := doseKnownRefs[len(doseKnownRefs)-1]
	if x := uSv / last.v; x >= 0.4 && x < 0.75 {
		return last.half
	}
	return ""
}

// doseKnown junta as duas linhas com " · " ("" se nenhuma der).
func doseKnown(uSv, bg float64) string {
	var out []string
	for _, s := range []string{doseHomeTime(uSv, bg), doseKnownThing(uSv)} {
		if s != "" {
			out = append(out, s)
		}
	}
	return strings.Join(out, " · ")
}
