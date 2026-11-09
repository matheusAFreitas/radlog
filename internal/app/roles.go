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

// ---------- papel dos aparelhos ----------
//
// O normal é o GMC-800 parado em casa (monitor da casa) e o RadiaCode andando
// com você (dosímetro pessoal). Às vezes os papéis trocam: o GMC sai com você
// e o RadiaCode fica em casa. Cada troca é um trecho guardado em papeis.json
// (To = 0: trocado até agora), e o papel de cada minuto sai dela:
//   - trocado: o GMC está fora de casa (fica fora do fundo da casa, como um
//     período "fora de casa") e o RadiaCode está em casa (entra no fundo dele
//     mesmo dentro de um período "fora de casa", que nessa hora fala de você);
//   - normal: nada muda em relação ao que já era.
// A Minha dose junta os dois: a pessoal é a do aparelho que estava com você em
// cada minuto, e a da casa a do que estava em casa.

// RoleSwap é um trecho com os papéis trocados.
type RoleSwap struct {
	ID   int64  `json:"id"`
	From int64  `json:"from"` // ms
	To   int64  `json:"to"`   // ms; 0 = ainda trocado
	Name string `json:"name"`
}

// Roles diz o papel de agora e os trechos trocados.
type Roles struct {
	Swapped  bool       `json:"swapped"`  // trocado agora
	Personal string     `json:"personal"` // aparelho com você agora (radiacode, gmc)
	Home     string     `json:"home"`     // aparelho em casa agora
	Swaps    []RoleSwap `json:"swaps"`    // do mais antigo ao mais novo
}

const rolesFile = "papeis.json"

func (a *App) rolesPath() string { return filepath.Join(a.dataDir, rolesFile) }

func (a *App) loadRoles() {
	b, err := os.ReadFile(a.rolesPath())
	if err != nil {
		return
	}
	var list []RoleSwap
	if json.Unmarshal(b, &list) == nil {
		sort.Slice(list, func(i, j int) bool { return list[i].From < list[j].From })
		a.swaps = list
	}
}

func (a *App) saveRolesLocked() error {
	if a.dataDir == "" {
		return nil
	}
	b, _ := json.MarshalIndent(a.swaps, "", "  ")
	tmp := a.rolesPath() + ".tmp"
	if err := os.WriteFile(tmp, b, 0o644); err != nil {
		return err
	}
	return os.Rename(tmp, a.rolesPath())
}

// swapEnd é o fim de um trecho (o aberto vai até depois de qualquer instante).
func swapEnd(s RoleSwap) int64 {
	if s.To == 0 {
		return 1<<62 - 1
	}
	return s.To
}

func (a *App) rolesLocked(now time.Time) Roles {
	r := Roles{Personal: devRC, Home: "gmc", Swaps: append([]RoleSwap{}, a.swaps...)}
	ms := now.UnixMilli()
	for _, s := range a.swaps {
		if ms >= s.From && ms < swapEnd(s) {
			r.Swapped, r.Personal, r.Home = true, "gmc", devRC
		}
	}
	return r
}

// GetRoles devolve o papel de agora e os trechos trocados.
func (a *App) GetRoles() Roles {
	a.mu.Lock()
	defer a.mu.Unlock()
	return a.rolesLocked(time.Now())
}

func (a *App) rolesChangedLocked() Roles {
	a.bgAt = time.Time{} // o fundo da casa muda: recalcula
	return a.rolesLocked(time.Now())
}

func (a *App) emitRoles(r Roles) { a.emit("roles", r) }

// SwapRoles troca os papéis a partir de agora (swap = true: o GMC vai com
// você) ou desfaz a troca aberta (swap = false). name é opcional.
func (a *App) SwapRoles(swap bool, name string) (Roles, error) {
	return a.swapRoles(time.Now(), swap, name)
}

