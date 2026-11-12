package app

import (
	"errors"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"radlog/internal/store"

	wruntime "github.com/wailsapp/wails/v2/pkg/runtime"
)

// ---------- relatório do período (aba Resumo → Relatório) ----------
//
// Um PDF (pdf.go/reportpdf.go) com os mesmos números da aba Resumo.

type reportDay struct {
	Label         string
	Avg, Min, Max float64 // por minuto
	USv, Dose     float64
	Minutes       int
	LivePeak      float64 // maior pico ao vivo (janela de 60 s) de alerta no dia; 0 = nenhum
	// os mesmos em µSv/h, cada minuto pela calibração que valia nele (2.36)
	MinU, MaxU, LivePeakU float64
}

type reportAlert struct {
	When, Level, LevelCls, Span, Peak string
}

type reportMeas struct {
	When, Name, Result, Verdict string
	M                           SavedMeasure // dados completos (contagens, tempos, anotação)
}

// reportBg é uma linha da comparação do fundo da casa.
type reportBg struct {
	Label string
	CPM   float64
	USv   float64
	N     int // minutos em casa na janela
	OK    bool
}

type reportData struct {
	Title, Period, Generated, Device, Version string
	S                                         Summary
	HasData                                   bool
	Avg, AvgUSv, Min, Max, MinAt, MaxAt       string
	Dose, DoseYear, Known, Bg, BgUSv, VsBg    string
	Coverage, Away                            string
	Alerts                                    string
	Rain                                      string
	Days                                      []reportDay
	AlertList                                 []reportAlert
	Meas                                      []reportMeas
	Cal, DeadTime                             string

	// relatório técnico (reportstats.go)
	From, To     time.Time     // período; To não passa de agora
	Pts          []store.Point // leituras por minuto do período
	St           minuteStats
	Gaps         []gap
	SpanMin      int // minutos do período (até agora)
	HistMin      int // minutos desde o primeiro dado do GMC (até agora)
	HistFrom     time.Time
	DaysData     int
	Bins         []histBin
	Episodes     []reportEpisode
	Above        []reportAbove
	Config       [][2]string // configurações e método (rótulo, valor)
	AlertPeakMax float64     // maior pico ao vivo dos alertas (janela de 60 s); só alertas ao vivo
	BgRows       []reportBg  // fundo: última hora, 7 dias, 30 dias
	dayKeys      []string    // "AAAA-MM-DD" de cada linha de Days

	// µSv/h (2.36): o relatório mostra o GMC na unidade da tela; o CPM fica
	// só na estatística da contagem e no método
	tl            calTL       // calibração de cada minuto
	USt           minuteStats // estatística dos minutos em µSv/h
	AlertPeakMaxU float64     // AlertPeakMax em µSv/h
	BgU           float64     // fundo de 30 dias em µSv/h
	alertPeakAt   time.Time
	MaxLive       bool // o máximo do resumo veio de um pico ao vivo

	// 2.6: configurações que valeram no período e maturidade do histórico
	CfgRows  []cfgPeriodRow
	CfgFirst time.Time // primeiro registro do histórico de configurações
	Mature   Maturity
	// fita dos dias (2.42): um dia por linha, em pedaços de 10 min
	Strip     DayStrip
	StripDays []time.Time
	// relatório novo (2.68, report2.go)
	Opts      reportOpts
	Verdict   reportDoseVerdict
	HomeU     float64 // média do GMC-800 só com ele em casa (µSv/h)
	HomeN     int     // minutos dessa média
	Days2     []reportDay2
	Alerts2   []reportAlert2
	Trips     []reportTrip
	Spec      *MeasureSpectrum
	SpecLines []TripLine
	Notes     []Note
	pages     int // total de páginas (da primeira passada), para "página N de M"
}

type reportEpisode struct {
	Start, End        time.Time
	Level, LevelCls   string
	Minutes           int
	N                 int // minutos com leitura dentro do episódio
	MinC, MeanC, MaxC float64
	Peak              float64 // pico do alerta (ao vivo: janela de 60 s lida por segundo)
	PeakAt            time.Time
	Limit             int
	// em µSv/h, pela calibração da hora (2.36)
	MinU, MeanU, MaxU, PeakU, LimitU float64
	Retro                            bool
	DurSec                           int    // duração exata (do histórico: de minuto a minuto)
	LimitNow                         bool   // limite pelo valor atual (sem registro da época)
	Breaks                           int    // trechos sem leitura no meio (desconexão)
	EndReason                        string // terminou sem leitura (desconexão, app fechado)
	// para o gráfico de antes e depois (só os episódios escolhidos)
	Chart  bool
	Around []store.Point // por minuto, 30 min antes e depois
	Secs   []AlertPoint  // por segundo, se o app gravou o detalhe
}

