package app

import (
	"fmt"
	"math"
	"sort"
	"time"

	"radlog/internal/store"
)

// ---------- fundo da casa ao longo do tempo (2.4) ----------
//
// Média e mediana móveis de 24 h dos minutos em casa (sem fora de casa,
// sem objeto encostado e sem minutos saturados ou com interferência), a
// estatística de cada dia (o "sigma ao longo do tempo") e um aviso de
// tendência. O aviso só descreve o que os números mostram: a causa
// (estação, chuva, ventilação, contador em outro lugar) fica para você.

// BgTrendPt é a janela móvel de 24 h que termina em T.
type BgTrendPt struct {
	T      int64    `json:"t"`
	N      int      `json:"n"`
	Mean   *float64 `json:"mean"`
	Median *float64 `json:"median"`
}

// BgDay é a estatística de um dia (só os minutos em casa).
type BgDay struct {
	Day        int64   `json:"day"`
	N          int     `json:"n"`
	Mean       float64 `json:"mean"`
	Trimmed    float64 `json:"trimmed"` // média sem os 5% mais altos e os 5% mais baixos
	Median     float64 `json:"median"`
	SD         float64 `json:"sd"`         // desvio-padrão real dos minutos
	PoissonSD  float64 `json:"poissonSD"`  // √média: o que o acaso daria
	Dispersion float64 `json:"dispersion"` // SD ÷ √média
	Full       bool    `json:"full"`       // ao menos metade do dia medida (entra na tendência)
}

// BgFit é o aviso de tendência.
type BgFit struct {
	State     string  `json:"state"` // insuficiente, estavel, subindo, descendo, degrau
	DaysUsed  int     `json:"daysUsed"`
	Slope     float64 `json:"slope"` // CPM por dia
	SlopeErr  float64 `json:"slopeErr"`
	ChangePct float64 `json:"changePct"` // ao longo dos dias usados
	Level     float64 `json:"level"`     // média dos dias usados (CPM)
	StepAt    int64   `json:"stepAt"`    // degrau: primeiro dia do novo nível
	StepFrom  float64 `json:"stepFrom"`
	StepTo    float64 `json:"stepTo"`
	Text      string  `json:"text"`
	FitFrom   float64 `json:"fitFrom"` // reta: valor no primeiro e no último dia usado
	FitTo     float64 `json:"fitTo"`
	FitT0     int64   `json:"fitT0"`
	FitT1     int64   `json:"fitT1"`
}

// BgTrend é o que a tela mostra.
type BgTrend struct {
	Days    int         `json:"days"`
	From    int64       `json:"from"`
	To      int64       `json:"to"`
	StepMs  int64       `json:"stepMs"`
	Minutes int         `json:"minutes"` // minutos em casa usados
	First   int64       `json:"first"`   // primeiro dado do GMC (ms)
	Pts     []BgTrendPt `json:"pts"`
	Daily   []BgDay     `json:"daily"`
	Fit     BgFit       `json:"fit"`
}

const (
	bgMinPerWindow = 360 // janela móvel de 24 h: pelo menos 6 h medidas
	bgFullDay      = 720 // dia que entra na tendência: pelo menos 12 h
	bgMinDays      = 7   // dias completos para falar de tendência
)

func bgStep(days int) time.Duration {
	switch {
	case days <= 7:
		return time.Hour
	case days <= 30:
		return 3 * time.Hour
	case days <= 90:
		return 12 * time.Hour
	}
	return 24 * time.Hour
}

// GetBgTrend monta o fundo dos últimos days dias (7, 30, 90 ou 365).
func (a *App) GetBgTrend(days int) (BgTrend, error) {
	switch days {
	case 7, 30, 90, 365:
	default:
		days = 30
	}
	now := time.Now()
	to := now.Truncate(time.Minute).Add(time.Minute)
	from := to.AddDate(0, 0, -days)
	out := BgTrend{Days: days, From: from.UnixMilli(), To: to.UnixMilli(), Pts: []BgTrendPt{}, Daily: []BgDay{}}
	if f := a.gmcFirst(); !f.IsZero() {
		out.First = f.UnixMilli()
	}
	if a.store == nil {
		return out, nil
	}
	// um dia a mais antes, para a primeira janela móvel
	raw, err := a.store.Range(from.Add(-24*time.Hour), to)
	if err != nil {
		return out, err
	}
	pts := bgClean(a.homeOnly(raw))
	out.Pts, out.StepMs = bgMoving(pts, from, to, bgStep(days))
	out.Daily = bgDaily(pointsIn(pts, from, to))
	for _, d := range out.Daily {
		out.Minutes += d.N
	}
	// os textos do ajuste em µSv/h, pela calibração de agora (2.65.2)
	a.mu.Lock()
	k := a.convCalLocked().USv(100) / 100
	a.mu.Unlock()
	out.Fit = bgFitDays(out.Daily, k)
	return out, nil
}