func (a *App) swapRoles(now time.Time, swap bool, name string) (Roles, error) {
	ms := now.Truncate(time.Minute).UnixMilli()
	a.mu.Lock()
	open := -1
	for i, s := range a.swaps {
		if s.To == 0 {
			open = i
		}
	}
	switch {
	case swap && open >= 0:
		r := a.rolesLocked(now)
		a.mu.Unlock()
		return r, nil // já trocado
	case swap:
		var max int64
		for _, s := range a.swaps {
			max = maxInt64(max, s.ID)
		}
		name = strings.TrimSpace(name)
		if name == "" {
			name = "Papéis trocados"
		}
		a.swaps = append(a.swaps, RoleSwap{ID: max + 1, From: ms, Name: name})
	case open >= 0:
		if ms <= a.swaps[open].From { // desfez no mesmo minuto: não sobra trecho
			a.swaps = append(a.swaps[:open], a.swaps[open+1:]...)
		} else {
			a.swaps[open].To = ms
		}
	default:
		r := a.rolesLocked(now)
		a.mu.Unlock()
		return r, nil // já normal
	}
	err := a.saveRolesLocked()
	r := a.rolesChangedLocked()
	a.mu.Unlock()
	if swap {
		a.logEvent("Papéis trocados: o GMC-800 vai com você e o RadiaCode fica em casa")
	} else {
		a.logEvent("Papéis de volta ao normal: GMC-800 em casa e RadiaCode com você")
	}
	a.emitRoles(r)
	return r, err
}

// SaveRoleSwap cria (ID 0) ou altera um trecho trocado já passado (para
// marcar uma troca que você esqueceu de registrar na hora).
func (a *App) SaveRoleSwap(s RoleSwap) (Roles, error) {
	if s.To != 0 && s.To < s.From {
		s.From, s.To = s.To, s.From
	}
	if s.To != 0 && s.To-s.From < 60_000 {
		return a.GetRoles(), errors.New("o trecho precisa ter pelo menos 1 minuto")
	}
	s.Name = strings.TrimSpace(s.Name)
	if s.Name == "" {
		s.Name = "Papéis trocados"
	}
	a.mu.Lock()
	for _, q := range a.swaps {
		if q.ID != s.ID && s.From < swapEnd(q) && q.From < swapEnd(s) {
			r := a.rolesLocked(time.Now())
			a.mu.Unlock()
			return r, errors.New("esse trecho cruza com outro em que os papéis já estavam trocados")
		}
	}
	if s.ID == 0 {
		var max int64
		for _, q := range a.swaps {
			max = maxInt64(max, q.ID)
		}
		s.ID = max + 1
		a.swaps = append(a.swaps, s)
	} else {
		found := false
		for i := range a.swaps {
			if a.swaps[i].ID == s.ID {
				a.swaps[i], found = s, true
			}
		}
		if !found {
			r := a.rolesLocked(time.Now())
			a.mu.Unlock()
			return r, errors.New("trecho não encontrado")
		}
	}
	sort.Slice(a.swaps, func(i, j int) bool { return a.swaps[i].From < a.swaps[j].From })
	err := a.saveRolesLocked()
	r := a.rolesChangedLocked()
	a.mu.Unlock()
	a.emitRoles(r)
	return r, err
}

// DeleteRoleSwap apaga um trecho trocado.
func (a *App) DeleteRoleSwap(id int64) (Roles, error) {
	a.mu.Lock()
	kept := a.swaps[:0]
	for _, q := range a.swaps {
		if q.ID != id {
			kept = append(kept, q)
		}
	}
	a.swaps = kept
	err := a.saveRolesLocked()
	r := a.rolesChangedLocked()
	a.mu.Unlock()
	a.emitRoles(r)
	return r, err
}

