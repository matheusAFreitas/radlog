package app

import (
	"math"
	"sort"
	"strings"
	"time"
)

// ---------- relatório PDF novo (2.68.0, spec ux/pdf.html) ----------
//
// Página 1: o resumo (frase, 4 números, gráfico dos dois aparelhos, fita dos
// dias). Página 2: por dia, alertas, saídas de casa, medições, chuva e
// anotações. Página 3: o espectro do RC102 do período e de cada saída.
// Depois, o anexo técnico (A1–A8) com tudo o que o relatório tinha antes.
// "Só o resumo" sai só com a página 1. Nunca GPS nem nome de lugar.

// reportOpts são as escolhas do diálogo "Relatório (PDF)".
type reportOpts struct {
	Summary bool // só a página 1
	Notes   bool // incluir as anotações
	Cfg     bool // incluir o anexo das configurações vigentes
}

// reportDay2 é uma linha do "Por dia" da página 2.
type reportDay2 struct {
	Label   string
	HomeU   float64 // média do GMC-800 em µSv/h (0 = sem dado)
	HasHome bool
	Dose    float64 // sua dose (RC102), µSv
	HasDose bool
	Peak    float64 // µSv/h
	PeakDev string  // "GMC-800" | "RC102"
	Alerts  int
}

// reportAlert2 é um alerta da lista simples da página 2.
type reportAlert2 struct {
	Start     time.Time
	Dev       string // "GMC-800" | "RC102"
	Level     string // atencao | alarme | perigo
	Peak      float64
	DurSec    int
	Confirmed string // "confirmado" | "não confirmado" | ""
}

// reportTrip é uma saída de casa (o RC102 foi junto) no período.
type reportTrip struct {
	Title    string
	From, To time.Time
	Dose     float64
	HasDose  bool
	Peak     float64
	PeakAt   time.Time
	Spec     *MeasureSpectrum
	SpecRows int
}

// reportDoseVerdict compara a sua dose com a de ficar em casa (ACE, 12/11).
type reportDoseVerdict struct {
	OK       bool
	Dose     float64 // µSv
	Expected float64 // µSv, ficando em casa
	Ratio    float64
	Level    string // "normal" | "acima" | "alto"
	YearMSv  float64
}

// doseVerdict: esperado = fundo de casa do RC102 × horas; até 1,25× normal,
// até 2× acima do normal, mais que isso alto.
func doseVerdict(rc *RcSum, span time.Duration) reportDoseVerdict {
	var v reportDoseVerdict
	if rc == nil || rc.Minutes == 0 || rc.BgUSv <= 0 {
		return v
	}
	dose, h := rc.Dose, float64(rc.Minutes)/60
	if rc.DevOK {
		dose, h = rc.DevDose, span.Hours()
	}
	if h <= 0 {
		return v
	}
	v.OK, v.Dose, v.Expected = true, dose, rc.BgUSv*h
	v.Ratio = dose / v.Expected
	v.YearMSv = dose / h * 8766 / 1000
	switch {
	case v.Ratio <= 1.25:
		v.Level = "normal"
	case v.Ratio <= 2:
		v.Level = "acima"
	default:
		v.Level = "alto"
	}
	return v
}

