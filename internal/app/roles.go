package app

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"radlog/internal/store"
)

// ---------- onde está cada aparelho ----------
//
// Cada aparelho está "em casa" (parado, medindo a casa) ou "comigo" (anda com
// você). O normal é o GMC-800 em casa e o RadiaCode com você, mas dá para
// mudar cada um: levar o GMC junto, deixar o RadiaCode em casa, levar os dois
// ou deixar os dois em casa (lado a lado, como na intercalibração). Cada
// mudança é um trecho guardado em aparelhos.json (To = 0: até agora); fora
// dos trechos vale o normal.
//
// O que muda com o local de cada minuto:
//   - fundo da casa: um aparelho "em casa" sempre conta, mesmo dentro de um
//     período "fora de casa" (que fala de você); sem trecho marcado, os
//     períodos tiram os dois, como sempre foi;
//   - Minha dose: a pessoal é a do aparelho com você (o RadiaCode, se os dois
//     estão com você) e a da casa a do aparelho em casa (o GMC, se os dois
//     estão em casa). Sem nenhum aparelho num papel, aquele minuto não conta.

// Locais.
const (
	WhereHome = "casa"
	WhereMe   = "comigo"
)

// Stay é um trecho em que um aparelho ficou num local diferente do normal.
type Stay struct {
	ID     int64  `json:"id"`
	Device string `json:"device"` // gmc, radiacode
	Where  string `json:"where"`  // casa, comigo
	From   int64  `json:"from"`   // ms
	To     int64  `json:"to"`     // ms; 0 = até agora
	Name   string `json:"name"`
}

// Whereabouts diz onde está cada aparelho agora e os trechos marcados.
type Whereabouts struct {
	GMC      string `json:"gmc"`       // casa, comigo
	RC       string `json:"radiacode"` // casa, comigo
	Personal string `json:"personal"`  // aparelho que dá a dose pessoal agora ("" = nenhum com você)
	Home     string `json:"home"`      // aparelho que dá a dose da casa agora ("" = nenhum em casa)
	Normal   bool   `json:"normal"`    // GMC em casa e RadiaCode com você
	Stays    []Stay `json:"stays"`     // do mais antigo ao mais novo
}

const staysFile = "aparelhos.json"

// whereDefault é o local normal de cada aparelho.
func whereDefault(dev string) string {
	if dev == devRC {
		return WhereMe
	}
	return WhereHome
}

func validStay(s Stay) bool {
	return (s.Device == "gmc" || s.Device == devRC) && (s.Where == WhereHome || s.Where == WhereMe)
}

func (a *App) staysPath() string { return filepath.Join(a.dataDir, staysFile) }

func (a *App) loadStays() {
	b, err := os.ReadFile(a.staysPath())
	if err != nil {
		return
	}
	var list []Stay
	if json.Unmarshal(b, &list) == nil {
		kept := list[:0]
		for _, s := range list {
			if validStay(s) {
				kept = append(kept, s)
			}
		}
		sort.Slice(kept, func(i, j int) bool { return kept[i].From < kept[j].From })
		a.stays = kept
	}
}

func (a *App) saveStaysLocked() error {
	if a.dataDir == "" {
		return nil
	}
	b, _ := json.MarshalIndent(a.stays, "", "  ")
	tmp := a.staysPath() + ".tmp"
	if err := os.WriteFile(tmp, b, 0o644); err != nil {
		return err
	}
	return os.Rename(tmp, a.staysPath())
}

// stayEnd é o fim de um trecho (o aberto vai até depois de qualquer instante).
func stayEnd(s Stay) int64 {
	if s.To == 0 {
		return 1<<62 - 1
	}
	return s.To
}

// pickRoles diz qual aparelho dá a dose pessoal e qual a da casa.
func pickRoles(gmc, rc string) (personal, home string) {
	switch {
	case rc == WhereMe:
		personal = devRC
	case gmc == WhereMe:
		personal = "gmc"
	}
	switch {
	case gmc == WhereHome:
		home = "gmc"
	case rc == WhereHome:
		home = devRC
	}
	return
}

func (a *App) whereaboutsLocked(now time.Time) Whereabouts {
	ms := now.UnixMilli()
	w := Whereabouts{GMC: whereDefault("gmc"), RC: whereDefault(devRC), Stays: append([]Stay{}, a.stays...)}
	for _, s := range a.stays {
		if ms >= s.From && ms < stayEnd(s) {
			if s.Device == devRC {
				w.RC = s.Where
			} else {
				w.GMC = s.Where
			}
		}
	}
	w.Personal, w.Home = pickRoles(w.GMC, w.RC)
	w.Normal = w.GMC == WhereHome && w.RC == WhereMe
	return w
}

// GetWhereabouts diz onde está cada aparelho agora e os trechos marcados.
func (a *App) GetWhereabouts() Whereabouts {
	a.mu.Lock()
	defer a.mu.Unlock()
	return a.whereaboutsLocked(time.Now())
}

func (a *App) staysChanged() Whereabouts {
	a.mu.Lock()
	a.bgAt = time.Time{} // o fundo da casa muda: recalcula
	w := a.whereaboutsLocked(time.Now())
	a.mu.Unlock()
	a.emit("whereabouts", w)
	return w
}