type reportAbove struct {
	Label   string
	Limit   float64 // µSv/h (2.36)
	Minutes int
	Limits  []float64 // limites vigentes no período (µSv/h), na ordem em que valeram
	UsedNow bool      // algum minuto sem registro: usou o valor atual
}

// uPt é o µSv/h de um minuto gravado, pela calibração que valia nele.
func (r reportData) uPt(p store.Point) float64 { return r.tl.USv(p) }

// uAt converte um CPM sem minuto gravado (pico ao vivo, limite) pela
// calibração de t.
func (r reportData) uAt(t time.Time, cpm float64) float64 { return r.tl.USvAt(t, cpm) }

// fmtU escreve um µSv/h com as casas da tela (3 algarismos significativos).
func fmtU(v float64) string { return fmtBR(v, usvDec(v)) }

func fmtBR(v float64, dec int) string {
	s := fmt.Sprintf("%.*f", dec, v)
	return strings.Replace(s, ".", ",", 1)
}

func levelName(l string) string {
	switch l {
	case "perigo":
		return "Perigo"
	case "alarme":
		return "Alarme"
	}
	return "Atenção"
}

func fmtMinutes(m int) string {
	if m < 60 {
		return fmt.Sprintf("%d min", m)
	}
	h, r := m/60, m%60
	if r == 0 {
		return fmt.Sprintf("%d h", h)
	}
	return fmt.Sprintf("%d h %02d min", h, r)
}

// fmtSeconds escreve uma duração exata: "45 s", "4 min 37 s", "1 h 02 min 05 s".
func fmtSeconds(sec int) string {
	if sec < 0 {
		sec = 0
	}
	h, m, ss := sec/3600, sec%3600/60, sec%60
	switch {
	case h > 0:
		return fmt.Sprintf("%d h %02d min %02d s", h, m, ss)
	case m > 0:
		return fmt.Sprintf("%d min %02d s", m, ss)
	}
	return fmt.Sprintf("%d s", ss)
}

