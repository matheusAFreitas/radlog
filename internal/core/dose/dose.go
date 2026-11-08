// Package dose tem as contas da "Minha dose" que não dependem do app (sem
// Wails, sem estado): os períodos do calendário e a projeção para o ano.
// É o primeiro pacote do núcleo puro (internal/core), que o app Android
// (3.0) vai reaproveitar.
package dose

import "time"

// MinutesPerYear é o ano médio (365,25 dias) em minutos.
const MinutesPerYear = 365.25 * 24 * 60

// Period é um período do calendário até agora.
type Period struct {
	Key  string // "dia", "semana", "mes", "ano"
	From time.Time
	To   time.Time
}

// Calendar devolve hoje, esta semana (desde segunda-feira), este mês e este
// ano, todos terminando em now, no fuso de now.
func Calendar(now time.Time) []Period {
	y, m, d := now.Date()
	loc := now.Location()
	day := time.Date(y, m, d, 0, 0, 0, 0, loc)
	back := (int(day.Weekday()) + 6) % 7 // segunda = 0
	week := day.AddDate(0, 0, -back)
	month := time.Date(y, m, 1, 0, 0, 0, 0, loc)
	year := time.Date(y, 1, 1, 0, 0, 0, 0, loc)
	return []Period{
		{"dia", day, now},
		{"semana", week, now},
		{"mes", month, now},
		{"ano", year, now},
	}
}

// YearProjection é quanto daria num ano inteiro no ritmo dos minutos
// medidos (µSv). Sem minutos medidos, 0.
func YearProjection(total float64, measuredMinutes int) float64 {
	if measuredMinutes <= 0 {
		return 0
	}
	return total / float64(measuredMinutes) * MinutesPerYear
}

// Above é quanto o total passou do fundo, em fração do fundo (0,03 = 3%).
// Sem fundo, 0.
func Above(total, background float64) float64 {
	if background <= 0 {
		return 0
	}
	return total/background - 1
}
