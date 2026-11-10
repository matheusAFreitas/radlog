package app

import (
	"fmt"
	"math"
	"sort"
	"time"

	"radlog/internal/store"
)

// ---------- padrões do fundo (2.5) ----------
//
// Mediana (com o intervalo interquartil) por hora do dia, por dia da semana
// e por mês, e a comparação dia × noite. Só os minutos em casa, sem objeto
// encostado e sem minutos saturados ou com interferência. GMC em CPM,
// RadiaCode em µSv/h.

// PatStat resume um grupo de minutos.
type PatStat struct {
	N      int     `json:"n"`    // minutos
	Days   int     `json:"days"` // dias diferentes
	Median float64 `json:"median"`
	P25    float64 `json:"p25"`
	P75    float64 `json:"p75"`
	Mean   float64 `json:"mean"`
}

// PatMonth é um mês.
type PatMonth struct {
	Month int64 `json:"month"` // início do mês (ms)
	PatStat
}

// Patterns é o que a tela mostra.
type Patterns struct {
	Device   string     `json:"device"` // "" = GMC, "radiacode"
	Unit     string     `json:"unit"`
	Days     int        `json:"days"`
	From     int64      `json:"from"`
	To       int64      `json:"to"`
	Minutes  int        `json:"minutes"`
	First    int64      `json:"first"`
	Hours    []PatStat  `json:"hours"`    // 0–23
	Weekdays []PatStat  `json:"weekdays"` // 0 = domingo
	Months   []PatMonth `json:"months"`
	Day      PatStat    `json:"day"`   // 06h–18h
	Night    PatStat    `json:"night"` // 18h–06h
	// dia × noite pareado: em cada dia com as duas metades medidas (6 h ou
	// mais de cada), a diferença das médias; aqui, a média dessas diferenças
	DNDays   int     `json:"dnDays"`
	DNDiff   float64 `json:"dnDiff"` // noite − dia
	DNErr    float64 `json:"dnErr"`
	DNPct    float64 `json:"dnPct"`
	DNText   string  `json:"dnText"`
	PeakHour int     `json:"peakHour"` // pela mediana; -1 sem dados
	LowHour  int     `json:"lowHour"`
}

type patVal struct {
	t time.Time
	v float64
}

func patStat(vs []patVal) PatStat {
	if len(vs) == 0 {
		return PatStat{}
	}
	x := make([]float64, len(vs))
	days := map[string]bool{}
	var sum float64
	for i, p := range vs {
		x[i] = p.v
		sum += p.v
		days[p.t.Format("2006-01-02")] = true
	}
	sort.Float64s(x)
	return PatStat{N: len(x), Days: len(days), Median: percentile(x, .5), P25: percentile(x, .25), P75: percentile(x, .75), Mean: sum / float64(len(x))}
}

func isDaytime(t time.Time) bool { h := t.Hour(); return h >= 6 && h < 18 }

// GetPatterns monta os padrões dos últimos days dias (30, 90 ou 365).
func (a *App) GetPatterns(days int, dev string) (Patterns, error) {
	switch days {
	case 30, 90, 365:
	default:
		days = 90
	}
	if dev != devRC {
		dev = ""
	}
	now := time.Now()
	to := now.Truncate(time.Minute).Add(time.Minute)
	from := to.AddDate(0, 0, -days)
	out := Patterns{Device: dev, Unit: "CPM", Days: days, From: from.UnixMilli(), To: to.UnixMilli(), PeakHour: -1, LowHour: -1}
	var vals []patVal
	if dev == devRC {
		out.Unit = "µSv/h"
		if f := a.rcFirst(); !f.IsZero() {
			out.First = f.UnixMilli()
		}
		away, measuring := a.rcAwayChecker(), a.measuringChecker()
		for _, r := range a.rcRange(from, to) {
			if !away(r.T) && !measuring(r.T) {
				vals = append(vals, patVal{r.T, r.USvh})
			}
		}
	} else {
		if f := a.gmcFirst(); !f.IsZero() {
			out.First = f.UnixMilli()
		}
		if a.store != nil {
			pts, err := a.store.Range(from, to)
			if err != nil {
				return out, err
			}
			for _, p := range bgClean(a.homeOnly(pts)) {
				vals = append(vals, patVal{p.T, p.CPM})
			}
		}
	}
	fillPatterns(&out, vals)
	return out, nil
}