// buildReport monta os dados do relatório de [from, to).
func (a *App) buildReport(fromMs, toMs int64, label string) (reportData, error) {
	s, err := a.GetSummary(fromMs, toMs)
	if err != nil {
		return reportData{}, err
	}
	from, to := time.UnixMilli(s.From), time.UnixMilli(s.To)
	a.mu.Lock()
	cal := a.convCalLocked()
	tl := a.calTLLocked()
	devSrc := a.cal.Source
	info := a.info
	connected := a.state == "conectado"
	demo := a.demo.Active
	eff := a.effLocked()
	lv := eff.Levels.Norm()
	attnU, alarmU, dangerU := thrUSvLocked(a, eff)
	var evs []Event
	for _, e := range a.events {
		if e.Kind == kindAlert && e.Time >= s.From && e.Time < s.To && (!e.Demo || demo) && e.Device != devRC {
			evs = append(evs, e)
		}
	}
	var meas []SavedMeasure
	for _, m := range a.measurements {
		if m.Time >= s.From && m.Time < s.To {
			meas = append(meas, m)
		}
	}
	a.mu.Unlock()

	r := reportData{
		Title:     "Relatório de radiação",
		Period:    fmt.Sprintf("%s · %s a %s", label, from.Format("02/01/2006 15:04"), to.Format("02/01/2006 15:04")),
		Generated: time.Now().Format("02/01/2006 15:04"),
		Version:   AppVersion,
		S:         s,
		HasData:   s.Minutes > 0,
	}
	if connected && info.Version != "" {
		r.Device = "GQ GMC-800"
		if info.Serial != "" {
			r.Device += " · série " + info.Serial
		}
	}
	if s.Avg != nil {
		r.Avg = fmtBR(*s.Avg, 1)
		r.AvgUSv = fmtBR(s.AvgUSv, 3)
		r.DoseYear = fmtBR(s.AvgUSv*24*365/1000, 2)
	}
	if s.Min != nil {
		r.Min = fmtBR(*s.Min, 0)
		r.MinAt = time.UnixMilli(s.MinAt).Format("02/01 15:04")
	}
	if s.Max != nil {
		r.Max = fmtBR(*s.Max, 0)
		r.MaxAt = time.UnixMilli(s.MaxAt).Format("02/01 15:04")
	}
	r.Dose = fmtBR(s.Dose, 2)
	if s.BgOK && s.BgCPM > 0 {
		r.Bg = fmtBR(s.BgCPM, 1)
		r.BgU = cal.USv(s.BgCPM)
		r.BgUSv = fmtU(r.BgU)
		if s.Avg != nil {
			sign := "+"
			if s.VsBgPct < 0 {
				sign = ""
			}
			r.VsBg = sign + fmtBR(s.VsBgPct, 1) + "%"
		}
	}
	// dose em coisas conhecidas (2.41): tempo de fundo da casa e uma referência
	r.Known = doseKnown(s.Dose, r.BgU)
	r.Coverage = fmtBR(s.Coverage*100, 0) + "%"
	if s.AwayMinutes > 0 {
		r.Away = fmtMinutes(s.AwayMinutes)
	}
	var parts []string
	if s.Danger > 0 {
		parts = append(parts, fmt.Sprintf("%d perigo", s.Danger))
	}
	if s.Alarm > 0 {
		parts = append(parts, plural(s.Alarm, "1 alarme", fmt.Sprintf("%d alarmes", s.Alarm)))
	}
	if s.Attention > 0 {
		parts = append(parts, plural(s.Attention, "1 atenção", fmt.Sprintf("%d atenções", s.Attention)))
	}
	r.Alerts = "nenhum"
	if len(parts) > 0 {
		r.Alerts = strings.Join(parts, ", ")
	}
	if s.RainKnown {
		r.Rain = fmt.Sprintf("%s mm em %s", fmtBR(s.RainMM, 1), plural(s.RainHours, "1 hora", fmt.Sprintf("%d horas", s.RainHours)))
		if s.RainHours == 0 {
			r.Rain = "sem chuva"
		}
	}

	// leituras por minuto do período (base de tudo o que vem abaixo)
	pts, err := a.points(from.Truncate(time.Minute), to)
	if err == nil {
		k := 0
		for _, p := range pts {
			if !p.T.Before(from) && p.T.Before(to) {
				pts[k] = p
				k++
			}
		}
		pts = pts[:k]
		sort.Slice(pts, func(i, j int) bool { return pts[i].T.Before(pts[j].T) })
	}
	effTo := to
	if now := time.Now(); effTo.After(now) {
		effTo = now
	}
	r.From, r.To, r.Pts, r.tl = from, effTo, pts, tl
	r.St = calcMinuteStats(pts)
	r.USt = usvMinuteStats(pts, tl)
	cs := coverageSpan(from, to, a.pointsFirst(pts), time.Now())
	r.SpanMin, r.HistMin, r.HistFrom = cs.Span, cs.Hist, cs.HistFrom
	if cs.Hist > 0 {
		r.Gaps = findGaps(pts, cs.HistFrom, effTo)
	}
	r.DaysData = daysWithData(pts)
	r.Mature = a.maturityAt(effTo)
	r.Bins = histogram(pts, r.St.Mean)
	if r.St.N > 0 {
		r.Min = fmtBR(r.St.Min, 0)
		r.MinAt = r.St.MinAt.Format("02/01 15:04")
		r.Max = fmtBR(r.St.Max, 0)
		r.MaxAt = r.St.MaxAt.Format("02/01 15:04")
	}
	r.Coverage = r.coverageTxt()

	// por dia
	if err == nil {
		type acc struct {
			sum, usum, dose, min, max, umin, umax float64
			n                                     int
		}
		byDay := map[int64]*acc{}
		var keys []int64
		for _, p := range pts {
			if p.T.Before(from) {
				continue
			}
			y, m, d := p.T.Date()
			k := time.Date(y, m, d, 0, 0, 0, 0, p.T.Location()).UnixMilli()
			x := byDay[k]
			u := tl.USv(p) // pela calibração que valia nesse minuto
			if x == nil {
				x = &acc{min: p.CPM, max: p.CPM, umin: u, umax: u}
				byDay[k] = x
				keys = append(keys, k)
			}
			x.sum += p.CPM
			x.usum += u
			x.dose += u / 60
			x.min = math.Min(x.min, p.CPM)
			x.max = math.Max(x.max, p.CPM)
			x.umin = math.Min(x.umin, u)
			x.umax = math.Max(x.umax, u)
			x.n++
		}
		sort.Slice(keys, func(i, j int) bool { return keys[i] < keys[j] })
		wd := []string{"dom", "seg", "ter", "qua", "qui", "sex", "sáb"}
		for _, k := range keys {
			x := byDay[k]
			t := time.UnixMilli(k)
			avg := x.sum / float64(x.n)
			r.Days = append(r.Days, reportDay{
				Label: wd[t.Weekday()] + " " + t.Format("02/01"),
				Avg:   avg, Min: x.min, Max: x.max, USv: x.usum / float64(x.n), Dose: x.dose, Minutes: x.n,
				MinU: x.umin, MaxU: x.umax,
			})
			r.dayKeys = append(r.dayKeys, t.Format("2006-01-02"))
		}
	}

	// episódios com a estatística dos minutos de dentro e o limite passado
	a.mu.Lock()
	alarmCPM, alarmOn := a.effLocked().AlarmCPM, a.settings.AlarmEnabled
	a.mu.Unlock()
	a.cfgMu.Lock()
	cfgHist := a.readCfgHist()
	a.cfgMu.Unlock()
	r.CfgRows, r.CfgFirst = cfgInPeriod(cfgHist, from, to)
	limitOf := func(level string) int {
		switch level {
		case "perigo":
			return lv.DangerCPM
		case "alarme":
			return alarmCPM
		}
		return lv.AttentionCPM
	}
	// o mesmo em µSv/h: o escolhido hoje (desde a 2.29 é o µSv/h que fica fixo)
	limitUOf := func(level string) float64 {
		switch level {
		case "perigo":
			return dangerU
		case "alarme":
			return alarmU
		}
		return attnU
	}
	evByID := map[int64]Event{}
	for _, e := range evs {
		evByID[e.ID] = e
	}
	livePeakDay, livePeakDayU := map[string]float64{}, map[string]float64{}
	for _, ep := range alertEpisodes(evs, time.Now()) {
		st, en := time.UnixMilli(ep.Start), time.UnixMilli(ep.Until)
		re := reportEpisode{Start: st, End: en, Level: levelName(ep.Level), LevelCls: ep.Level,
			Minutes: int(math.Ceil(en.Sub(st).Minutes())), Peak: ep.Peak, Limit: limitOf(ep.Level),
			DurSec: int(math.Round(en.Sub(st).Seconds())), PeakAt: st, Breaks: len(ep.Breaks), EndReason: ep.EndReason}
		// o limite que valia quando o episódio começou (histórico de configurações)
		if l, ok := cfgLimitAt(cfgHist, ep.Level, ep.Start); ok {
			re.Limit = l
		}
		if u, _, ok := cfgLimitUSvAt(cfgHist, ep.Level, ep.Start, tl); ok {
			re.LimitU = u
		} else {
			re.LimitU, re.LimitNow = limitUOf(ep.Level), true
		}
		if len(ep.IDs) > 0 {
			re.Retro = evByID[ep.IDs[0]].Retro
		}
		// hora do pico: o evento do episódio com o maior pico
		best := -1.0
		for _, id := range ep.IDs {
			e := evByID[id]
			if e.Peak > best {
				best = e.Peak
				if e.PeakAt != 0 {
					re.PeakAt = time.UnixMilli(e.PeakAt)
				} else {
					re.PeakAt = time.UnixMilli(e.Time)
				}
			}
		}
		if re.Retro {
			re.Minutes++ // do histórico: de minuto a minuto, inclusive o último
			re.DurSec += 60
		}
		in := pointsIn(pts, st.Truncate(time.Minute), en)
		if ms := calcMinuteStats(in); ms.N > 0 {
			re.N, re.MinC, re.MeanC, re.MaxC = ms.N, ms.Min, ms.Mean, ms.Max
			mu := usvMinuteStats(in, tl)
			re.MinU, re.MeanU, re.MaxU = mu.Min, mu.Mean, mu.Max
		}
		re.PeakU = tl.USvAt(re.PeakAt, ep.Peak)
		if !re.Retro {
			if ep.Peak > r.AlertPeakMax {
				r.AlertPeakMax, r.alertPeakAt = ep.Peak, re.PeakAt
			}
			r.AlertPeakMaxU = math.Max(r.AlertPeakMaxU, re.PeakU)
			k := re.PeakAt.Format("2006-01-02")
			livePeakDay[k] = math.Max(livePeakDay[k], ep.Peak)
			livePeakDayU[k] = math.Max(livePeakDayU[k], re.PeakU)
		}
		r.Episodes = append(r.Episodes, re)
	}
	for i := range r.Days {
		if v, ok := livePeakDay[r.dayKeys[i]]; ok {
			r.Days[i].LivePeak = v
			r.Days[i].LivePeakU = livePeakDayU[r.dayKeys[i]]
		}
	}
	// máximo do relatório = o mesmo do app: o maior valor visto, contando o
	// pico ao vivo dos alertas (que pode passar do maior minuto gravado)
	if r.St.N > 0 && r.AlertPeakMax > r.St.Max {
		r.Max = fmtBR(r.AlertPeakMax, 0)
		r.MaxAt = r.alertPeakAt.Format("02/01 15:04")
		r.MaxLive = true
	}
	// gráficos de antes e depois: até 12 episódios, os mais graves e altos
	if len(r.Episodes) > 0 {
		idx := make([]int, len(r.Episodes))
		for i := range idx {
			idx[i] = i
		}
		sort.SliceStable(idx, func(i, j int) bool {
			ei, ej := r.Episodes[idx[i]], r.Episodes[idx[j]]
			if levelRank[ei.LevelCls] != levelRank[ej.LevelCls] {
				return levelRank[ei.LevelCls] > levelRank[ej.LevelCls]
			}
			return ei.Peak > ej.Peak
		})
		if len(idx) > maxEpisodeCharts {
			idx = idx[:maxEpisodeCharts]
		}
		for _, i := range idx {
			ep := &r.Episodes[i]
			ep.Chart = true
			w0, w1 := ep.Start.Add(-30*time.Minute).Truncate(time.Minute), ep.End.Add(31*time.Minute)
			if around, err := a.points(w0, w1); err == nil {
				ep.Around = around
			}
			if !ep.Retro {
				ep.Secs = a.episodeSeconds(evs, ep.Start, ep.End)
			}
		}
	}
	// tempo acima de cada limite, com o limite vigente em cada minuto
	// (histórico de configurações; sem registro, o valor atual)
	above := func(label, level string, cur float64) reportAbove {
		ra := reportAbove{Label: label, Limit: cur}
		seen := map[string]bool{}
		for _, p := range pts {
			u := tl.USv(p)
			l, cpm, ok := cfgLimitUSvAt(cfgHist, level, p.T.UnixMilli(), tl)
			if !ok {
				l = cur
				if level == "alarme" && !alarmOn {
					continue
				}
				ra.UsedNow = true
			}
			if k := fmtU(l); !seen[k] {
				seen[k] = true
				ra.Limits = append(ra.Limits, l)
			}
			// limite gravado em CPM (antes da 2.29): o que tocava era o CPM
			if (cpm > 0 && p.CPM >= float64(cpm)) || (cpm == 0 && u >= l*(1-1e-9)) {
				ra.Minutes++
			}
		}
		return ra
	}
	r.Above = append(r.Above, above("Atenção", "atencao", attnU))
	if ra := above("Alarme", "alarme", alarmU); alarmOn || len(ra.Limits) > 0 {
		r.Above = append(r.Above, ra)
	}
	r.Above = append(r.Above, above("Perigo", "perigo", dangerU))

	// alertas por episódio (o mais recente primeiro)
	eps := alertEpisodes(evs, time.Now())
	for i := len(eps) - 1; i >= 0; i-- {
		ep := eps[i]
		st := time.UnixMilli(ep.Start)
		span := st.Format("15:04")
		if ep.Until > ep.Start {
			span += "–" + time.UnixMilli(ep.Until).Format("15:04") + " (" + fmtMinutes(int(math.Round(float64(ep.Until-ep.Start)/60000))) + ")"
		}
		pk := fmtU(tl.USvAt(st, ep.Peak)) + " µSv/h"
		if s.BgCPM > 0 {
			pk += fmt.Sprintf(" (%sσ)", fmtBR((ep.Peak-s.BgCPM)/math.Sqrt(s.BgCPM), 1))
		}
		r.AlertList = append(r.AlertList, reportAlert{When: st.Format("02/01"), Level: levelName(ep.Level), LevelCls: ep.Level, Span: span, Peak: pk})
	}

	for _, m := range meas {
		sign := "+"
		if m.Result.NetCPM < 0 {
			sign = ""
		}
		res := fmt.Sprintf("%s%s ± %s CPM", sign, fmtBR(m.Result.NetCPM, 1), fmtBR(m.Result.NetErr, 1))
		if m.Result.NetUSv != 0 {
			res += fmt.Sprintf(" (≈ %s µSv/h)", fmtBR(m.Result.NetUSv, 3))
		}
		verdict := m.Result.Verdict
		if i := strings.IndexAny(verdict, ":(—"); i > 0 {
			verdict = strings.TrimSpace(verdict[:i])
		}
		r.Meas = append(r.Meas, reportMeas{When: time.UnixMilli(m.Time).Format("02/01 15:04"), Name: m.Name, Result: res, Verdict: verdict, M: m})
	}

	r.Cal = fmtBR(cal.USv(100)/100, 4) + " µSv/h por CPM"
	calPeriod := tl.periodDesc(from.UnixMilli(), to.UnixMilli())
	if lv.DeadTimeOn {
		r.DeadTime = fmt.Sprintf("ligada (τ = %s µs)", fmtBR(lv.DeadTimeUs, 0))
	} else {
		r.DeadTime = "desligada"
	}

	// fundo da casa: agora × 7 dias × 30 dias
	now := time.Now()
	for _, w := range []struct {
		label string
		d     time.Duration
		min   int
	}{
		{"Última hora (agora)", time.Hour, 30},
		{"Mediana de 7 dias", 7 * 24 * time.Hour, bgMinCount},
		{"Mediana de 30 dias (referência)", bgWindow, bgMinCount},
	} {
		v, n := a.homeMedian(now.Add(-w.d), now.Add(time.Minute))
		row := reportBg{Label: w.label, CPM: v, N: n, OK: n >= w.min}
		if w.d == bgWindow && s.BgOK && s.BgCPM > 0 {
			row.CPM, row.OK = s.BgCPM, true // o mesmo número do resto do relatório
		}
		row.USv = cal.USv(row.CPM)
		r.BgRows = append(r.BgRows, row)
	}

	// configurações e método
	calSrc := "lida do contador agora"
	switch devSrc {
	case calFromLast:
		calSrc = "última lida do contador; ele não estava conectado"
	case "padrão":
		calSrc = "padrão de fábrica do GMC-800; o app ainda não leu a do contador"
	}
	alarm := "desligado"
	if alarmOn {
		alarm = fmt.Sprintf("a partir de %s µSv/h (≈ %d CPM com a calibração atual; limite operacional)", fmtU(alarmU), alarmCPM)
	}
	bg := "ainda sem fundo"
	if s.BgOK && s.BgCPM > 0 {
		bg = fmt.Sprintf("%s CPM (≈ %s µSv/h), mediana das leituras por minuto dos últimos 30 dias em casa", fmtBR(s.BgCPM, 1), fmtBR(cal.USv(s.BgCPM), 3))
	}
	dev := r.Device
	if dev == "" {
		dev = "contador não conectado na hora do relatório"
	}
	zone, _ := time.Now().Zone()
	// ajustes que estão na tabela do período (com valor conhecido no início)
	// saem daqui; sem registro da época, fica o valor atual, marcado como tal
	known := map[string]bool{}
	for _, c := range r.CfgRows {
		if c.Known {
			known[c.Field] = true
		}
	}
	curVal := func(field, v string) [2]string {
		return [2]string{field, v + " — valor atual; sem registro do valor vigente no início do período"}
	}
	r.Config = [][2]string{
		{"Contador", dev},
		{"Calibração", r.Cal + " (" + calSrc + ")"},
		{"Calibração no período", calPeriod},
	}
	if !known["Tempo morto"] {
		r.Config = append(r.Config, curVal("Tempo morto", r.DeadTime))
	}
	r.Config = append(r.Config,
		[2]string{"Registro de cada minuto", traceSummary(r.Pts)},
		[2]string{"Fundo de referência", bg},
	)
	if !known["Aviso de atenção"] {
		r.Config = append(r.Config, curVal("Aviso de atenção", fmt.Sprintf("a partir de %s µSv/h (≈ %d CPM com a calibração atual; limite operacional)", fmtU(attnU), lv.AttentionCPM)))
	}
	if !known["Alarme"] {
		r.Config = append(r.Config, curVal("Alarme", alarm))
	}
	if !known["Alarme de perigo"] {
		r.Config = append(r.Config, curVal("Alarme de perigo", fmt.Sprintf("a partir de %s µSv/h (≈ %d CPM com a calibração atual; limite operacional)", fmtU(dangerU), lv.DangerCPM)))
	}
	r.Config = append(r.Config, [][2]string{
		{"Dose do GMC", "calculada pelo app: soma, minuto a minuto, de µSv/h ÷ 60, com o µSv/h obtido do CPM pela calibração vigente em cada minuto. O GMC-800 não informa o acumulador interno de dose pela interface serial."},
		{"σ (Poisson)", "√média: desvio-padrão esperado apenas pela flutuação estatística de uma contagem de 1 minuto"},
		{"Percentis", "interpolação linear entre as leituras ordenadas (tipo 7)"},
		{"Máximo", "maior valor observado no resumo e na tabela por dia: o maior minuto gravado ou, se superior, o pico ao vivo de um alerta (marcado com *). A estatística usa apenas os minutos gravados."},
		{"Pico ao vivo", "maior CPM da leitura ao vivo durante um alerta. O contador soma os últimos 60 s e o app consulta a cada segundo; como essa janela não coincide com o minuto do relógio gravado, o pico pode superar o maior minuto."},
		{"Unidade", "o GMC-800 mede contagens (CPM); o relatório mostra µSv/h, com cada minuto, pico e limite convertidos pela calibração que valia naquele momento. Limites gravados em CPM (antes da 2.29) são comparados em CPM, como tocavam, e aparecem convertidos. A estatística da contagem fica em CPM."},
		{"Fundo", "fundo atual: mediana da última hora em casa; 7 e 30 dias: mediana das leituras por minuto em casa, excluídos os períodos fora de casa e as medições de objeto"},
		{"Cobertura", "minutos com leitura ÷ minutos do período até o fim do período ou o momento da geração, contados a partir do primeiro dado do contador"},
		{"Lacuna", "minuto sem leitura registrada (contador desconectado, app fechado sem histórico baixado); nunca é tratado como zero"},
		{"Horários", "hora local (" + zone + ")"},
		{"Programa", "RadLog " + AppVersion},
	}...)
	// fita dos dias do período (até 31)
	d0 := time.Date(from.Year(), from.Month(), from.Day(), 0, 0, 0, 0, from.Location())
	for d := d0; d.Before(to) && len(r.StripDays) < 31; d = d.AddDate(0, 0, 1) {
		r.StripDays = append(r.StripDays, d)
	}
	for i := 0; i < len(r.StripDays); i += 7 {
		f := r.StripDays[i]
		t := f.AddDate(0, 0, 7)
		if t.After(to) {
			t = to
		}
		if ds, err := a.GetDayStrip(f.UnixMilli(), t.UnixMilli()); err == nil {
			r.Strip.Slots = append(r.Strip.Slots, ds.Slots...)
			r.Strip.Marks = append(r.Strip.Marks, ds.Marks...)
		}
	}
	a.reportExtras(&r, a.reportOptsSaved())
	return r, nil
}