// SetWhere muda o local de um aparelho a partir de agora (device: gmc ou
// radiacode; where: casa ou comigo). Voltar ao normal fecha o trecho aberto.
func (a *App) SetWhere(device, where string) (Whereabouts, error) {
	return a.setWhere(time.Now(), device, where)
}

func (a *App) setWhere(now time.Time, device, where string) (Whereabouts, error) {
	if !validStay(Stay{Device: device, Where: where}) {
		return a.GetWhereabouts(), errors.New("aparelho ou local inválido")
	}
	ms := now.Truncate(time.Minute).UnixMilli()
	a.mu.Lock()
	if cur := a.whereaboutsLocked(now); (device == devRC && cur.RC == where) || (device == "gmc" && cur.GMC == where) {
		a.mu.Unlock()
		return a.GetWhereabouts(), nil // já está lá
	}
	// fecha o trecho aberto desse aparelho
	for i := 0; i < len(a.stays); i++ {
		s := &a.stays[i]
		if s.Device != device || s.To != 0 {
			continue
		}
		if ms <= s.From { // mudou de novo no mesmo minuto: não sobra trecho
			a.stays = append(a.stays[:i], a.stays[i+1:]...)
			i--
		} else {
			s.To = ms
		}
	}
	if where != whereDefault(device) {
		var max int64
		for _, s := range a.stays {
			max = maxInt64(max, s.ID)
		}
		a.stays = append(a.stays, Stay{ID: max + 1, Device: device, Where: where, From: ms})
		sort.Slice(a.stays, func(i, j int) bool { return a.stays[i].From < a.stays[j].From })
	}
	err := a.saveStaysLocked()
	a.mu.Unlock()
	name := map[string]string{"gmc": "GMC-800", devRC: "RadiaCode"}[device]
	if where == WhereHome {
		a.logEvent(name + " em casa")
	} else {
		a.logEvent(name + " com você")
	}
	return a.staysChanged(), err
}

// SaveStay cria (ID 0) ou altera um trecho já passado (para marcar uma
// mudança que você esqueceu de registrar na hora).
func (a *App) SaveStay(s Stay) (Whereabouts, error) {
	if !validStay(s) {
		return a.GetWhereabouts(), errors.New("aparelho ou local inválido")
	}
	if s.To != 0 && s.To < s.From {
		s.From, s.To = s.To, s.From
	}
	if s.To != 0 && s.To-s.From < 60_000 {
		return a.GetWhereabouts(), errors.New("o trecho precisa ter pelo menos 1 minuto")
	}
	s.Name = strings.TrimSpace(s.Name)
	a.mu.Lock()
	for _, q := range a.stays {
		if q.ID != s.ID && q.Device == s.Device && s.From < stayEnd(q) && q.From < stayEnd(s) {
			a.mu.Unlock()
			return a.GetWhereabouts(), errors.New("esse trecho cruza com outro do mesmo aparelho")
		}
	}
	if s.ID == 0 {
		var max int64
		for _, q := range a.stays {
			max = maxInt64(max, q.ID)
		}
		s.ID = max + 1
		a.stays = append(a.stays, s)
	} else {
		found := false
		for i := range a.stays {
			if a.stays[i].ID == s.ID {
				a.stays[i], found = s, true
			}
		}
		if !found {
			a.mu.Unlock()
			return a.GetWhereabouts(), errors.New("trecho não encontrado")
		}
	}
	sort.Slice(a.stays, func(i, j int) bool { return a.stays[i].From < a.stays[j].From })
	err := a.saveStaysLocked()
	a.mu.Unlock()
	return a.staysChanged(), err
}

// DeleteStay apaga um trecho.
func (a *App) DeleteStay(id int64) (Whereabouts, error) {
	a.mu.Lock()
	kept := a.stays[:0]
	for _, q := range a.stays {
		if q.ID != id {
			kept = append(kept, q)
		}
	}
	a.stays = kept
	err := a.saveStaysLocked()
	a.mu.Unlock()
	return a.staysChanged(), err
}

// mergeStays junta trechos vindos de um backup ou de outro PC. A chave é o
// aparelho e o começo: um trecho aberto aqui que chegou fechado é fechado.
func (a *App) mergeStays(list []Stay) int {
	a.mu.Lock()
	defer a.mu.Unlock()
	key := func(s Stay) string { return s.Device + "@" + time.UnixMilli(s.From).UTC().Format(time.RFC3339) }
	n := 0
	var max int64
	at := map[string]int{}
	for i, s := range a.stays {
		at[key(s)] = i
		max = maxInt64(max, s.ID)
	}
	for _, s := range list {
		if !validStay(s) || (s.To != 0 && s.To <= s.From) {
			continue
		}
		if i, ok := at[key(s)]; ok {
			if a.stays[i].To == 0 && s.To != 0 {
				a.stays[i].To = s.To
				n++
			}
			continue
		}
		max++
		s.ID = max
		a.stays = append(a.stays, s)
		at[key(s)] = len(a.stays) - 1
		n++
	}
	if n > 0 {
		sort.Slice(a.stays, func(i, j int) bool { return a.stays[i].From < a.stays[j].From })
		_ = a.saveStaysLocked()
		a.bgAt = time.Time{}
	}
	return n
}

