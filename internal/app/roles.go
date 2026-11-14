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
// você). O normal é o GMC-800 em casa e o Radiacode com você. Os trechos fora
// do normal são os períodos de "Fora de casa" (periods.go), que dizem onde
// estava cada um; os de antes, sem isso, são "os dois com você".
//
//	GMC     Radiacode  casa (fundo e dose)        dose pessoal
//	casa    comigo     GMC                        Radiacode           (normal)
//	comigo  casa       Radiacode                  GMC                 (trocados)
//	comigo  comigo     sem medição                Radiacode; o GMC preenche as falhas
//	casa    casa       GMC; o Radiacode preenche  sem dosímetro
//
// Um aparelho também pode ficar "fora do cálculo" (quebrou, leitura não
// confiável): sai do fundo e da dose, e o papel dele fica com o outro, se o
// outro estiver no mesmo lugar, ou vira lacuna.
//
// Fundo da casa: o GMC só conta em casa; o Radiacode conta em casa e também
// com você enquanto o GMC não está com você (você em casa, como sempre foi),
// mas não quando os dois saíram com você nem fora do cálculo.
//
// Desconectado ou desligado não se marca: é minuto sem dado, que não entra
// na dose nem no fundo (NoDataMinutes); o estado ao vivo de cada aparelho
// vem em Whereabouts.Live.

// Locais.
const (
	WhereHome = "casa"
	WhereMe   = "comigo"
	WhereOff  = "fora" // fora do cálculo
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
		if w != WhereHome && w != WhereMe && w != WhereOff {
			return gmc, rc, errors.New("local inválido")
		}
	}
	if gmc == WhereHome && rc == WhereMe {
		return gmc, rc, errors.New("GMC-800 em casa e RC102 com você é o normal: não precisa marcar")
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
	case gmc == WhereOff && rc == WhereOff:
		return "Os dois fora do cálculo"
	case gmc == WhereOff:
		return "GMC-800 fora do cálculo"
	case rc == WhereOff:
		return "RC102 fora do cálculo"
	case gmc == WhereMe && rc == WhereMe:
		return "Levei os dois"
	case gmc == WhereMe:
		return "Levei o GMC-800, deixei o RC102"
	default:
		return "Deixei os dois em casa"
	}
}

// periodNameOut é o nome padrão de "saí com o Radiacode, o GMC ficou".
const periodNameOut = "Saí com o RC102"

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

// periodRoles é o pickRoles de um trecho: com os dois em casa você também
// estava em casa (a dose pessoal usa a leitura de casa), a não ser que o
// trecho diga "eu não estava em casa".
func periodRoles(p Period) (personal, personalFill, home, homeFill string) {
	g, r := periodWhere(p)
	personal, personalFill, home, homeFill = pickRoles(g, r)
	if g == WhereHome && r == WhereHome && !p.NotHome {
		personal, personalFill = home, homeFill
	}
	return
}

// Whereabouts diz onde está cada aparelho agora.
type Whereabouts struct {
	GMC      string  `json:"gmc"`       // casa, comigo
	RC       string  `json:"radiacode"` // casa, comigo
	Normal   bool    `json:"normal"`    // GMC em casa e RC102 com você
	Personal string  `json:"personal"`  // aparelho da dose pessoal ("" = sem dosímetro)
	Home     string  `json:"home"`      // aparelho da casa ("" = sem medição)
	Since    int64   `json:"since"`     // ms: começo do trecho aberto (0 = normal)
	NotHome  bool    `json:"notHome"`   // os dois em casa, mas você não estava
	Open     *Period `json:"open"`      // o trecho aberto, se houver
	// estado ao vivo de cada aparelho (só em GetWhereabouts)
	Live map[string]DevLive `json:"live,omitempty"`
}