// defaultReportDir é Documentos/RadLog/Relatórios (no Linux, a pasta
// Documentos ou Documents que existir; senão a pasta pessoal).
func defaultReportDir() string {
	docs := docsBase()
	if docs == "" {
		return ""
	}
	return filepath.Join(docs, appDirName, "Relatórios")
}

// ReportDirInfo é o que as Configurações mostram.
type ReportDirInfo struct {
	Dir     string `json:"dir"`
	Default bool   `json:"default"` // usando a pasta padrão
}

func (a *App) reportDir() ReportDirInfo {
	a.mu.Lock()
	d := a.settings.ReportDir
	a.mu.Unlock()
	if d == "" {
		return ReportDirInfo{Dir: defaultReportDir(), Default: true}
	}
	return ReportDirInfo{Dir: d}
}

// GetReportDir devolve a pasta onde os relatórios são salvos.
func (a *App) GetReportDir() ReportDirInfo { return a.reportDir() }

// ChooseReportDir abre o seletor de pasta. Cancelar não muda nada.
func (a *App) ChooseReportDir() (ReportDirInfo, error) {
	if a.ctx == nil {
		return a.reportDir(), errors.New("sem janela")
	}
	cur := a.reportDir().Dir
	_ = os.MkdirAll(cur, 0o755)
	dir, err := wruntime.OpenDirectoryDialog(a.ctx, wruntime.OpenDialogOptions{Title: "Pasta dos relatórios", DefaultDirectory: cur, CanCreateDirectories: true})
	if err != nil || dir == "" {
		return a.reportDir(), err
	}
	a.mu.Lock()
	a.settings.ReportDir = dir
	a.saveSettingsLocked()
	a.mu.Unlock()
	a.logf("relatórios: pasta trocada para %s", dir)
	return a.reportDir(), nil
}

