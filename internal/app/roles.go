package app

import (
	"errors"
	"sort"
	"strings"
	"time"

	"radlog/internal/store"
)

// ---------- onde está cada aparelho ----------
//
// Cada aparelho está "em casa" (parado, medindo a casa) ou "comigo" (anda com
// você). O normal é o GMC-800 em casa e o RadiaCode com você. Os trechos fora
// do normal são os períodos de "Fora de casa" (periods.go), que dizem onde
// estava cada um; os de antes, sem isso, são "os dois com você".
//
//	GMC     RadiaCode  casa (fundo e dose)        dose pessoal
//	casa    comigo     GMC                        RadiaCode           (normal)
//	comigo  casa       RadiaCode                  GMC                 (trocados)
//	comigo  comigo     sem medição                RadiaCode; o GMC preenche as falhas
//	casa    casa       GMC; o RadiaCode preenche  sem dosímetro
//
// Fundo da casa: o GMC só conta em casa; o RadiaCode conta em casa e também
// com você enquanto o GMC está em casa (você em casa, como sempre foi), mas
// não quando os dois saíram com você.

// Locais.
const (
	WhereHome = "casa"
	WhereMe   = "comigo"
)

// normWhere confere o local de cada aparelho num trecho ("" = comigo).
func normWhere(gmc, rc string) (string, string, error) {
	if gmc == "" {
		gmc = WhereMe
	}
	if rc == "" {
		rc = WhereMe
	}
	for _, w := range []string{gmc, rc} {
		if w != WhereHome && w != WhereMe {
			return gmc, rc, errors.New("local inválido")
		}
	}
	if gmc == WhereHome && rc == WhereMe {
		return gmc, rc, errors.New("GMC-800 em casa e RadiaCode com você é o normal: não precisa marcar")
	}
	return gmc, rc, nil
}

// periodWhere é onde estava cada aparelho num trecho.
func periodWhere(p Period) (gmc, rc string) {
	gmc, rc = p.GMC, p.RC
	if gmc == "" {
		gmc = WhereMe
	}
	if rc == "" {
		rc = WhereMe
	}
	return
}

// periodName é o nome padrão de um trecho.
func periodName(gmc, rc string) string {
	switch {
	case gmc == WhereMe && rc == WhereMe:
		return "Levei os dois"
	case gmc == WhereMe:
		return "Levei o GMC-800, deixei o RadiaCode"
	default:
		return "Deixei os dois em casa"
	}
}

// periodEnd é o fim de um trecho (o aberto vai até depois de qualquer instante).
func periodEnd(p Period) int64 {
	if p.To == 0 {
		return 1<<62 - 1
	}
	return p.To
}

// pickRoles diz qual aparelho dá a dose pessoal e qual a da casa ("" =
// nenhum), e qual preenche as falhas dele.
func pickRoles(gmc, rc string) (personal, personalFill, home, homeFill string) {
	switch {
	case rc == WhereMe && gmc == WhereMe:
		personal, personalFill = devRC, "gmc"
	case rc == WhereMe:
		personal = devRC
	case gmc == WhereMe:
		personal = "gmc"
	}
	switch {
	case gmc == WhereHome && rc == WhereHome:
		home, homeFill = "gmc", devRC
	case gmc == WhereHome:
		home = "gmc"
	case rc == WhereHome:
		home = devRC
	}
	return
}

// Whereabouts diz onde está cada aparelho agora.
type Whereabouts struct {
	GMC      string  `json:"gmc"`       // casa, comigo
	RC       string  `json:"radiacode"` // casa, comigo
	Normal   bool    `json:"normal"`    // GMC em casa e RadiaCode com você
	Personal string  `json:"personal"`  // aparelho da dose pessoal ("" = sem dosímetro)
	Home     string  `json:"home"`      // aparelho da casa ("" = sem medição)
	Since    int64   `json:"since"`     // ms: começo do trecho aberto (0 = normal)
	Open     *Period `json:"open"`      // o trecho aberto, se houver
}