// DevLive é o estado ao vivo de um aparelho.
type DevLive struct {
	Connected bool   `json:"connected"`
	State     string `json:"state"` // lendo, saturado, sem contagens, sem dado
	Since     int64  `json:"since"` // ms: começo de "sem contagens" (0 nos outros)
	Last      int64  `json:"last"`  // ms do último minuto gravado (0 = nenhum em 2 dias)
	// último minuto bom (contou, sem saturar): o "desde a última leitura boa"
	// de tirar do cálculo
	LastGood int64 `json:"lastGood"`
	// Radiacode longe do PC: ele continua somando a dose sozinho, e a dose
	// pessoal sai do total dele quando reconectar
	CountsOffline bool `json:"countsOffline,omitempty"`
}

// devLive diz se cada aparelho está lendo agora.
func (a *App) devLive(now time.Time) map[string]DevLive {
	const fresh = 3 * time.Minute
	out := map[string]DevLive{}
	g := DevLive{State: "sem dado"}
	a.mu.Lock()
	g.Connected = a.state == "conectado"
	nc := a.noCount
	rcSat := a.effLocked().Levels.Norm().RcSatUSv
	a.mu.Unlock()
	if a.store != nil {
		if pts, _ := a.store.Range(now.Add(-48*time.Hour), now.Add(time.Minute)); len(pts) > 0 {
			last := pts[len(pts)-1]
			g.Last = last.T.UnixMilli()
			for i := len(pts) - 1; i >= 0; i-- {
				if pts[i].CPM > 0 && !pts[i].HasObs(store.ObsSaturation) {
					g.LastGood = pts[i].T.UnixMilli()
					break
				}
			}
			if now.Sub(last.T) <= fresh {
				g.State = "lendo"
				if last.HasObs(store.ObsSaturation) {
					g.State = "saturado"
				}
			}
		}
	}
	if nc.on && g.Connected {
		g.State, g.Since = "sem contagens", nc.since.UnixMilli()
	}
	out["gmc"] = g
	r := DevLive{State: "sem dado"}
	a.rc.mu.Lock()
	r.Connected = a.rc.st.State == "conectado"
	// fora da escala: o aviso do próprio aparelho ou o fim da escala do app
	rcOver := r.Connected && (a.rc.st.Offscale || (rcSat > 0 && a.rc.st.USvh >= rcSat))
	a.rc.mu.Unlock()
	if pts := a.rcRange(now.Add(-48*time.Hour), now.Add(time.Minute)); len(pts) > 0 {
		r.Last = pts[len(pts)-1].T.UnixMilli()
		r.LastGood = r.Last
		if now.Sub(pts[len(pts)-1].T) <= fresh {
			r.State = "lendo"
			if rcOver {
				r.State = "saturado"
			}
		}
	}
	r.CountsOffline = !r.Connected
	out[devRC] = r
	return out
}

func (a *App) whereaboutsLocked(now time.Time) Whereabouts {
	w := Whereabouts{GMC: WhereHome, RC: WhereMe}
	w.Personal, _, w.Home, _ = pickRoles(w.GMC, w.RC)
	ms := now.UnixMilli()
	for i, p := range a.periods {
		if ms >= p.From && ms < periodEnd(p) {
			w.GMC, w.RC = periodWhere(p)
			w.NotHome = p.NotHome
			w.Personal, _, w.Home, _ = periodRoles(p)
			w.Since = p.From
			if p.To == 0 {
				q := a.periods[i]
				w.Open = &q
			}
		}
	}
	w.Normal = w.GMC == WhereHome && w.RC == WhereMe && !w.NotHome
	return w
}