// ResetReportDir volta para a pasta padrão.
func (a *App) ResetReportDir() ReportDirInfo {
	a.mu.Lock()
	a.settings.ReportDir = ""
	a.saveSettingsLocked()
	a.mu.Unlock()
	return a.reportDir()
}

// OpenReportDir abre a pasta dos relatórios.
func (a *App) OpenReportDir() error {
	d := a.reportDir().Dir
	if err := os.MkdirAll(d, 0o755); err != nil {
		return err
	}
	return openFolder(d)
}

// uniquePath não sobrescreve: nome.pdf, nome-2.pdf, nome-3.pdf...
func uniquePath(p string) string {
	if _, err := os.Stat(p); err != nil {
		return p
	}
	ext := filepath.Ext(p)
	base := strings.TrimSuffix(p, ext)
	for i := 2; ; i++ {
		q := fmt.Sprintf("%s-%d%s", base, i, ext)
		if _, err := os.Stat(q); err != nil {
			return q
		}
	}
}

// writeReportFile grava o PDF com um nome livre (uniquePath) e devolve o
// caminho. O nome é escolhido na hora de gravar, sob reportFileMu: antes era
// escolhido antes de montar o relatório, e relatórios gerados ao mesmo tempo
// pegavam o mesmo nome e se sobrescreviam.
func (a *App) writeReportFile(want string, body []byte) (string, error) {
	a.reportFileMu.Lock()
	defer a.reportFileMu.Unlock()
	dst := uniquePath(want)
	f, err := os.OpenFile(dst, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o644)
	if err != nil {
		return "", err
	}
	if _, err := f.Write(body); err != nil {
		f.Close()
		_ = os.Remove(dst)
		return "", err
	}
	if err := f.Close(); err != nil {
		_ = os.Remove(dst)
		return "", err
	}
	return dst, nil
}