func (a *App) whereaboutsLocked(now time.Time) Whereabouts {
	w := Whereabouts{GMC: WhereHome, RC: WhereMe}
	ms := now.UnixMilli()
	for i, p := range a.periods {
		if ms >= p.From && ms < periodEnd(p) {
			w.GMC, w.RC = periodWhere(p)
			w.Since = p.From
			if p.To == 0 {
				q := a.periods[i]
				w.Open = &q
			}
		}
	}
	w.Normal = w.GMC == WhereHome && w.RC == WhereMe
	w.Personal, _, w.Home, _ = pickRoles(w.GMC, w.RC)
	return w
}

// GetWhereabouts diz onde está cada aparelho agora.
func (a *App) GetWhereabouts() Whereabouts {
	a.mu.Lock()
	defer a.mu.Unlock()
	return a.whereaboutsLocked(time.Now())
}

func (a *App) emitPeriods() {
	a.emit("periods", a.GetPeriods())
	a.emit("whereabouts", a.GetWhereabouts())
}

// SetWhere diz onde está cada aparelho a partir de agora (gmc e rc: casa ou
// comigo). Fecha o trecho aberto e, fora do normal, abre outro. name é
// opcional.
func (a *App) SetWhere(gmc, rc, name string) (Whereabouts, error) {
	return a.setWhere(time.Now(), gmc, rc, name)
}

func (a *App) setWhere(now time.Time, gmc, rc, name string) (Whereabouts, error) {
	for _, w := range []string{gmc, rc} {
		if w != WhereHome && w != WhereMe {
			return a.GetWhereabouts(), errors.New("local inválido")
		}
	}
	ms := now.Truncate(time.Minute).UnixMilli()
	a.mu.Lock()
	if cur := a.whereaboutsLocked(now); cur.GMC == gmc && cur.RC == rc {
		a.mu.Unlock()
		return a.GetWhereabouts(), nil // já está assim
	}
	// fecha o trecho aberto (ou o que cobre agora: não deixa cruzar)
	for i := 0; i < len(a.periods); i++ {
		p := &a.periods[i]
		if ms < p.From || ms >= periodEnd(*p) {
			continue
		}
		if ms-p.From < 60_000 { // mudou de novo no mesmo minuto: não sobra trecho
			a.periods = append(a.periods[:i], a.periods[i+1:]...)
			i--
		} else {
			p.To = ms
		}
	}
	if !(gmc == WhereHome && rc == WhereMe) {
		var max int64
		for _, p := range a.periods {
			max = maxInt64(max, p.ID)
		}
		name = strings.TrimSpace(name)
		if name == "" {
			name = periodName(gmc, rc)
		}
		p := Period{ID: max + 1, From: ms, Name: name}
		p.GMC, p.RC = gmc, rc
		// um trecho marcado depois de agora (raro: relógio voltou) não pode cruzar
		for _, q := range a.periods {
			if q.From > ms && (p.To == 0 || q.From < p.To) {
				p.To = q.From
			}
		}
		a.periods = append(a.periods, p)
		sort.Slice(a.periods, func(i, j int) bool { return a.periods[i].From < a.periods[j].From })
	}
	a.bgAt = time.Time{} // o fundo da casa muda: recalcula
	err := a.savePeriodsLocked()
	a.mu.Unlock()
	where := map[string]string{WhereHome: "em casa", WhereMe: "com você"}
	a.logEvent("GMC-800 " + where[gmc] + ", RadiaCode " + where[rc])
	a.emitPeriods()
	return a.GetWhereabouts(), err
}

// mergePeriods junta trechos vindos de um backup ou de outro PC. A chave é o
// começo e o fim; um trecho aberto aqui que chegou fechado de lá é fechado.
func (a *App) mergePeriods(list []Period) int {
	a.mu.Lock()
	defer a.mu.Unlock()
	n := 0
	var max int64
	have := map[[2]int64]bool{}
	open := map[int64]int{}
	for i, p := range a.periods {
		have[[2]int64{p.From, p.To}] = true
		if p.To == 0 {
			open[p.From] = i
		}
		max = maxInt64(max, p.ID)
	}
	for _, p := range list {
		if have[[2]int64{p.From, p.To}] || (p.To != 0 && p.To <= p.From) {
			continue
		}
		if i, ok := open[p.From]; ok && p.To != 0 {
			a.periods[i].To = p.To
			delete(open, p.From)
			n++
			continue
		}
		if p.To == 0 { // aberto lá, fechado (ou ausente) aqui: o daqui vale
			if _, ok := open[p.From]; ok {
				continue
			}
		}
		max++
		p.ID = max
		a.periods = append(a.periods, p)
		have[[2]int64{p.From, p.To}] = true
		n++
	}
	if n > 0 {
		sort.Slice(a.periods, func(i, j int) bool { return a.periods[i].From < a.periods[j].From })
		_ = a.savePeriodsLocked()
		a.bgAt = time.Time{}
	}
	return n
}