// bgClean tira os minutos que não são fundo: saturados ou com interferência.
func bgClean(pts []store.Point) []store.Point {
	out := pts[:0:0]
	for _, p := range pts {
		if p.HasObs(store.ObsSaturation) || p.HasObs(store.ObsInterference) {
			continue
		}
		out = append(out, p)
	}
	return out
}

// bgMoving calcula a média e a mediana das 24 h que terminam em cada passo.
func bgMoving(pts []store.Point, from, to time.Time, step time.Duration) ([]BgTrendPt, int64) {
	out := []BgTrendPt{}
	i, j := 0, 0
	buf := []float64{}
	for t := from.Add(step); !t.After(to); t = t.Add(step) {
		w0 := t.Add(-24 * time.Hour)
		for i < len(pts) && pts[i].T.Before(w0) {
			i++
		}
		for j < len(pts) && pts[j].T.Before(t) {
			j++
		}
		pt := BgTrendPt{T: t.UnixMilli(), N: j - i}
		if pt.N >= bgMinPerWindow {
			buf = buf[:0]
			sum := 0.0
			for _, p := range pts[i:j] {
				buf = append(buf, p.CPM)
				sum += p.CPM
			}
			sort.Float64s(buf)
			mean, med := sum/float64(len(buf)), percentile(buf, .5)
			pt.Mean, pt.Median = &mean, &med
		}
		out = append(out, pt)
	}
	return out, step.Milliseconds()
}

// bgDaily agrupa por dia do calendário.
func bgDaily(pts []store.Point) []BgDay {
	byDay := map[int64][]float64{}
	var keys []int64
	for _, p := range pts {
		y, m, d := p.T.Date()
		k := time.Date(y, m, d, 0, 0, 0, 0, p.T.Location()).UnixMilli()
		if _, ok := byDay[k]; !ok {
			keys = append(keys, k)
		}
		byDay[k] = append(byDay[k], p.CPM)
	}
	sort.Slice(keys, func(i, j int) bool { return keys[i] < keys[j] })
	out := make([]BgDay, 0, len(keys))
	for _, k := range keys {
		v := byDay[k]
		sort.Float64s(v)
		d := BgDay{Day: k, N: len(v), Median: percentile(v, .5), Full: len(v) >= bgFullDay}
		var sum float64
		for _, x := range v {
			sum += x
		}
		d.Mean = sum / float64(len(v))
		cut := len(v) / 20
		var ts float64
		for _, x := range v[cut : len(v)-cut] {
			ts += x
		}
		d.Trimmed = ts / float64(len(v)-2*cut)
		if len(v) > 1 {
			var ss float64
			for _, x := range v {
				ss += (x - d.Mean) * (x - d.Mean)
			}
			d.SD = math.Sqrt(ss / float64(len(v)-1))
		}
		if d.Mean > 0 {
			d.PoissonSD = math.Sqrt(d.Mean)
			d.Dispersion = d.SD / d.PoissonSD
		}
		out = append(out, d)
	}
	return out
}

