package app

import (
	"math"
	"sort"
	"time"

	"radlog/internal/store"
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
	Coverage         float64 `json:"coverage"` // 0 a 1, desde o primeiro dado (até agora)
	HistMinutes      int     `json:"histMinutes"`
	SpanMinutes      int     `json:"spanMinutes"`
	HistFrom         int64   `json:"histFrom"` // ms; ≠ 0 quando o período começa antes do primeiro dado

	// de qual aparelho é a dose ("radiacode": sua dose pessoal; "gmc": a da
	// casa) e a do outro no mesmo período, para comparar
	Device       string  `json:"device"`
	Other        string  `json:"other"`
	OtherTotal   float64 `json:"otherTotal"`
	OtherMinutes int     `json:"otherMinutes"`
	HasRC        bool    `json:"hasRC"` // o RadiaCode tem dado no período

	// RadiaCode: "aparelho" = a curva é a dose que o próprio aparelho soma
	// (continua contando desconectado); "app" = a soma dos minutos gravados.
	// App* é a conta do app no mesmo período, para comparar.
	DoseSource  string  `json:"doseSource"`
	AppTotal    float64 `json:"appTotal"`
	AppMinutes  int     `json:"appMinutes"`
	AppCoverage float64 `json:"appCoverage"`
	// GMC: o contador não informa a dose que soma; a do app sai do CPM de
	// cada minuto pela calibração que valia nele (CalNote diz qual).
	CalNote string `json:"calNote,omitempty"`
}

// GetDose monta o gráfico do dosímetro. period: "15m", "30m", "1h", "3h",
// "6h", "12h", "24h", "7d", "30d" ou "1y".
// limit: "publico" ou "trabalhador".
//
// Só os minutos com leitura entram na soma, e o fundo e o teto também só
// crescem nesses minutos — assim as três linhas são comparáveis mesmo com
// buracos nos dados.
func (a *App) GetDose(period, limit string) (DoseChart, error) {
	return a.GetDoseSrc(period, limit, "gmc")
}

