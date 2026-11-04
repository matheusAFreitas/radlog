package app

import (
	"sort"
	"time"
)

// Limites anuais de dose efetiva (CNEN NN 3.01, art. 52; mesmos valores da
// ICRP). Eles NÃO incluem a radiação natural de fundo nem exposições médicas,
// então o teto do gráfico é: fundo medido + limite.
var doseLimits = map[string]struct {
	Name    string
	MSvYear float64
}{
	"publico":     {"Público: 1 mSv/ano acima do fundo", 1},
	"trabalhador": {"Trabalhador exposto: 20 mSv/ano acima do fundo", 20},
}

const minutesPerYear = 365.25 * 24 * 60

// DoseChart é o gráfico do dosímetro: dose acumulada no período, o que o
// fundo natural sozinho daria e o teto (fundo + limite anual proporcional).
type DoseChart struct {
	From   int64   `json:"from"`
	To     int64   `json:"to"`
	StepMs int64   `json:"stepMs"`
	Times  []int64 `json:"times"` // fim de cada intervalo

	Dose       []float64 `json:"dose"`       // µSv acumulados
	Background []float64 `json:"background"` // µSv que o fundo daria
	Ceiling    []float64 `json:"ceiling"`    // fundo + limite

	Total           float64 `json:"total"`
	BackgroundTotal float64 `json:"backgroundTotal"`
	CeilingTotal    float64 `json:"ceilingTotal"`
	Allowance       float64 `json:"allowance"` // parte do limite no período (µSv)
	Excess          float64 `json:"excess"`    // total - fundo

	BackgroundRate   float64 `json:"backgroundRate"` // µSv/h
	BackgroundSource string  `json:"backgroundSource"`
	LimitName        string  `json:"limitName"`
	LimitMSvYear     float64 `json:"limitMSvYear"`
	MeasuredMinutes  int     `json:"measuredMinutes"`
	Coverage         float64 `json:"coverage"` // 0 a 1
}

// GetDose monta o gráfico do dosímetro. period: "3h", "6h", "12h", "24h",
// "7d", "30d" ou "1y".
// limit: "publico" ou "trabalhador".
//
// Só os minutos com leitura entram na soma, e o fundo e o teto também só
// crescem nesses minutos — assim as três linhas são comparáveis mesmo com
// buracos nos dados.
func (a *App) GetDose(period, limit string) (DoseChart, error) {
	var span, step time.Duration
	switch period {
	case "3h":
		span, step = 3*time.Hour, time.Minute
	case "6h":
		span, step = 6*time.Hour, time.Minute
	case "12h":
		span, step = 12*time.Hour, 2*time.Minute
	case "7d":
		span, step = 7*24*time.Hour, time.Hour
	case "30d":
		span, step = 30*24*time.Hour, 6*time.Hour
	case "1y":
		span, step = 365*24*time.Hour, 24*time.Hour
	default:
		span, step = 24*time.Hour, 5*time.Minute
	}
	to := time.Now().Truncate(time.Minute).Add(time.Minute)
	return a.buildDose(to.Add(-span), to, step, limit)
}

// doseSteps são os passos possíveis no zoom da dose.
var doseSteps = []time.Duration{
	time.Minute, 2 * time.Minute, 5 * time.Minute, 10 * time.Minute, 15 * time.Minute,
	30 * time.Minute, time.Hour, 2 * time.Hour, 6 * time.Hour, 12 * time.Hour, 24 * time.Hour,
}

// GetDoseRange é a dose acumulada só no trecho escolhido arrastando no
// gráfico (zoom): começa do zero no início do trecho, por minuto quando cabe.
func (a *App) GetDoseRange(fromMs, toMs int64, limit string) (DoseChart, error) {
	from, to := time.UnixMilli(fromMs), time.UnixMilli(toMs)
	if to.Before(from) {
		from, to = to, from
	}
	if max := time.Now().Truncate(time.Minute).Add(time.Minute); to.After(max) {
		to = max
	}
	if to.Sub(from) < 5*time.Minute {
		mid := from.Add(to.Sub(from) / 2)
		from, to = mid.Add(-150*time.Second), mid.Add(150*time.Second)
	}
	span := to.Sub(from)
	step := doseSteps[len(doseSteps)-1]
	for _, st := range doseSteps {
		if span/st <= 720 {
			step = st
			break
		}
	}
	from = from.Truncate(step)
	to = ceilTo(to, step)
	return a.buildDose(from, to, step, limit)
}

// buildDose monta o gráfico do dosímetro em [from, to).
func (a *App) buildDose(from, to time.Time, step time.Duration, limit string) (DoseChart, error) {
	span := to.Sub(from)
	lim, ok := doseLimits[limit]
	if !ok {
		lim = doseLimits["publico"]
	}
	now := time.Now()
	pts, err := a.points(from, to) // com o demo ligado, inclui os minutos simulados
	if err != nil {
		return DoseChart{}, err
	}
	a.mu.Lock()
	cal := a.cal
	a.mu.Unlock()

	c := DoseChart{
		From: from.UnixMilli(), To: to.UnixMilli(), StepMs: step.Milliseconds(),
		LimitName: lim.Name, LimitMSvYear: lim.MSvYear,
	}

	// Fundo: mediana do CPM real dos últimos 30 dias (resiste a picos; o
	// demo não mexe nele). Se tiver pouco dado, usa o próprio período.
	bgPts, _ := a.store.Range(now.Add(-30*24*time.Hour), to)
	bgPts = a.homeOnly(bgPts) // o fundo é o de casa
	c.BackgroundSource = "mediana dos últimos 30 dias"
	if len(bgPts) < 60 {
		bgPts, _ = a.store.Range(from, to)
		bgPts = a.homeOnly(bgPts)
		c.BackgroundSource = "mediana do período"
	}
	if len(bgPts) == 0 {
		c.BackgroundSource = ""
	} else {
		cpms := make([]float64, len(bgPts))
		for i, p := range bgPts {
			cpms[i] = p.CPM
		}
		sort.Float64s(cpms)
		med := cpms[len(cpms)/2]
		if len(cpms)%2 == 0 {
			med = (cpms[len(cpms)/2-1] + cpms[len(cpms)/2]) / 2
		}
		c.BackgroundRate = cal.USv(med)
	}

	bgPerMin := c.BackgroundRate / 60
	limPerMin := lim.MSvYear * 1000 / minutesPerYear

	n := int(span / step)
	i := 0
	var dose, bg, allow float64
	for k := 1; k <= n; k++ {
		end := from.Add(time.Duration(k) * step)
		for i < len(pts) && pts[i].T.Before(end) {
			dose += cal.USv(pts[i].CPM) / 60
			bg += bgPerMin
			allow += limPerMin
			c.MeasuredMinutes++
			i++
		}
		c.Times = append(c.Times, end.UnixMilli())
		c.Dose = append(c.Dose, dose)
		c.Background = append(c.Background, bg)
		c.Ceiling = append(c.Ceiling, bg+allow)
	}
	c.Total, c.BackgroundTotal, c.Allowance = dose, bg, allow
	c.CeilingTotal = bg + allow
	c.Excess = dose - bg
	c.Coverage = float64(c.MeasuredMinutes) / span.Minutes()
	if c.Coverage > 1 {
		c.Coverage = 1
	}
	return c, nil
}