// bgFitDays procura tendência na média aparada dos dias completos: reta
// (rampa) e dois níveis (degrau). Só avisa quando a mudança é grande perto
// da variação entre os dias e passa de 5% do nível. k converte CPM em µSv/h
// nos textos (0: os textos ficam em CPM).
func bgFitDays(days []BgDay, k float64) BgFit {
	unit, lv, sl := "CPM", func(v float64) string { return fmtBR(v, 1) }, func(v float64) string { return fmtSigned(v, 2) }
	slErr := func(v float64) string { return fmtBR(v, 2) }
	if k > 0 {
		unit = "µSv/h"
		lv = func(v float64) string { return fmtBR(v*k, 3) }
		sl = func(v float64) string { return fmtSigned(v*k, 4) }
		slErr = func(v float64) string { return fmtBR(v*k, 4) }
	}
	var xs, ys []float64
	var ts []int64
	for _, d := range days {
		if !d.Full {
			continue
		}
		xs = append(xs, float64(d.Day-days[0].Day)/864e5)
		ys = append(ys, d.Trimmed)
		ts = append(ts, d.Day)
	}
	n := len(ys)
	f := BgFit{DaysUsed: n}
	if n < bgMinDays {
		f.State = "insuficiente"
		f.Text = fmt.Sprintf("Poucos dias completos para falar de tendência: %d de %d (um dia conta com pelo menos 12 h medidas em casa).", n, bgMinDays)
		return f
	}
	var mx, my float64
	for i := range xs {
		mx += xs[i]
		my += ys[i]
	}
	mx /= float64(n)
	my /= float64(n)
	var sxx, sxy float64
	for i := range xs {
		sxx += (xs[i] - mx) * (xs[i] - mx)
		sxy += (xs[i] - mx) * (ys[i] - my)
	}
	slope := sxy / sxx
	icpt := my - slope*mx
	var ssLin float64
	for i := range xs {
		r := ys[i] - (icpt + slope*xs[i])
		ssLin += r * r
	}
	// espalhamento entre os dias, com piso no acaso de um dia (√média ÷ √minutos)
	s := math.Sqrt(ssLin / float64(n-2))
	if floor := math.Sqrt(my / bgFullDay); s < floor {
		s = floor
	}
	f.Slope, f.SlopeErr, f.Level = slope, s/math.Sqrt(sxx), my
	span := xs[n-1] - xs[0]
	if my > 0 {
		f.ChangePct = slope * span / my * 100
	}
	f.FitT0, f.FitT1 = ts[0], ts[n-1]
	f.FitFrom, f.FitTo = icpt+slope*xs[0], icpt+slope*xs[n-1]

	// degrau: o melhor corte em dois níveis (pelo menos 3 dias de cada lado)
	bestK, bestSS := -1, math.Inf(1)
	var m1b, m2b float64
	for k := 3; k <= n-3; k++ {
		var m1, m2 float64
		for _, y := range ys[:k] {
			m1 += y
		}
		for _, y := range ys[k:] {
			m2 += y
		}
		m1 /= float64(k)
		m2 /= float64(n - k)
		var ss float64
		for _, y := range ys[:k] {
			ss += (y - m1) * (y - m1)
		}
		for _, y := range ys[k:] {
			ss += (y - m2) * (y - m2)
		}
		if ss < bestSS {
			bestK, bestSS, m1b, m2b = k, ss, m1, m2
		}
	}
	stepT := 0.0
	if bestK > 0 {
		sp := math.Sqrt(bestSS / float64(n-2))
		if floor := math.Sqrt(my / bgFullDay); sp < floor {
			sp = floor
		}
		stepT = math.Abs(m2b-m1b) / (sp * math.Sqrt(1/float64(bestK)+1/float64(n-bestK)))
	}
	slopeT := math.Abs(slope) / f.SlopeErr
	caveat := " É um aviso, não uma conclusão: estação do ano, chuva, ventilação e o contador em outro lugar também mudam o fundo."
	switch {
	case bestK > 0 && stepT >= 4 && my > 0 && math.Abs(m2b-m1b)/my >= 0.05 && bestSS < 0.6*ssLin:
		f.State = "degrau"
		f.StepAt, f.StepFrom, f.StepTo = ts[bestK], m1b, m2b
		f.Text = fmt.Sprintf("Mudança brusca por volta de %s: o fundo passou de %s para %s %s (%s%%), com %d dias antes e %d depois.",
			time.UnixMilli(f.StepAt).Format("02/01"), lv(m1b), lv(m2b), unit, fmtSigned((m2b/m1b-1)*100, 0), bestK, n-bestK) + caveat
	case slopeT >= 3 && math.Abs(f.ChangePct) >= 5:
		f.State = "subindo"
		if slope < 0 {
			f.State = "descendo"
		}
		f.Text = fmt.Sprintf("O fundo %s cerca de %s%% em %d dias (%s ± %s %s por dia, %d dias completos).",
			map[bool]string{true: "subiu", false: "desceu"}[slope > 0], fmtBR(math.Abs(f.ChangePct), 0), int(math.Round(span))+1,
			sl(slope), slErr(f.SlopeErr), unit, n) + caveat
	default:
		f.State = "estavel"
		f.Text = fmt.Sprintf("Sem tendência: a média dos dias variou dentro do esperado (%s ± %s %s por dia, %s%% no período, %d dias completos).",
			sl(slope), slErr(f.SlopeErr), unit, fmtSigned(f.ChangePct, 1), n)
	}
	return f
}