// errReportBusy: já há um relatório sendo gerado.
var errReportBusy = errors.New("já estou gerando um relatório; espere ele abrir")

// ExportReport salva o relatório (PDF) do período na pasta dos relatórios e
// abre no leitor padrão. Devolve o caminho completo. Um por vez: um segundo
// pedido enquanto o primeiro gera volta com errReportBusy.
// ExportReportWith salva as escolhas do diálogo "Relatório (PDF)" (2.68) e
// gera o relatório: mode "completo" ou "resumo", as anotações e o anexo das
// configurações vigentes.
func (a *App) ExportReportWith(fromMs, toMs int64, label, mode string, notes, cfg bool) (string, error) {
	if mode != "resumo" {
		mode = "completo"
	}
	a.mu.Lock()
	a.settings.ReportMode, a.settings.ReportNotes, a.settings.ReportNoCfg = mode, notes, !cfg
	a.saveSettingsLocked()
	a.mu.Unlock()
	return a.ExportReport(fromMs, toMs, label)
}

// reportOptsSaved são as últimas escolhas do diálogo.
func (a *App) reportOptsSaved() reportOpts {
	a.mu.Lock()
	defer a.mu.Unlock()
	return reportOpts{Summary: a.settings.ReportMode == "resumo", Notes: a.settings.ReportNotes, Cfg: !a.settings.ReportNoCfg}
}

