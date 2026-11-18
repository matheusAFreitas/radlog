package app

import (
	"math"
	"regexp"
	"strconv"
	"strings"
)

// Unidades da tela (2.84.0, revisão da interface): a taxa em µSv/h ou
// mrem/h (rateUnit) e a contagem em CPM ou CPS (countUnit), para o app todo.
// O que o tubo mede (calibração, parte técnica) continua em CPM e µSv/h; o
// app grava tudo em µSv e CPM e só converte ao mostrar.

const (
	rateUSv  = "usv"
	rateMrem = "mrh"
	countCPM = "cpm"
	countCPS = "cps"
)

func normRateUnit(u string) string {
	if u == rateMrem {
		return rateMrem
	}
	return rateUSv
}

func normCountUnit(u string) string {
	if u == countCPM {
		return countCPM
	}
	return countCPS
}

// unitNum é um número como o app escreve (vírgula ou ponto decimal, com ou
// sem separador de milhar) seguido, com ou sem espaço, de µSv ou mSv (com
// /h, /ano, /dia...). Faixas ("0,10 a 0,20 µSv/h", "0,1–0,2 µSv/h") entram
// juntas, para os dois números mudarem.
var unitRe = regexp.MustCompile(`([+±−-]?\d[\d.,]*)((?:\s?(?:–|-|±|→|a|e|até|ou|para|to|and|or)\s?[+±−-]?\d[\d.,]*)*)(\s|\x{00A0})?(µSv|mSv)(/h|/ano|/dia|/year|/day|\b)`)

// "0,52 µSv/h (limite 0,50)": o limite entre parênteses é na mesma unidade;
// ganha a unidade escrita para mudar junto
var unitParenRe = regexp.MustCompile(`(\d\s?)µSv/h \(([^()]{0,40}?\s)(\d[\d.,]*\d|\d)\)`)

var (
	unitNamedRe = regexp.MustCompile(`[A-Za-zÀ-ÿ]+-\d+`)
	unitKeptRe  = regexp.MustCompile("+")
)

var unitNumRe = regexp.MustCompile(`[+±−-]?\d[\d.,]*`)

// remText troca µSv e mSv por mrem nos textos com número (1 µSv = 0,1 mrem;
// 1 mSv = 100 mrem). sep é o separador decimal do idioma do texto.
func remText(s, sep string) string {
	if !strings.Contains(s, "Sv") {
		return s
	}
	// 2.101.5: nome com número ("GMC-800", "Cs-137") não é número de taxa
	var kept []string
	s = unitNamedRe.ReplaceAllStringFunc(s, func(m string) string {
		kept = append(kept, m)
		return "" + strings.Repeat("", len(kept)) + ""
	})
	s = unitParenRe.ReplaceAllString(s, "${1}µSv/h ($2$3 µSv/h)")
	s = unitRe.ReplaceAllStringFunc(s, func(m string) string {
		p := unitRe.FindStringSubmatch(m)
		k := 0.1
		if p[4] == "mSv" {
			k = 100
		}
		nums := unitNumRe.ReplaceAllStringFunc(p[1]+p[2], func(n string) string { return remNum(n, sep, k) })
		return nums + p[3] + "mrem" + p[5]
	})
	if len(kept) > 0 {
		s = unitKeptRe.ReplaceAllStringFunc(s, func(m string) string {
			return kept[strings.Count(m, "")-1]
		})
	}
	return s
}

// remNum converte um número escrito, mantendo o sinal, o separador e uma
// casa a mais quando divide (0,26 → 0,026).
func remNum(n, sep string, k float64) string {
	sign := ""
	if strings.ContainsAny(n[:1], "+±−-") {
		r := []rune(n)
		sign, n = string(r[0]), string(r[1:])
	}
	n = strings.TrimRight(n, ".,")
	th := "."
	if sep == "." {
		th = ","
	}
	clean := strings.ReplaceAll(n, th, "")
	dec := 0
	if i := strings.Index(clean, sep); i >= 0 {
		dec = len(clean) - i - 1
	}
	v, err := strconv.ParseFloat(strings.Replace(clean, sep, ".", 1), 64)
	if err != nil {
		return sign + n
	}
	v *= k
	if k < 1 {
		dec++
	} else {
		dec = int(math.Max(0, float64(dec-2)))
	}
	out := strconv.FormatFloat(v, 'f', dec, 64)
	if sep != "." {
		out = strings.Replace(out, ".", sep, 1)
	}
	return sign + out
}