// whereChecker diz onde estava cada aparelho num instante (cópia dos
// trechos, para usar sem segurar o lock).
func (a *App) whereChecker() func(time.Time) (gmc, rc string) {
	a.mu.Lock()
	list := append([]Period{}, a.periods...)
	a.mu.Unlock()
	return func(t time.Time) (string, string) {
		ms := t.UnixMilli()
		for _, p := range list { // poucos trechos: busca simples
			if ms >= p.From && ms < periodEnd(p) {
				return periodWhere(p)
			}
		}
		return WhereHome, WhereMe
	}
}

// awayCheckerFor diz se um aparelho estava fora do fundo da casa: o GMC
// quando estava com você; o RadiaCode quando os dois estavam com você.
func (a *App) awayCheckerFor(dev string) func(time.Time) bool {
	where := a.whereChecker()
	return func(t time.Time) bool {
		g, r := where(t)
		if dev == devRC {
			return r == WhereMe && g == WhereMe
		}
		return g == WhereMe
	}
}

// rcAwayChecker é o "fora de casa" do RadiaCode.
func (a *App) rcAwayChecker() func(time.Time) bool { return a.awayCheckerFor(devRC) }

// rcHomeOnly tira os pontos do RadiaCode fora de casa e das medições.
func (a *App) rcHomeOnly(pts []store.Point) []store.Point {
	away := a.rcAwayChecker()
	measuring := a.measuringChecker()
	out := pts[:0:0]
	for _, p := range pts {
		if !away(p.T) && !measuring(p.T) {
			out = append(out, p)
		}
	}
	return out
}

// periodsIn diz se algum trecho cruza [from, to).
func (a *App) periodsIn(from, to time.Time) bool {
	a.mu.Lock()
	defer a.mu.Unlock()
	f, t := from.UnixMilli(), to.UnixMilli()
	for _, p := range a.periods {
		if p.From < t && f < periodEnd(p) {
			return true
		}
	}
	return false
}