// GetDoseSrc é o GetDose escolhendo o aparelho: "radiacode" ou "gmc".
func (a *App) GetDoseSrc(period, limit, src string) (DoseChart, error) {
	var span, step time.Duration
	switch period {
	case "15m": // 2.12: o mesmo período do gráfico da taxa
		span, step = 15*time.Minute, time.Minute
	case "30m":
		span, step = 30*time.Minute, time.Minute
	case "1h":
		span, step = time.Hour, time.Minute
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
	return a.buildDose(to.Add(-span), to, step, limit, src)
}

// doseSteps são os passos possíveis no zoom da dose.
var doseSteps = []time.Duration{
	time.Minute, 2 * time.Minute, 5 * time.Minute, 10 * time.Minute, 15 * time.Minute,
	30 * time.Minute, time.Hour, 2 * time.Hour, 6 * time.Hour, 12 * time.Hour, 24 * time.Hour,
}

// GetDoseRange é a dose acumulada só no trecho escolhido arrastando no
// gráfico (zoom): começa do zero no início do trecho, por minuto quando cabe.
func (a *App) GetDoseRange(fromMs, toMs int64, limit string) (DoseChart, error) {
	return a.GetDoseRangeSrc(fromMs, toMs, limit, "gmc")
}

// GetDoseRangeSrc é o GetDoseRange escolhendo o aparelho.
func (a *App) GetDoseRangeSrc(fromMs, toMs int64, limit, src string) (DoseChart, error) {
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
	return a.buildDose(from, to, step, limit, src)
}

// doseMin é um minuto medido, em µSv/h.
type doseMin struct {
	T   time.Time
	USv float64
}

// doseSeries são os minutos medidos de um aparelho em [from, to), e o fundo
// de casa dele (µSv/h, mediana) com a origem.
func (a *App) doseSeries(src string, from, to time.Time) ([]doseMin, float64, string, error) {
	now := time.Now()
	median := func(v []float64) float64 {
		sort.Float64s(v)
		if len(v)%2 == 0 {
			return (v[len(v)/2-1] + v[len(v)/2]) / 2
		}
		return v[len(v)/2]
	}
	if src == "radiacode" {
		var out []doseMin
		for _, p := range a.rcRange(from, to) {
			out = append(out, doseMin{p.T, p.USvh})
		}
		// fundo: minutos em casa dos últimos 30 dias (ou do período)
		home := func(f, t time.Time) []float64 {
			var sp []store.Point
			for _, p := range a.rcRange(f, t) {
				sp = append(sp, store.Point{T: p.T, CPM: p.USvh})
			}
			var v []float64
			for _, p := range a.homeOnly(sp) {
				v = append(v, p.CPM)
			}
			return v
		}
		v, srcTxt := home(now.Add(-30*24*time.Hour), to), "mediana em casa dos últimos 30 dias"
		if len(v) < 60 {
			v, srcTxt = home(from, to), "mediana em casa no período"
		}
		if len(v) == 0 {
			return out, 0, "", nil
		}
		return out, median(v), srcTxt, nil
	}
	pts, err := a.points(from, to) // com o demo ligado, inclui os minutos simulados
	if err != nil {
		return nil, 0, "", err
	}
	a.mu.Lock()
	cal := a.convCalLocked()
	tl := a.calTLLocked()
	a.mu.Unlock()
	out := make([]doseMin, len(pts))
	for i, p := range pts {
		out[i] = doseMin{p.T, tl.USv(p)} // pela calibração que valia nesse minuto
	}
	// Fundo: mediana do CPM real dos últimos 30 dias (resiste a picos; o
	// demo não mexe nele). Se tiver pouco dado, usa o próprio período.
	bgPts, _ := a.store.Range(now.Add(-30*24*time.Hour), to)
	bgPts = a.homeOnly(bgPts) // o fundo é o de casa
	srcTxt := "mediana dos últimos 30 dias"
	if len(bgPts) < 60 {
		bgPts, _ = a.store.Range(from, to)
		bgPts = a.homeOnly(bgPts)
		srcTxt = "mediana do período"
	}
	if len(bgPts) == 0 {
		return out, 0, "", nil
	}
	cpms := make([]float64, len(bgPts))
	for i, p := range bgPts {
		cpms[i] = p.CPM
	}
	return out, cal.USv(median(cpms)), srcTxt, nil
}

// buildDose monta o gráfico do dosímetro em [from, to).
func (a *App) buildDose(from, to time.Time, step time.Duration, limit, src string) (DoseChart, error) {
	span := to.Sub(from)
	lim, ok := doseLimits[limit]
	if !ok {
		lim = doseLimits["publico"]
	}
	if src != "radiacode" {
		src = "gmc"
	}
	other := map[string]string{"gmc": "radiacode", "radiacode": "gmc"}[src]
	pts, bgRate, bgSrc, err := a.doseSeries(src, from, to)
	if err != nil {
		return DoseChart{}, err
	}
	c := DoseChart{
		From: from.UnixMilli(), To: to.UnixMilli(), StepMs: step.Milliseconds(),
		LimitName: lim.Name, LimitMSvYear: lim.MSvYear,
		BackgroundRate: bgRate, BackgroundSource: bgSrc, Device: src, Other: other,
	}
	if ops, _, _, err := a.doseSeries(other, from, to); err == nil {
		for _, p := range ops {
			c.OtherTotal += p.USv / 60
		}
		c.OtherMinutes = len(ops)
	}
	c.HasRC = (src == "radiacode" && len(pts) > 0) || (other == "radiacode" && c.OtherMinutes > 0)

	bgPerMin := c.BackgroundRate / 60
	limPerMin := lim.MSvYear * 1000 / minutesPerYear
	c.DoseSource = "app"
	if src == "radiacode" {
		if a.rcDoseFromDevice(&c, from, to, step, pts, bgPerMin, limPerMin) {
			return c, nil
		}
	} else {
		a.mu.Lock()
		tl := a.calTLLocked()
		a.mu.Unlock()
		c.CalNote = tl.periodDesc(from.UnixMilli(), to.UnixMilli())
	}

	n := int(span / step)
	i := 0
	var dose, bg, allow float64
	for k := 1; k <= n; k++ {
		end := from.Add(time.Duration(k) * step)
		for i < len(pts) && pts[i].T.Before(end) {
			dose += pts[i].USv / 60
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
	// cobertura desde o primeiro dado do aparelho (2.3): antes dele não é falta
	first := a.gmcFirst()
	if src == "radiacode" {
		first = a.rcFirst()
	}
	if len(pts) > 0 && (first.IsZero() || pts[0].T.Before(first)) {
		first = pts[0].T
	}
	cs := coverageSpan(from, to, first, time.Now())
	c.Coverage, c.HistMinutes, c.SpanMinutes = ratio(c.MeasuredMinutes, cs.Hist), cs.Hist, cs.Span
	if cs.Pre > 0 && cs.Hist > 0 {
		c.HistFrom = cs.HistFrom.UnixMilli()
	}
	return c, nil
}

// rcDoseFromDevice monta a curva com a dose do próprio RadiaCode (os totais
// que ele manda). O fundo e o teto crescem com o tempo em que o aparelho
// contou (ele conta mesmo longe do PC). Devolve false se não há totais
// suficientes no período (aí vale a soma dos minutos do app).
func (a *App) rcDoseFromDevice(c *DoseChart, from, to time.Time, step time.Duration, mins []doseMin, bgPerMin, limPerMin float64) bool {
	raw := a.rcDoseLog(from.AddDate(0, 0, -7), to)
	// o total de agora (ao vivo), se conectado
	a.rc.mu.Lock()
	if d := a.rc.dose; d.has && a.rc.st.State == "conectado" {
		now := time.Now()
		if now.Before(to) && (len(raw) == 0 || now.After(raw[len(raw)-1].T)) {
			raw = append(raw, rcDosePt{now, d.base + d.acc, d.baseSec + now.Sub(d.baseAt).Seconds()})
		}
	}
	a.rc.mu.Unlock()
	if len(raw) < 2 || !raw[len(raw)-1].T.After(from) {
		return false
	}
	curve := rcDoseCurve(raw)
	// tempo contando, também contínuo (zerou: recomeça do que veio depois)
	secs := rcSecsCurve(raw)
	first, last := raw[0].T, raw[len(raw)-1].T
	start := from
	if first.After(start) {
		start = first
	}
	d0, s0 := curveAt(curve, start), curveAt(secs, start)
	// depois do último total do aparelho (ele manda a cada ~4 min; se parou
	// de mandar ou a conexão caiu), segue com os minutos que o app gravou,
	// em vez de ficar parado: a dose e o tempo contando continuam subindo
	var tT []time.Time
	var tD []float64 // dose somada até cada minuto depois do último total
	for _, p := range mins {
		if p.T.Before(last) {
			continue
		}
		prev := 0.0
		if len(tD) > 0 {
			prev = tD[len(tD)-1]
		}
		tT, tD = append(tT, p.T), append(tD, prev+p.USv/60)
	}
	tail := func(t time.Time) (dose, minutes float64) {
		k := sort.Search(len(tT), func(i int) bool { return !tT[i].Before(t) })
		if k == 0 {
			return 0, 0
		}
		return tD[k-1], float64(k)
	}
	at := func(t time.Time) (dose, minutes float64) {
		if !t.After(start) {
			return 0, 0
		}
		var td, tm float64
		if t.After(last) {
			td, tm = tail(t)
			t = last
		}
		return curveAt(curve, t) - d0 + td, (curveAt(secs, t)-s0)/60 + tm
	}
	n := int(to.Sub(from) / step)
	c.Times, c.Dose, c.Background, c.Ceiling = nil, nil, nil, nil
	for k := 1; k <= n; k++ {
		end := from.Add(time.Duration(k) * step)
		dose, m := at(end)
		c.Times = append(c.Times, end.UnixMilli())
		c.Dose = append(c.Dose, dose)
		c.Background = append(c.Background, m*bgPerMin)
		c.Ceiling = append(c.Ceiling, m*(bgPerMin+limPerMin))
	}
	dose, m := at(to)
	c.Total, c.BackgroundTotal, c.Allowance = dose, m*bgPerMin, m*limPerMin
	c.CeilingTotal = c.BackgroundTotal + c.Allowance
	c.Excess = dose - c.BackgroundTotal
	c.MeasuredMinutes = int(m + 0.5)
	c.Coverage = math.Min(1, m/to.Sub(from).Minutes())
	c.DoseSource = "aparelho"
	for _, p := range mins {
		c.AppTotal += p.USv / 60
	}
	c.AppMinutes = len(mins)
	c.AppCoverage = math.Min(1, float64(len(mins))/to.Sub(from).Minutes())
	c.HasRC = true
	return true
}