func (a *App) ExportReport(fromMs, toMs int64, label string) (string, error) {
	if !a.reportBusy.TryLock() {
		return "", errReportBusy
	}
	defer a.reportBusy.Unlock()
	dir := a.reportDir().Dir
	if dir == "" {
		return "", errors.New("não achei uma pasta para salvar (escolha uma nas Configurações → Relatórios)")
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", fmt.Errorf("não consegui criar a pasta %s: %w", dir, err)
	}
	slug := strings.Trim(badFileChars.ReplaceAllString(strings.ReplaceAll(strings.ToLower(label), " ", "-"), ""), "-")
	if slug == "" {
		slug = "periodo"
	}
	from, to := time.UnixMilli(fromMs), time.UnixMilli(toMs)
	span := from.Format("2006-01-02")
	if to.Sub(from) < 26*time.Hour && from.YearDay() == to.Add(-time.Millisecond).YearDay() {
		if to.Sub(from) < 23*time.Hour {
			span += "_" + from.Format("15h04") + "-" + to.Format("15h04")
		}
	} else {
		span += "_a_" + to.Add(-time.Millisecond).Format("2006-01-02")
	}
	r, err := a.buildReport(fromMs, toMs, label)
	if err != nil {
		return "", err
	}
	body := a.inLang(func() []byte { return renderReportPDF(r) })
	dst, err := a.writeReportFile(filepath.Join(dir, fmt.Sprintf("gmc-relatorio-%s-%s.pdf", slug, span)), body)
	if err != nil {
		return "", err
	}
	a.logf("relatório salvo: %s", dst)
	_ = openFolder(dst) // abre no leitor de PDF padrão
	return dst, nil
}