// roleDose é a dose de um papel ("pessoal" ou "casa") em [from, to): em
// cada minuto, a do aparelho que tinha esse papel (ou a do que preenche as
// falhas dele). Sem trecho no período, é a dose do próprio aparelho (com a
// dose que o RadiaCode soma, se houver). Com trecho, é a soma dos minutos
// gravados, e o fundo de cada minuto é o do aparelho que mediu. Os minutos
// em que ninguém tinha o papel são lacuna: não contam (GapMinutes).
func (a *App) roleDose(from, to time.Time, step time.Duration, limit, role string) (DoseChart, error) {
	if role != "pessoal" {
		role = "casa"
	}
	normal := map[string]string{"pessoal": devRC, "casa": "gmc"}[role]
	if !a.periodsIn(from, to) {
		return a.buildDose(from, to, step, limit, normal)
	}
	lim, ok := doseLimits[limit]
	if !ok {
		lim = doseLimits["publico"]
	}
	rcPts, rcBg, _, err := a.doseSeries(devRC, from, to)
	if err != nil {
		return DoseChart{}, err
	}
	gPts, gBg, _, err := a.doseSeries("gmc", from, to)
	if err != nil {
		return DoseChart{}, err
	}
	where := a.whereChecker()
	pick := func(t time.Time) (main, fill string) {
		p, pf, h, hf := pickRoles(where(t))
		if role == "pessoal" {
			return p, pf
		}
		return h, hf
	}
	type rm struct {
		USv, Bg float64 // µSv/h
		dev     string
	}
	byMin := map[int64]rm{}
	bgOf := map[string]float64{devRC: rcBg, "gmc": gBg}
	for _, dev := range []string{devRC, "gmc"} {
		pts := rcPts
		if dev == "gmc" {
			pts = gPts
		}
		for _, p := range pts {
			main, fill := pick(p.T)
			k := p.T.Truncate(time.Minute).UnixMilli()
			switch dev {
			case main:
				byMin[k] = rm{p.USv, bgOf[dev], dev} // o principal sempre vale
			case fill:
				if _, ok := byMin[k]; !ok {
					byMin[k] = rm{p.USv, bgOf[dev], dev}
				}
			}
		}
	}
	keys := make([]int64, 0, len(byMin))
	for k := range byMin {
		keys = append(keys, k)
	}
	sort.Slice(keys, func(i, j int) bool { return keys[i] < keys[j] })

	c := DoseChart{
		From: from.UnixMilli(), To: to.UnixMilli(), StepMs: step.Milliseconds(),
		LimitName: lim.Name, LimitMSvYear: lim.MSvYear,
		Device: normal, DoseSource: "app",
		BackgroundSource: "fundo de casa de cada aparelho, nos minutos em que ele mediu",
	}
	limPerMin := lim.MSvYear * 1000 / minutesPerYear
	n := int(to.Sub(from) / step)
	i := 0
	var dose, bg, allow float64
	for k := 1; k <= n; k++ {
		end := from.Add(time.Duration(k) * step).UnixMilli()
		for i < len(keys) && keys[i] < end {
			m := byMin[keys[i]]
			dose += m.USv / 60
			bg += m.Bg / 60
			allow += limPerMin
			c.MeasuredMinutes++
			if m.dev != normal {
				c.SwapMinutes++
			}
			i++
		}
		c.Times = append(c.Times, end)
		c.Dose = append(c.Dose, dose)
		c.Background = append(c.Background, bg)
		c.Ceiling = append(c.Ceiling, bg+allow)
	}
	c.Total, c.BackgroundTotal, c.Allowance = dose, bg, allow
	c.CeilingTotal = bg + allow
	c.Excess = dose - bg
	if c.MeasuredMinutes > 0 {
		c.BackgroundRate = bg / float64(c.MeasuredMinutes) * 60 // média dos minutos medidos
	}
	if c.SwapMinutes > 0 {
		c.Device, c.Mixed = "misto", c.SwapMinutes < c.MeasuredMinutes
		if !c.Mixed {
			c.Device = map[string]string{devRC: "gmc", "gmc": devRC}[normal]
		}
	}
	c.GapMinutes = a.roleGapMinutes(from, to, role)
	first := a.gmcFirst()
	if f := a.rcFirst(); !f.IsZero() && (first.IsZero() || f.Before(first)) {
		first = f
	}
	if len(keys) > 0 && (first.IsZero() || keys[0] < first.UnixMilli()) {
		first = time.UnixMilli(keys[0])
	}
	cs := coverageSpan(from, to, first, time.Now())
	c.Coverage, c.HistMinutes, c.SpanMinutes = ratio(c.MeasuredMinutes, max(cs.Hist-c.GapMinutes, 0)), cs.Hist, cs.Span
	if cs.Pre > 0 && cs.Hist > 0 {
		c.HistFrom = cs.HistFrom.UnixMilli()
	}
	return c, nil
}

// roleGapMinutes são os minutos de [from, min(to, agora)) em que nenhum
// aparelho tinha o papel (os dois com você: casa sem medição; os dois em
// casa: sem dosímetro).
func (a *App) roleGapMinutes(from, to time.Time, role string) int {
	if now := time.Now(); to.After(now) {
		to = now
	}
	a.mu.Lock()
	list := append([]Period{}, a.periods...)
	a.mu.Unlock()
	var ms int64
	for _, p := range list {
		g, r := periodWhere(p)
		pm, _, h, _ := pickRoles(g, r)
		if (role == "pessoal" && pm != "") || (role != "pessoal" && h != "") {
			continue
		}
		f, t := maxInt64(p.From, from.UnixMilli()), periodEnd(p)
		if t > to.UnixMilli() {
			t = to.UnixMilli()
		}
		if t > f {
			ms += t - f
		}
	}
	return int(ms / 60_000)
}
