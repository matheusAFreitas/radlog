package app

import (
	"regexp"
	"strconv"
)

// Formato da data (2.97.0): o app escreve sempre dia/mês ("16/11",
// "16/11/2026"). Com "iso" nas Configurações, o que vai para a tela e para o
// PDF vira mês-dia e ano-mês-dia ("11-16", "2026-11-16"), nos dois idiomas.
// Na tela a troca é do observador (unidades.js, dateText); aqui é a do PDF.
// Mês/dia não existe, de propósito.

const dateISO = "iso"

func normDateFormat(f string) string {
	if f == dateISO {
		return dateISO
	}
	return ""
}

// dd/mm ou dd/mm/aaaa sem número, barra ou separador colado antes ou depois
var dateRe = regexp.MustCompile(`(^|[^\d/.,:-])(\d{2})/(\d{2})(?:/(\d{4}))?([^\d/]|$)`)

// isoText troca as datas dia/mês de um texto pelo formato ISO.
func isoText(s string) string {
	for i := 0; i < 2; i++ { // datas coladas ("13/11–14/11") dividem o separador
		s = dateRe.ReplaceAllStringFunc(s, func(m string) string {
			p := dateRe.FindStringSubmatch(m)
			d, _ := strconv.Atoi(p[2])
			mo, _ := strconv.Atoi(p[3])
			if d < 1 || d > 31 || mo < 1 || mo > 12 {
				return m
			}
			if p[4] != "" {
				return p[1] + p[4] + "-" + p[3] + "-" + p[2] + p[5]
			}
			return p[1] + p[3] + "-" + p[2] + p[5]
		})
	}
	return s
}