// GetWhereabouts diz onde está cada aparelho agora e se está lendo.
func (a *App) GetWhereabouts() Whereabouts {
	now := time.Now()
	a.mu.Lock()
	w := a.whereaboutsLocked(now)
	a.mu.Unlock()
	w.Live = a.devLive(now)
	return w
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

// SetWhereAt é o SetWhere valendo desde fromMs (no passado: "desde a
// última leitura boa" ao tirar um aparelho do cálculo). O trecho aberto é
// fechado nesse instante; não pode voltar para antes de outro trecho.
func (a *App) SetWhereAt(gmc, rc, name string, fromMs int64) (Whereabouts, error) {
	now := time.Now()
	if fromMs <= 0 || fromMs > now.UnixMilli() {
		return a.setWhere(now, gmc, rc, name)
	}
	start := time.UnixMilli(fromMs)
	a.mu.Lock()
	for _, p := range a.periods {
		if p.To != 0 && p.To > start.Truncate(time.Minute).UnixMilli() {
			a.mu.Unlock()
			return a.GetWhereabouts(), errors.New("esse começo cai dentro de outro trecho já marcado")
		}
		// o trecho aberto começou depois: o novo começa logo após o começo dele
		if p.To == 0 && !start.After(time.UnixMilli(p.From)) {
			start = time.UnixMilli(p.From).Add(time.Minute)
		}
	}
	a.mu.Unlock()
	if start.After(now) {
		start = now
	}
	return a.setWhere(start, gmc, rc, name)
}

// LeaveHome é o "Saí de casa" do botão do logo e da busca (2.48): abre um
// trecho fora de casa com quem vai junto. O GMC-800 indo junto é um trecho
// comum; ficando, é "eu não estava em casa" (o fundo da casa segue nele).
// "Voltei" é o SetWhere de volta ao normal.
func (a *App) LeaveHome(gmcGoes, rcGoes bool) (Whereabouts, error) {
	gmc, rc := WhereHome, WhereHome
	if gmcGoes {
		gmc = WhereMe
	}
	if rcGoes {
		rc = WhereMe
	}
	return a.setWhereOut(time.Now(), gmc, rc, "", !gmcGoes)
}

func (a *App) setWhere(now time.Time, gmc, rc, name string) (Whereabouts, error) {
	return a.setWhereOut(now, gmc, rc, name, false)
}

// setWhereOut é o setWhere com "você não estava em casa" (só vale com o
// GMC-800 em casa).
func (a *App) setWhereOut(now time.Time, gmc, rc, name string, notHome bool) (Whereabouts, error) {
	for _, w := range []string{gmc, rc} {
		if w != WhereHome && w != WhereMe && w != WhereOff {
			return a.GetWhereabouts(), errors.New("local inválido")
		}
	}
	notHome = notHome && gmc == WhereHome && rc != WhereOff
	ms := now.Truncate(time.Minute).UnixMilli()
	a.mu.Lock()
	if cur := a.whereaboutsLocked(now); cur.GMC == gmc && cur.RC == rc && cur.NotHome == notHome {
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
	if !(gmc == WhereHome && rc == WhereMe && !notHome) {
		var max int64 // (os locais já foram conferidos acima)
		for _, p := range a.periods {
			max = maxInt64(max, p.ID)
		}
		name = strings.TrimSpace(name)
		if name == "" {
			name = periodName(gmc, rc)
			if notHome && rc == WhereMe {
				name = periodNameOut
			}
		}
		p := Period{ID: max + 1, From: ms, Name: name, NotHome: notHome}
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
	where := map[string]string{WhereHome: "em casa", WhereMe: "com você", WhereOff: "fora do cálculo"}
	a.logEvent("GMC-800 " + where[gmc] + ", RC102 " + where[rc])
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
	a.tombsLoadLocked()
	byUID := map[string]int{}
	for i, p := range a.periods {
		if p.UID != "" {
			byUID[p.UID] = i
		}
	}
	list = append([]Period{}, list...)
	sort.Slice(list, func(i, j int) bool { return list[i].From < list[j].From })
	for _, p := range list {
		if p.To != 0 && p.To <= p.From {
			continue
		}
		if p.UID == "" {
			p.UID = legacyPeriodUID(p)
		}
		if a.tombBlocks(recPeriod, p.UID, p.Edited) {
			continue
		}
		// o mesmo trecho: a edição mais nova vale (N4)
		if i, ok := byUID[p.UID]; ok {
			cur := a.periods[i]
			if p.Edited > cur.Edited || (p.Edited == 0 && cur.To == 0 && p.To != 0 && p.From == cur.From) {
				others := append(append([]Period{}, a.periods[:i]...), a.periods[i+1:]...)
				if p.To != cur.To || p.From != cur.From {
					if crossesPeriod(others, p) {
						continue
					}
				}
				p.ID = cur.ID
				a.periods[i] = p
				n++
			}
			continue
		}
		if have[[2]int64{p.From, p.To}] {
			continue
		}
		if i, ok := open[p.From]; ok && p.To != 0 {
			a.periods[i].To = p.To
			delete(open, p.From)
			n++
			continue
		}
		// cruza com um que já está aqui (o aberto lá e fechado aqui, ou dois
		// abertos): o daqui vale, senão o aparelho ficaria "fora" para sempre
		if crossesPeriod(a.periods, p) {
			continue
		}
		max++
		p.ID = max
		a.periods = append(a.periods, p)
		byUID[p.UID] = len(a.periods) - 1
		have[[2]int64{p.From, p.To}] = true
		n++
	}
	if n > 0 {
		sort.Slice(a.periods, func(i, j int) bool { return a.periods[i].From < a.periods[j].From })
		a.recMerge = true
		_ = a.savePeriodsLocked()
		a.recMerge = false
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

// rolesChecker diz quem tinha cada papel num instante (periodRoles do
// trecho, ou o normal fora deles).
func (a *App) rolesChecker() func(time.Time) (personal, personalFill, home, homeFill string) {
	a.mu.Lock()
	list := append([]Period{}, a.periods...)
	a.mu.Unlock()
	return func(t time.Time) (string, string, string, string) {
		ms := t.UnixMilli()
		for _, p := range list {
			if ms >= p.From && ms < periodEnd(p) {
				return periodRoles(p)
			}
		}
		return pickRoles(WhereHome, WhereMe)
	}
}

// awayCheckerFor diz se um aparelho estava fora do fundo da casa: o GMC
// quando não estava em casa; o Radiacode quando os dois estavam com você ou
// quando ele estava fora do cálculo.
// Com "eu não estava em casa" e o Radiacode com você (2.38.0), ele também
// estava fora (você saiu com ele e o GMC ficou).
func (a *App) awayCheckerFor(dev string) func(time.Time) bool {
	a.mu.Lock()
	list := append([]Period{}, a.periods...)
	a.mu.Unlock()
	return func(t time.Time) bool {
		g, r, out := WhereHome, WhereMe, false
		ms := t.UnixMilli()
		for _, p := range list {
			if ms >= p.From && ms < periodEnd(p) {
				g, r = periodWhere(p)
				out = p.NotHome && r == WhereMe
				break
			}
		}
		if dev == devRC {
			return r == WhereOff || (r == WhereMe && (g == WhereMe || out))
		}
		return g != WhereHome
	}
}

// rcAwayChecker é o "fora de casa" do Radiacode.
func (a *App) rcAwayChecker() func(time.Time) bool { return a.awayCheckerFor(devRC) }

// rcHomeOnly tira os pontos do Radiacode fora de casa e das medições.
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
// dose que o Radiacode soma, se houver). Com trecho, é a soma dos minutos
// gravados, e o fundo de cada minuto é o do aparelho que mediu. Os minutos
// em que ninguém tinha o papel são lacuna: não contam (GapMinutes).
func (a *App) roleDose(from, to time.Time, step time.Duration, limit, role string) (DoseChart, error) {
	if role != "pessoal" {
		role = "casa"
	}
	normal := map[string]string{"pessoal": devRC, "casa": "gmc"}[role]
	if !a.periodsIn(from, to) {
		c, err := a.buildDose(from, to, step, limit, normal)
		c.NoDataMinutes = max(c.HistMinutes-c.MeasuredMinutes, 0)
		if normal == "gmc" {
			c.SatMinutes = a.satMinutes(from, to, nil)
		}
		return c, err
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
	roles := a.rolesChecker()
	pick := func(t time.Time) (main, fill string) {
		p, pf, h, hf := roles(t)
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
	c.NoDataMinutes = max(cs.Hist-c.MeasuredMinutes-c.GapMinutes, 0)
	c.SatMinutes = a.satMinutes(from, to, func(t time.Time) bool {
		k := t.Truncate(time.Minute).UnixMilli()
		return byMin[k].dev == "gmc"
	})
	return c, nil
}

// satMinutes conta os minutos do GMC saturado em [from, to) que entraram na
// dose (use: nil = todos): a dose deles é "pelo menos" (≥).
func (a *App) satMinutes(from, to time.Time, use func(time.Time) bool) int {
	if a.store == nil {
		return 0
	}
	pts, err := a.storeRange(from, to)
	if err != nil {
		return 0
	}
	n := 0
	for _, p := range pts {
		if p.HasObs(store.ObsSaturation) && (use == nil || use(p.T)) {
			n++
		}
	}
	return n
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
		pm, _, h, _ := periodRoles(p)
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

// DevMark é um trecho que o app detectou sozinho num aparelho (não é marcado
// por você): sem dado (desconectado ou desligado), saturando ou sem
// contagens.
type DevMark struct {
	Device string `json:"device"` // gmc, radiacode
	Kind   string `json:"kind"`   // sem dado, saturando, sem contagens
	From   int64  `json:"from"`   // ms
	To     int64  `json:"to"`     // ms
}

// markGap é o buraco mínimo entre dois minutos gravados que vira "sem dado".
const markGap = 3 * time.Minute

// GetDeviceMarks devolve, de cada aparelho, os trechos sem dado, saturando e
// sem contagens em [fromMs, toMs), para a linha do tempo de Fora de casa.
func (a *App) GetDeviceMarks(fromMs, toMs int64) ([]DevMark, error) {
	from, to := time.UnixMilli(fromMs), time.UnixMilli(toMs)
	if now := time.Now(); to.After(now) {
		to = now
	}
	out := []DevMark{}
	add := func(dev, kind string, f, t time.Time) {
		if n := len(out); n > 0 && out[n-1].Device == dev && out[n-1].Kind == kind && out[n-1].To >= f.UnixMilli() {
			out[n-1].To = t.UnixMilli()
			return
		}
		out = append(out, DevMark{dev, kind, f.UnixMilli(), t.UnixMilli()})
	}
	gaps := func(dev string, ts []time.Time) {
		for i := 1; i < len(ts); i++ {
			if ts[i].Sub(ts[i-1]) > markGap {
				add(dev, "sem dado", ts[i-1].Add(time.Minute), ts[i])
			}
		}
		if n := len(ts); n > 0 && to.Sub(ts[n-1]) > markGap {
			add(dev, "sem dado", ts[n-1].Add(time.Minute), to)
		}
	}
	if a.store != nil {
		pts, err := a.store.Range(from, to)
		if err != nil {
			return out, err
		}
		ts := make([]time.Time, len(pts))
		for i, p := range pts {
			ts[i] = p.T
		}
		gaps("gmc", ts)
		for _, p := range pts {
			switch {
			case p.HasObs(store.ObsSaturation):
				add("gmc", "saturando", p.T, p.T.Add(time.Minute))
			case p.CPM == 0:
				add("gmc", "sem contagens", p.T, p.T.Add(time.Minute))
			}
		}
	}
	rc := a.rcRange(from, to)
	ts := make([]time.Time, len(rc))
	for i, p := range rc {
		ts[i] = p.T
	}
	gaps(devRC, ts)
	sort.SliceStable(out, func(i, j int) bool { return out[i].From < out[j].From })
	// um minuto isolado em 0 CPM acontece; sem contagens é de 2 min para cima
	kept := out[:0]
	for _, m := range out {
		if m.Kind != "sem contagens" || m.To-m.From >= 2*60_000 {
			kept = append(kept, m)
		}
	}
	return kept, nil
}

// crossesPeriod diz se p cruza com algum dos trechos (o aberto vai até agora
// e além).
func crossesPeriod(ps []Period, p Period) bool {
	for _, q := range ps {
		if p.From < periodEnd(q) && q.From < periodEnd(p) {
			return true
		}
	}
	return false
}