// reportExtras junta o que a página 1–3 precisa e o relatório antigo não tinha.
func (a *App) reportExtras(r *reportData, o reportOpts) {
	r.Opts = o
	r.Verdict = doseVerdict(r.S.RC, r.To.Sub(r.From))
	if o.Summary {
		return
	}
	// alertas dos dois aparelhos, por episódio
	a.mu.Lock()
	var rcEvs []Event
	demo := a.demo.Active
	for _, e := range a.events {
		if e.Kind == kindAlert && e.Device == devRC && e.Time >= r.From.UnixMilli() && e.Time < r.To.UnixMilli() && (!e.Demo || demo) {
			c := e
			c.Active = a.isActiveLocked(e.ID)
			rcEvs = append(rcEvs, c)
		}
	}
	var periods []Period
	for _, p := range a.periods {
		if isTrip(p) && periodEnd(p) > r.From.UnixMilli() && p.From < r.To.UnixMilli() {
			periods = append(periods, p)
		}
	}
	a.mu.Unlock()
	now := time.Now()
	cc := func(dev string, from, to time.Time) string {
		switch a.GetCrossCheck(dev, from.UnixMilli(), to.UnixMilli(), false).State {
		case "ok":
			return "confirmado"
		case "no":
			return "não confirmado"
		}
		return ""
	}
	for _, e := range r.Episodes {
		r.Alerts2 = append(r.Alerts2, reportAlert2{Start: e.Start, Dev: "GMC-800", Level: e.LevelCls, Peak: e.PeakU, DurSec: e.DurSec,
			Confirmed: cc("gmc", e.Start, e.End)})
	}
	for _, ep := range alertEpisodes(rcEvs, now) {
		st := time.UnixMilli(ep.Start)
		end := now
		if ep.Until > 0 {
			end = time.UnixMilli(ep.Until)
		}
		r.Alerts2 = append(r.Alerts2, reportAlert2{Start: st, Dev: "RC102", Level: ep.Level, Peak: ep.Peak, DurSec: int(end.Sub(st).Seconds()),
			Confirmed: cc(devRC, st, end)})
	}
	sort.Slice(r.Alerts2, func(i, j int) bool { return r.Alerts2[i].Start.Before(r.Alerts2[j].Start) })

	// por dia: casa (GMC-800), sua dose (RC102), pico e alertas
	gmcDay := map[string]reportDay{}
	for i, d := range r.Days {
		gmcDay[r.dayKeys[i]] = d
	}
	d0 := time.Date(r.From.Year(), r.From.Month(), r.From.Day(), 0, 0, 0, 0, r.From.Location())
	for d := d0; d.Before(r.To) && len(r.Days2) < 92; d = d.AddDate(0, 0, 1) {
		f, t := d, d.AddDate(0, 0, 1)
		if f.Before(r.From) {
			f = r.From
		}
		if t.After(r.To) {
			t = r.To
		}
		row := reportDay2{Label: weekdayBR(d) + " " + d.Format("02/01")}
		if g, ok := gmcDay[d.Format("2006-01-02")]; ok {
			row.HomeU, row.HasHome = g.USv, true
			row.Peak, row.PeakDev = math.Max(g.MaxU, g.LivePeakU), "GMC-800"
		}
		if rc := a.rcSummary(f, t, t.Sub(f)+time.Minute, 1); rc != nil && rc.Minutes > 0 {
			row.Dose, row.HasDose = rc.Dose, true
			if rc.DevOK {
				row.Dose = rc.DevDose
			}
			if rc.Max > row.Peak {
				row.Peak, row.PeakDev = rc.Max, "RC102"
			}
		}
		for _, al := range r.Alerts2 {
			if !al.Start.Before(f) && al.Start.Before(t) {
				row.Alerts++
			}
		}
		if row.HasHome || row.HasDose || row.Alerts > 0 {
			r.Days2 = append(r.Days2, row)
		}
	}

	// saídas de casa: horário, dose e pico (e o espectro de cada uma)
	for _, p := range periods {
		ts := a.tripSummary(p, now)
		tr := reportTrip{Title: tripReportTitle(p), From: time.UnixMilli(ts.From), To: time.UnixMilli(ts.To), Spec: ts.Spec, SpecRows: ts.SpecRows}
		if rc := ts.RC; rc != nil && rc.Minutes > 0 {
			tr.Dose, tr.HasDose = rc.Dose, true
			if rc.DevOK {
				tr.Dose = rc.DevDose
			}
			tr.Peak, tr.PeakAt = rc.Max, time.UnixMilli(rc.MaxAt)
		}
		r.Trips = append(r.Trips, tr)
	}

	// o espectro do período inteiro contra o de casa
	ps := TripSummary{From: r.From.UnixMilli(), To: r.To.UnixMilli(), Lines: []TripLine{}}
	a.spectrumVsHome(&ps, true)
	if ps.Spec != nil {
		r.Spec, r.SpecLines = ps.Spec, ps.Lines
	}

	if o.Notes {
		r.Notes = a.GetNotes(r.From.UnixMilli(), r.To.UnixMilli())
	}
}

func weekdayBR(t time.Time) string {
	return []string{"dom", "seg", "ter", "qua", "qui", "sex", "sáb"}[t.Weekday()]
}

// tripReportTitle é o nome que o ACE deu à saída (nunca lugar do GPS); com o
// nome padrão, "Saída de ter 10/11".
func tripReportTitle(p Period) string {
	if isDefaultPeriodName(p.Name) {
		t := time.UnixMilli(p.From)
		return "Saída de " + weekdayBR(t) + " " + t.Format("02/01")
	}
	return strings.TrimSpace(p.Name)
}
