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
// limite crescem junto com eles. Com um aparelho fora do local normal em
// parte do período (roles.go), a pessoal e a da casa juntam os minutos de
// cada aparelho no papel certo (roleDose).

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
	// aparelho fora do local normal em parte do período: a dose junta os dois
	// (roles.go) e SwapMinutes são os minutos do que normalmente não tem o papel
	Device      string `json:"device"` // quem mediu: radiacode, gmc, misto ("" = ninguém)
	Mixed       bool   `json:"mixed,omitempty"`
	SwapMinutes int    `json:"swapMinutes,omitempty"`
	GapMinutes  int    `json:"gapMinutes,omitempty"` // minutos sem ninguém nesse papel
	// minutos em que o aparelho do papel não mandou leitura (desconectado,
	// desligado ou fora do cálculo)
	NoDataMinutes int `json:"noDataMinutes,omitempty"`
	SatMinutes    int `json:"satMinutes,omitempty"` // GMC saturado: a dose é "pelo menos" (≥)
}

// MyDoseDevice é a Minha dose de um aparelho.
type MyDoseDevice struct {
	Device         string         `json:"device"` // radiacode, gmc: o aparelho com esse papel agora ("" = nenhum)
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
	Personal     MyDoseDevice `json:"personal"` // com você (normalmente o RadiaCode; roles.go)
	Home         MyDoseDevice `json:"home"`     // em casa (normalmente o GMC-800; roles.go)
	Normal       bool         `json:"normal"`   // GMC em casa e RadiaCode com você agora
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
	a.mu.Lock()
	roles := a.whereaboutsLocked(now.Add(-time.Minute))
	a.mu.Unlock()
	out.Normal = roles.Normal
	for _, role := range []string{"pessoal", "casa"} {
		d := MyDoseDevice{Device: roles.Home}
		if role == "pessoal" {
			d.Device = roles.Personal
		}
		for _, p := range coredose.Calendar(now) {
			step := p.To.Sub(p.From).Truncate(time.Minute)
			if step < time.Minute {
				step = time.Minute
			}
			c, err := a.roleDose(p.From, p.From.Add(step), step, limit, role)
			if err != nil {
				return out, err
			}
			mp := MyDosePeriod{Key: p.Key, From: c.From, To: c.To, Total: c.Total, Background: c.BackgroundTotal,
				Allowance: c.Allowance, Excess: c.Excess, Above: coredose.Above(c.Total, c.BackgroundTotal),
				MeasuredMinutes: c.MeasuredMinutes, Coverage: c.Coverage, DoseSource: c.DoseSource,
				Device: c.Device, Mixed: c.Mixed, SwapMinutes: c.SwapMinutes, GapMinutes: c.GapMinutes, NoDataMinutes: c.NoDataMinutes, SatMinutes: c.SatMinutes}
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
		if role == "pessoal" {
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