// whereChecker diz onde um aparelho estava num instante e se foi marcado
// (cópia dos trechos, para usar sem segurar o lock).
func (a *App) whereChecker(dev string) func(time.Time) (where string, marked bool) {
	a.mu.Lock()
	var list []Stay
	for _, s := range a.stays {
		if s.Device == dev {
			list = append(list, s)
		}
	}
	a.mu.Unlock()
	def := whereDefault(dev)
	return func(t time.Time) (string, bool) {
		ms := t.UnixMilli()
		for _, s := range list {
			if ms >= s.From && ms < stayEnd(s) {
				return s.Where, true
			}
		}
		return def, false
	}
}

// staysIn diz se algum trecho cruza [from, to).
func (a *App) staysIn(from, to time.Time) bool {
	a.mu.Lock()
	defer a.mu.Unlock()
	f, t := from.UnixMilli(), to.UnixMilli()
	for _, s := range a.stays {
		if s.From < t && f < stayEnd(s) {
			return true
		}
	}
	return false
}

// awayCheckerFor diz se um aparelho estava fora de casa: num período "fora de
// casa", a não ser que ele tenha sido marcado em casa nessa hora.
func (a *App) awayCheckerFor(dev string) func(time.Time) bool {
	period, where := a.periodChecker(), a.whereChecker(dev)
	return func(t time.Time) bool {
		if !period(t) {
			return false
		}
		w, marked := where(t)
		return !(marked && w == WhereHome)
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

// roleDose é a dose de um papel ("pessoal" ou "casa") em [from, to): em
// cada minuto, a do aparelho que tinha esse papel. Sem trecho marcado no
// período, é a dose do próprio aparelho (com a dose que o RadiaCode soma, se
// houver). Com trecho, é a soma dos minutos gravados de quem tinha o papel,
// e o fundo de cada minuto é o do aparelho que mediu.
func (a *App) roleDose(from, to time.Time, step time.Duration, limit, role string) (DoseChart, error) {
	if role != "pessoal" {
		role = "casa"
	}
	if !a.staysIn(from, to) {
		return a.buildDose(from, to, step, limit, map[string]string{"pessoal": devRC, "casa": "gmc"}[role])
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
	whereG, whereR := a.whereChecker("gmc"), a.whereChecker(devRC)
	pick := func(t time.Time) string {
		g, _ := whereG(t)
		r, _ := whereR(t)
		p, h := pickRoles(g, r)
		if role == "pessoal" {
			return p
		}
		return h
	}
	normal := map[string]string{"pessoal": devRC, "casa": "gmc"}[role]
	type rm struct {
		T       time.Time
		USv, Bg float64 // µSv/h
		other   bool    // medido pelo aparelho que normalmente não tem esse papel
	}
	var mins []rm
	for _, p := range rcPts {
		if pick(p.T) == devRC {
			mins = append(mins, rm{p.T, p.USv, rcBg, normal != devRC})
		}
	}
	for _, p := range gPts {
		if pick(p.T) == "gmc" {
			mins = append(mins, rm{p.T, p.USv, gBg, normal != "gmc"})
		}
	}
	sort.Slice(mins, func(i, j int) bool { return mins[i].T.Before(mins[j].T) })

	c := DoseChart{
		From: from.UnixMilli(), To: to.UnixMilli(), StepMs: step.Milliseconds(),
		LimitName: lim.Name, LimitMSvYear: lim.MSvYear,
		Device: "misto", DoseSource: "app", Mixed: true,
		BackgroundSource: "fundo de casa de cada aparelho, nos minutos em que ele mediu",
	}
	limPerMin := lim.MSvYear * 1000 / minutesPerYear
	n := int(to.Sub(from) / step)
	i := 0
	var dose, bg, allow float64
	for k := 1; k <= n; k++ {
		end := from.Add(time.Duration(k) * step)
		for i < len(mins) && mins[i].T.Before(end) {
			dose += mins[i].USv / 60
			bg += mins[i].Bg / 60
			allow += limPerMin
			c.MeasuredMinutes++
			if mins[i].other {
				c.SwapMinutes++
			}
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
	if c.MeasuredMinutes > 0 {
		c.BackgroundRate = bg / float64(c.MeasuredMinutes) * 60 // média dos minutos medidos
	}
	first := a.gmcFirst()
	if f := a.rcFirst(); !f.IsZero() && (first.IsZero() || f.Before(first)) {
		first = f
	}
	if len(mins) > 0 && (first.IsZero() || mins[0].T.Before(first)) {
		first = mins[0].T
	}
	cs := coverageSpan(from, to, first, time.Now())
	c.Coverage, c.HistMinutes, c.SpanMinutes = ratio(c.MeasuredMinutes, cs.Hist), cs.Hist, cs.Span
	if cs.Pre > 0 && cs.Hist > 0 {
		c.HistFrom = cs.HistFrom.UnixMilli()
	}
	return c, nil
}