// traceSummary resume o que se sabe de cada minuto do período (2.1): quantos
// têm bruto e corrigido gravados, com a correção ligada ou não, quantos são
// de antes da 2.1 e as marcas.
func traceSummary(pts []store.Point) string {
	var has, on, legacy, seg, sat, intf int
	for _, p := range pts {
		if p.Src == "demo" {
			continue
		}
		if !p.Has {
			legacy++
			continue
		}
		has++
		if p.DTOn {
			on++
		}
		if p.HasObs(store.ObsSeconds) {
			seg++
		}
		if p.HasObs(store.ObsSaturation) {
			sat++
		}
		if p.HasObs(store.ObsInterference) {
			intf++
		}
	}
	var parts []string
	if has > 0 {
		parts = append(parts, fmt.Sprintf("%d com CPM bruto e corrigido gravados (correção de tempo morto ligada em %d)", has, on))
	}
	if legacy > 0 {
		parts = append(parts, fmt.Sprintf("%d de antes da 2.1, sem esse registro (até a 2.0 a correção nunca foi ligada: bruto = gravado)", legacy))
	}
	if seg+sat+intf > 0 {
		parts = append(parts, fmt.Sprintf("marcas: %d vindos de gravação por segundo, %d com saturação, %d com interferência", seg, sat, intf))
	}
	if len(parts) == 0 {
		return "sem minutos no período"
	}
	return strings.Join(parts, "; ")
}

// coverageTxt é a cobertura com os denominadores: desde o primeiro dado e,
// se o período começa antes dele, também no período inteiro.
func (r reportData) coverageTxt() string {
	pct := ratio(r.St.N, r.HistMin) * 100
	if r.HistMin >= r.SpanMin {
		return fmt.Sprintf("%s%% · %d de %d min", fmtBR(pct, 0), r.St.N, r.SpanMin)
	}
	return fmt.Sprintf("%s%% · %d de %d min desde o primeiro dado (%s) · %s%% do período inteiro (%d min)",
		fmtBR(pct, 0), r.St.N, r.HistMin, r.HistFrom.Format("02/01/2006 15:04"), fmtBR(ratio(r.St.N, r.SpanMin)*100, 0), r.SpanMin)
}