func fillPatterns(out *Patterns, vals []patVal) {
	out.Minutes = len(vals)
	byHour := make([][]patVal, 24)
	byWd := make([][]patVal, 7)
	byMonth := map[int64][]patVal{}
	var day, night []patVal
	type half struct {
		ds, ns float64
		dn, nn int
	}
	perDay := map[int64]*half{}
	for _, p := range vals {
		byHour[p.t.Hour()] = append(byHour[p.t.Hour()], p)
		byWd[int(p.t.Weekday())] = append(byWd[int(p.t.Weekday())], p)
		m := time.Date(p.t.Year(), p.t.Month(), 1, 0, 0, 0, 0, p.t.Location()).UnixMilli()
		byMonth[m] = append(byMonth[m], p)
		// a noite de um dia vai das 18h às 6h do dia seguinte: as horas antes
		// das 6h contam para o dia anterior
		ref := p.t
		if p.t.Hour() < 6 {
			ref = ref.AddDate(0, 0, -1)
		}
		k := time.Date(ref.Year(), ref.Month(), ref.Day(), 0, 0, 0, 0, ref.Location()).UnixMilli()
		h := perDay[k]
		if h == nil {
			h = &half{}
			perDay[k] = h
		}
		if isDaytime(p.t) {
			day = append(day, p)
			h.ds += p.v
			h.dn++
		} else {
			night = append(night, p)
			h.ns += p.v
			h.nn++
		}
	}
	out.Hours = make([]PatStat, 24)
	for h := range byHour {
		out.Hours[h] = patStat(byHour[h])
		if out.Hours[h].N < 30 {
			continue
		}
		if out.PeakHour < 0 || out.Hours[h].Median > out.Hours[out.PeakHour].Median {
			out.PeakHour = h
		}
		if out.LowHour < 0 || out.Hours[h].Median < out.Hours[out.LowHour].Median {
			out.LowHour = h
		}
	}
	out.Weekdays = make([]PatStat, 7)
	for d := range byWd {
		out.Weekdays[d] = patStat(byWd[d])
	}
	var mk []int64
	for k := range byMonth {
		mk = append(mk, k)
	}
	sort.Slice(mk, func(i, j int) bool { return mk[i] < mk[j] })
	out.Months = []PatMonth{}
	for _, k := range mk {
		out.Months = append(out.Months, PatMonth{Month: k, PatStat: patStat(byMonth[k])})
	}
	out.Day, out.Night = patStat(day), patStat(night)

	// diferença pareada por dia (6 h ou mais de cada metade)
	var diffs []float64
	var lvl float64
	for _, h := range perDay {
		if h.dn >= 360 && h.nn >= 360 {
			dm, nm := h.ds/float64(h.dn), h.ns/float64(h.nn)
			diffs = append(diffs, nm-dm)
			lvl += dm
		}
	}
	out.DNDays = len(diffs)
	if len(diffs) < 3 {
		out.DNText = fmt.Sprintf("Poucos dias com dia e noite medidos (%d; precisa de 3, com 6 h ou mais de cada metade).", len(diffs))
		return
	}
	var mean, ss float64
	for _, d := range diffs {
		mean += d
	}
	mean /= float64(len(diffs))
	for _, d := range diffs {
		ss += (d - mean) * (d - mean)
	}
	lvl /= float64(len(diffs))
	out.DNDiff = mean
	out.DNErr = math.Sqrt(ss/float64(len(diffs)-1)) / math.Sqrt(float64(len(diffs)))
	if lvl > 0 {
		out.DNPct = mean / lvl * 100
	}
	dec := 1
	if out.Unit != "CPM" {
		dec = 4
	}
	num := func(v float64) string { return fmtBR(v, dec) }
	sig := math.Abs(mean) >= 3*out.DNErr && math.Abs(out.DNPct) >= 2
	switch {
	case sig && mean > 0:
		out.DNText = fmt.Sprintf("A noite (18h–6h) fica %s%% acima do dia (+%s ± %s %s, %d dias com as duas metades). É o padrão comum do radônio, que se acumula com a casa fechada e o ar parado à noite.",
			fmtBR(out.DNPct, 0), num(mean), num(out.DNErr), out.Unit, len(diffs))
	case sig:
		out.DNText = fmt.Sprintf("O dia (6h–18h) fica %s%% acima da noite (%s ± %s %s, %d dias com as duas metades).",
			fmtBR(-out.DNPct, 0), num(-mean), num(out.DNErr), out.Unit, len(diffs))
	default:
		out.DNText = fmt.Sprintf("Sem diferença clara entre dia e noite (noite − dia = %s ± %s %s, %d dias com as duas metades).",
			fmtSigned(mean, dec), num(out.DNErr), out.Unit, len(diffs))
	}
}

// pointsVals é usado nos testes.
func pointsVals(pts []store.Point) []patVal {
	out := make([]patVal, len(pts))
	for i, p := range pts {
		out[i] = patVal{p.T, p.CPM}
	}
	return out
}