// mergeRoleSwaps junta trechos vindos de um backup ou de outro PC. A chave é
// o começo: um trecho aberto aqui que chegou fechado de lá é fechado.
func (a *App) mergeRoleSwaps(list []RoleSwap) int {
	a.mu.Lock()
	defer a.mu.Unlock()
	n := 0
	var max int64
	at := map[int64]int{}
	for i, s := range a.swaps {
		at[s.From] = i
		max = maxInt64(max, s.ID)
	}
	for _, s := range list {
		if s.To != 0 && s.To <= s.From {
			continue
		}
		if i, ok := at[s.From]; ok {
			if a.swaps[i].To == 0 && s.To != 0 {
				a.swaps[i].To = s.To
				n++
			}
			continue
		}
		max++
		s.ID = max
		a.swaps = append(a.swaps, s)
		at[s.From] = len(a.swaps) - 1
		n++
	}
	if n > 0 {
		sort.Slice(a.swaps, func(i, j int) bool { return a.swaps[i].From < a.swaps[j].From })
		_ = a.saveRolesLocked()
		a.bgAt = time.Time{}
	}
	return n
}

// swappedChecker diz se um instante caiu num trecho trocado (cópia dos
// trechos, para usar sem segurar o lock).
func (a *App) swappedChecker() func(time.Time) bool {
	a.mu.Lock()
	list := append([]RoleSwap{}, a.swaps...)
	a.mu.Unlock()
	if len(list) == 0 {
		return func(time.Time) bool { return false }
	}
	return func(t time.Time) bool {
		ms := t.UnixMilli()
		for _, s := range list {
			if ms >= s.From && ms < swapEnd(s) {
				return true
			}
		}
		return false
	}
}

// swapsIn diz se algum trecho trocado cruza [from, to).
func (a *App) swapsIn(from, to time.Time) bool {
	a.mu.Lock()
	defer a.mu.Unlock()
	f, t := from.UnixMilli(), to.UnixMilli()
	for _, s := range a.swaps {
		if s.From < t && f < swapEnd(s) {
			return true
		}
	}
	return false
}

// rcAwayChecker é o "fora de casa" do RadiaCode: nos trechos trocados ele
// está em casa; fora deles, valem os períodos marcados (como sempre foi).
func (a *App) rcAwayChecker() func(time.Time) bool {
	away, swapped := a.periodChecker(), a.swappedChecker()
	return func(t time.Time) bool { return away(t) && !swapped(t) }
}

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
// cada minuto, a do aparelho que tinha esse papel. Sem troca no período, é
// a dose do próprio aparelho (com a dose que o RadiaCode soma, se houver).
// Com troca, é a soma dos minutos gravados de cada um, e o fundo de cada
// minuto é o do aparelho que mediu.
func (a *App) roleDose(from, to time.Time, step time.Duration, limit, role string) (DoseChart, error) {
	normal := map[string]string{"pessoal": devRC, "casa": "gmc"}[role]
	if normal == "" {
		normal = "gmc"
	}
	if !a.swapsIn(from, to) {
		return a.buildDose(from, to, step, limit, normal)
	}
	swapped := map[string]string{devRC: "gmc", "gmc": devRC}[normal]
	lim, ok := doseLimits[limit]
	if !ok {
		lim = doseLimits["publico"]
	}
	np, nBg, _, err := a.doseSeries(normal, from, to)
	if err != nil {
		return DoseChart{}, err
	}
	sp, sBg, _, err := a.doseSeries(swapped, from, to)
	if err != nil {
		return DoseChart{}, err
	}
	isSwapped := a.swappedChecker()
	type rm struct {
		T        time.Time
		USv, Bg  float64 // µSv/h
		fromSwap bool
	}
	var mins []rm
	for _, p := range np {
		if !isSwapped(p.T) {
			mins = append(mins, rm{p.T, p.USv, nBg, false})
		}
	}
	for _, p := range sp {
		if isSwapped(p.T) {
			mins = append(mins, rm{p.T, p.USv, sBg, true})
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
			if mins[i].fromSwap {
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
