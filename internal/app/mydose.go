package app

import (
	"time"

	coredose "radlog/internal/core/dose"
)

// ---------- Minha dose (2.17) ----------
//
// A tela "Agora" mostra a dose de hoje, desta semana, deste mês e deste ano
// (do calendário, até agora) de cada aparelho: o RadiaCode é a dose pessoal
// (anda com você) e o GMC-800 é a da casa. Cada período usa a mesma conta da
// Dose acumulada (buildDose): só os minutos medidos entram, e o fundo e o
// limite crescem junto com eles.

// MyDosePeriod é a dose de um aparelho num período do calendário.
type MyDosePeriod struct {
	Key             string  `json:"key"` // dia, semana, mes, ano
	From            int64   `json:"from"`
	To              int64   `json:"to"`
	Total           float64 `json:"total"`      // µSv
	Background      float64 `json:"background"` // µSv que o fundo daria nos mesmos minutos
	Allowance       float64 `json:"allowance"`  // parte do limite (µSv)
	Excess          float64 `json:"excess"`     // total - fundo
	Above           float64 `json:"above"`      // total/fundo - 1
	MeasuredMinutes int     `json:"measuredMinutes"`
	Coverage        float64 `json:"coverage"`
	DoseSource      string  `json:"doseSource"` // "aparelho" ou "app"
}

// MyDoseDevice é a Minha dose de um aparelho.
type MyDoseDevice struct {
	Device         string         `json:"device"` // radiacode, gmc
	HasData        bool           `json:"hasData"`
	Periods        []MyDosePeriod `json:"periods"`
	BackgroundRate float64        `json:"backgroundRate"` // µSv/h
	// no ritmo do ano até agora, quanto daria o ano inteiro (µSv), e quanto
	// o fundo sozinho daria (µSv) e o teto (fundo + limite anual)
	YearProjection float64 `json:"yearProjection"`
	YearBackground float64 `json:"yearBackground"`
	YearCeiling    float64 `json:"yearCeiling"`
}

// MyDose é a resposta de GetMyDose.
type MyDose struct {
	Now          int64        `json:"now"`
	LimitName    string       `json:"limitName"`
	LimitMSvYear float64      `json:"limitMSvYear"`
	Personal     MyDoseDevice `json:"personal"` // RadiaCode
	Home         MyDoseDevice `json:"home"`     // GMC-800
	Default      string       `json:"default"`  // "pessoal" se o RadiaCode tem dado no ano, senão "casa"
}

// GetMyDose monta a Minha dose dos dois aparelhos. limit: "publico" ou
// "trabalhador".
func (a *App) GetMyDose(limit string) (MyDose, error) {
	return a.myDose(time.Now(), limit)
}

func (a *App) myDose(now time.Time, limit string) (MyDose, error) {
	now = now.Truncate(time.Minute).Add(time.Minute)
	lim, ok := doseLimits[limit]
	if !ok {
		lim = doseLimits["publico"]
	}
	out := MyDose{Now: now.UnixMilli(), LimitName: lim.Name, LimitMSvYear: lim.MSvYear}
	for _, src := range []string{"radiacode", "gmc"} {
		d := MyDoseDevice{Device: src}
		for _, p := range coredose.Calendar(now) {
			step := p.To.Sub(p.From).Truncate(time.Minute)
			if step < time.Minute {
				step = time.Minute
			}
			c, err := a.buildDose(p.From, p.From.Add(step), step, limit, src)
			if err != nil {
				return out, err
			}
			mp := MyDosePeriod{Key: p.Key, From: c.From, To: c.To, Total: c.Total, Background: c.BackgroundTotal,
				Allowance: c.Allowance, Excess: c.Excess, Above: coredose.Above(c.Total, c.BackgroundTotal),
				MeasuredMinutes: c.MeasuredMinutes, Coverage: c.Coverage, DoseSource: c.DoseSource}
			if c.MeasuredMinutes > 0 || c.Total > 0 {
				d.HasData = true
			}
			d.BackgroundRate = c.BackgroundRate
			if p.Key == "ano" {
				d.YearProjection = coredose.YearProjection(c.Total, c.MeasuredMinutes)
				d.YearBackground = c.BackgroundRate * 24 * 365.25
				d.YearCeiling = d.YearBackground + lim.MSvYear*1000
			}
			d.Periods = append(d.Periods, mp)
		}
		if src == "radiacode" {
			out.Personal = d
		} else {
			out.Home = d
		}
	}
	out.Default = "casa"
	if out.Personal.HasData {
		out.Default = "pessoal"
	}
	return out, nil
}
