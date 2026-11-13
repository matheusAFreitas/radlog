package app

import (
	"encoding/json"
	"errors"
	"fmt"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"radlog/internal/core/intercal"
	"radlog/internal/store"
)

// ---------- saída programada (2.75) ----------
//
// Spec ux/saida-programada.html (ACE, 13/11). Você marca antes que vai sair
// (para onde, por volta de que horas, quais aparelhos vão) e pode desligar o
// PC. Quando o aparelho que foi conecta de novo e a memória chega, o app
// acha o horário de verdade e pergunta "Você saiu mesmo?" no lugar do "onde
// estava o RC102?". Só com o sim vira trecho de Fora de casa.
//
// A saída é achada, nesta ordem: o aparelho desconectou do PC na janela de
// 1 h do programado (PC ligado); a razão RC102 ÷ GMC-800 saiu do normal na
// janela (só quando um dos dois ficou em casa); o horário programado, com
// "horário estimado". A volta é quando o aparelho que foi conectou de novo.

const (
	plannedFile   = "saidas-programadas.json"
	plannedWindow = time.Hour          // a saída de verdade fica a até 1 h do programado
	plannedForget = 7 * 24 * time.Hour // programada vencida sem dado some sozinha
	plannedRunMin = 15                 // minutos de cada janela da razão
	askPlanned    = "programada"       // tipo da pergunta em fora-perguntas.json
)

// PlannedTrip é uma saída programada.
type PlannedTrip struct {
	ID   int64  `json:"id"`
	Name string `json:"nome"`
	From int64  `json:"de"`  // ms: "saio por volta de"
	GMC  bool   `json:"gmc"` // o GMC-800 vai junto
	RC   bool   `json:"rc"`  // o RC102 vai junto
	// o aparelho desconectou do PC dentro da janela (PC ligado): a saída
	// começou ali
	Left    int64  `json:"saiu,omitempty"`
	LeftDev string `json:"saiuAparelho,omitempty"`
}

func (a *App) plannedPath() string { return filepath.Join(a.dataDir, plannedFile) }

func (a *App) loadPlannedLocked() []PlannedTrip {
	if a.plannedLoaded || a.dataDir == "" {
		return a.planned
	}
	a.plannedLoaded = true
	var list []PlannedTrip
	if a.readStateJSON(a.plannedPath(), "saídas programadas", &list) {
		a.planned = list
	}
	return a.planned
}

func (a *App) savePlannedLocked() {
	sort.Slice(a.planned, func(i, j int) bool { return a.planned[i].From < a.planned[j].From })
	if a.dataDir == "" {
		return
	}
	b, _ := json.MarshalIndent(a.planned, "", "  ")
	if err := writeFileAtomic(a.plannedPath(), b, 0o644); err != nil {
		a.logf("saídas programadas: %v", err)
	}
}

func (a *App) emitPlanned() { a.emit("planned", a.GetPlannedTrips()) }

// GetPlannedTrips devolve as programadas (tira as vencidas há mais de 7 dias).
func (a *App) GetPlannedTrips() []PlannedTrip {
	now := time.Now()
	a.mu.Lock()
	list := a.loadPlannedLocked()
	kept := list[:0:0]
	var gone []PlannedTrip
	for _, p := range list {
		if now.Sub(time.UnixMilli(p.From)) > plannedForget {
			gone = append(gone, p)
			continue
		}
		kept = append(kept, p)
	}
	if len(gone) > 0 {
		a.planned = kept
		a.savePlannedLocked()
	}
	out := append([]PlannedTrip{}, kept...)
	a.mu.Unlock()
	for _, p := range gone {
		a.logEvent(fmt.Sprintf("Saída programada \"%s\" de %s apagada: passou uma semana sem o aparelho voltar", p.Name, time.UnixMilli(p.From).Format("02/01 15:04")))
	}
	return out
}

// SavePlannedTrip cria (ID 0) ou muda uma programada.
func (a *App) SavePlannedTrip(p PlannedTrip) (PlannedTrip, error) {
	p.Name = strings.TrimSpace(p.Name)
	if len([]rune(p.Name)) > 60 {
		p.Name = string([]rune(p.Name)[:60])
	}
	if p.Name == "" {
		p.Name = "Saída programada"
	}
	if !p.GMC && !p.RC {
		return p, errors.New("escolha pelo menos um aparelho que vai com você")
	}
	now := time.Now()
	if time.UnixMilli(p.From).Before(now.Add(-plannedWindow)) {
		return p, errors.New("esse horário já passou; para uma saída que já aconteceu, use \"Adicionar trecho passado\"")
	}
	a.mu.Lock()
	list := a.loadPlannedLocked()
	idx := -1
	var max int64
	for i, q := range list {
		if q.ID == p.ID && p.ID != 0 {
			idx = i
		}
		max = maxInt64(max, q.ID)
	}
	if idx >= 0 {
		old := list[idx]
		if old.From == p.From && old.GMC == p.GMC && old.RC == p.RC {
			p.Left, p.LeftDev = old.Left, old.LeftDev
		} else {
			p.Left, p.LeftDev = 0, ""
		}
		a.planned[idx] = p
	} else {
		if p.ID == 0 {
			p.ID = maxInt64(now.UnixMilli(), max+1)
		}
		p.Left, p.LeftDev = 0, ""
		a.planned = append(list, p)
	}
	a.savePlannedLocked()
	a.mu.Unlock()
	a.logEvent(fmt.Sprintf("Saída programada: %s, por volta de %s (%s)", p.Name, time.UnixMilli(p.From).Format("02/01 15:04"), plannedWho(p.GMC, p.RC)))
	a.emitPlanned()
	return p, nil
}

// DeletePlannedTrip cancela uma programada.
func (a *App) DeletePlannedTrip(id int64) error {
	a.mu.Lock()
	list := a.loadPlannedLocked()
	kept := list[:0:0]
	for _, q := range list {
		if q.ID != id {
			kept = append(kept, q)
		}
	}
	if len(kept) == len(list) {
		a.mu.Unlock()
		return errors.New("essa saída programada não existe mais")
	}
	a.planned = kept
	a.savePlannedLocked()
	a.mu.Unlock()
	a.emitPlanned()
	return nil
}

// NotLeaving é o "Não estou saindo": esquece a desconexão vista na janela.
func (a *App) NotLeaving(id int64) error {
	a.mu.Lock()
	list := a.loadPlannedLocked()
	ok := false
	for i := range list {
		if list[i].ID == id {
			list[i].Left, list[i].LeftDev, ok = 0, "", true
		}
	}
	if ok {
		a.savePlannedLocked()
	}
	a.mu.Unlock()
	if !ok {
		return errors.New("essa saída programada não existe mais")
	}
	a.emitPlanned()
	return nil
}

func plannedWho(gmc, rc bool) string {
	switch {
	case gmc && rc:
		return "vão o RC102 e o GMC-800"
	case gmc:
		return "vai o GMC-800, o RC102 fica"
	}
	return "vai o RC102, o GMC-800 fica"
}

func plannedGoes(p PlannedTrip, dev string) bool {
	if dev == devRC {
		return p.RC
	}
	return p.GMC
}

func devLabel(dev string) string {
	if dev == devRC {
		return "RC102"
	}
	return "GMC-800"
}

// plannedDisconnected anota a desconexão de um aparelho que vai numa
// programada, se ela cai na janela.
func (a *App) plannedDisconnected(dev string, t time.Time) {
	if a.isDemo() {
		return
	}
	var hit *PlannedTrip
	a.mu.Lock()
	list := a.loadPlannedLocked()
	for i := range list {
		p := &list[i]
		d := t.Sub(time.UnixMilli(p.From))
		if p.Left == 0 && plannedGoes(*p, dev) && d >= -plannedWindow && d <= plannedWindow {
			p.Left, p.LeftDev = t.UnixMilli(), dev
			c := *p
			hit = &c
			break
		}
	}
	if hit != nil {
		a.savePlannedLocked()
	}
	a.mu.Unlock()
	if hit != nil {
		a.logEvent(fmt.Sprintf("Começou a saída programada \"%s\": o %s desconectou às %s", hit.Name, devLabel(dev), t.Format("15:04")))
		a.emitPlanned()
	}
}

// plannedOnConnect: o aparelho dev voltou a conectar em conn (e a memória
// dele já chegou). Se ele foi numa programada, troca a pergunta normal pela
// "Você saiu mesmo?" e devolve true.
func (a *App) plannedOnConnect(dev string, conn time.Time) bool {
	if a.isDemo() {
		return false
	}
	a.mu.Lock()
	minLen := time.Duration(normAwayAsk(a.settings.AwayAskMin)) * time.Minute
	var trip PlannedTrip
	found, blip := false, false
	for i, p := range a.loadPlannedLocked() {
		if !plannedGoes(p, dev) {
			continue
		}
		plan := time.UnixMilli(p.From)
		if p.Left > 0 {
			left := time.UnixMilli(p.Left)
			if !left.Before(conn) {
				continue
			}
			if conn.Sub(left) < minLen {
				// caiu e voltou logo (o Bluetooth faz isso): não era a saída
				a.planned[i].Left, a.planned[i].LeftDev, blip = 0, "", true
				continue
			}
		} else if conn.Before(plan.Add(plannedWindow)) {
			// ainda dentro da janela (ligou o PC antes de sair): espera a
			// desconexão ou passar a janela
			continue
		}
		if conn.Sub(plan) > rcAwayMaxGap {
			continue
		}
		trip, found = p, true
		break
	}
	if blip {
		a.savePlannedLocked()
	}
	a.mu.Unlock()
	if !found {
		return false
	}
	ask := a.plannedAsk(trip, dev, conn)
	a.mu.Lock()
	// o que você marcou à mão ganha: a programada some
	manual := false
	for _, p := range a.periods {
		manual = manual || (ask.From < periodEnd(p) && p.From < ask.To)
	}
	kept := a.planned[:0:0]
	for _, q := range a.planned {
		if q.ID != trip.ID {
			kept = append(kept, q)
		}
	}
	a.planned = kept
	a.savePlannedLocked()
	if !manual {
		// a pergunta da programada substitui as normais do mesmo trecho
		list := a.loadRcAwayLocked()
		asks := list[:0:0]
		for _, q := range list {
			if q.Kind == rcAskBack || q.ID == ask.ID || !(ask.From < q.To && q.From < ask.To) {
				asks = append(asks, q)
			}
		}
		a.rcAsks = pruneRcAway(append(asks, ask), time.Now())
		a.saveRcAwayLocked()
	}
	a.mu.Unlock()
	if manual {
		a.logEvent(fmt.Sprintf("Saída programada \"%s\": já havia um trecho marcado à mão nesse horário; ficou o seu", trip.Name))
	} else {
		a.logEvent(fmt.Sprintf("Saída programada \"%s\": o %s voltou a conectar às %s; o app pergunta na tela Agora se você saiu mesmo", trip.Name, devLabel(dev), conn.Format("02/01 15:04")))
		a.emitRcAway()
	}
	a.emitPlanned()
	return true
}

// plannedAsk acha a saída e monta a pergunta.
func (a *App) plannedAsk(p PlannedTrip, dev string, conn time.Time) RcAwayAsk {
	plan := time.UnixMilli(p.From)
	ask := RcAwayAsk{ID: p.ID, To: conn.UnixMilli(), Kind: askPlanned, Planned: p.ID, Name: p.Name,
		GmcGoes: p.GMC, RcGoes: p.RC, PlanAt: p.From, Dev: dev}
	oneHome := p.GMC != p.RC
	var normal float64
	if oneHome {
		normal = a.normalRatioBefore(plan.Add(-plannedWindow))
	}
	switch {
	case p.Left > 0 && p.Left < conn.UnixMilli():
		ask.From, ask.Source = p.Left, "desconexao"
		ask.LeftDev = p.LeftDev
	case oneHome && normal > 0:
		if t, ok := a.ratioBreak(plan.Add(-plannedWindow), minTime(plan.Add(plannedWindow), conn), normal); ok {
			ask.From, ask.Source = t.UnixMilli(), "razao"
		}
	}
	if ask.From == 0 {
		ask.From, ask.Source, ask.Estimated = p.From, "plano", true
		if !plan.Before(conn) {
			ask.From = conn.Add(-time.Minute).UnixMilli()
		}
		// a conta ficou com o jeito de casa o tempo todo: talvez não tenha saído
		if oneHome && normal > 0 {
			r, _ := countRatio(a.plannedPairs(plan, conn))
			ask.Missed = rcAwayHint(r, normal) == "casa"
		}
	}
	// chegou bem antes de conectar? a razão voltou ao normal
	if oneHome && normal > 0 && !ask.Missed {
		if t, ok := a.ratioBack(time.UnixMilli(ask.From), conn, normal); ok && conn.Sub(t) > plannedWindow {
			ask.BackHint = t.UnixMilli()
		}
	}
	return ask
}

func minTime(a, b time.Time) time.Time {
	if a.Before(b) {
		return a
	}
	return b
}

// normalRatioBefore é a razão RC102/GMC dos 3 dias antes de t, sem os
// trechos em que o RC102 estava fora.
func (a *App) normalRatioBefore(t time.Time) float64 {
	away := a.rcAwayChecker()
	before := a.plannedPairs(t.Add(-72*time.Hour), t)
	kept := before[:0]
	for _, p := range before {
		if !away(p.T) {
			kept = append(kept, p)
		}
	}
	r, _ := countRatio(kept)
	return r
}

// runOut diz se a razão de ps[i:i+plannedRunMin] está longe da normal.
func runOut(ps []intercal.Pair, i int, normal float64) bool {
	if i < 0 || i+plannedRunMin > len(ps) {
		return false
	}
	r, _ := countRatio(ps[i : i+plannedRunMin])
	return rcAwayHint(r, normal) == "fora"
}

// ratioBreak é o minuto de [from, to) em que a razão sai do normal e fica
// fora por pelo menos duas janelas seguidas.
func (a *App) ratioBreak(from, to time.Time, normal float64) (time.Time, bool) {
	ps := a.plannedPairs(from, to.Add(2*plannedRunMin*time.Minute))
	for i := range ps {
		if !ps[i].T.Before(to) {
			break
		}
		if runOut(ps, i, normal) && (i+2*plannedRunMin > len(ps) || runOut(ps, i+plannedRunMin, normal)) {
			// a janela vira quando metade dela já é de fora: a troca fica
			// perto do meio
			return ps[i+plannedRunMin/2].T, true
		}
	}
	return time.Time{}, false
}

// ratioBack é quando a razão volta ao normal: o meio da última janela fora
// do normal em [from, to).
func (a *App) ratioBack(from, to time.Time, normal float64) (time.Time, bool) {
	ps := a.plannedPairs(from, to)
	for i := len(ps) - plannedRunMin; i >= 0; i-- {
		if runOut(ps, i, normal) {
			return ps[i+plannedRunMin/2].T, true
		}
	}
	return time.Time{}, false
}

// plannedPeriod é o trecho de uma resposta "saí" da programada.
func plannedPeriod(q RcAwayAsk, from, to int64, name string, gmcGoes, rcGoes bool) Period {
	if strings.TrimSpace(name) == "" {
		name = q.Name
	}
	gmc, rc := WhereHome, WhereHome
	if gmcGoes {
		gmc = WhereMe
	}
	if rcGoes {
		rc = WhereMe
	}
	return Period{From: from, To: to, Name: name, GMC: gmc, RC: rc, NotHome: true, Planned: q.Planned, Estimated: q.Estimated && from == q.From}
}

// AnswerPlanned responde o "Você saiu mesmo?": "sai" marca o trecho (from,
// to, nome e aparelhos podem vir ajustados; zero = os da pergunta), "naosai"
// só tira a pergunta. Devolve o trecho criado (ID 0 se nenhum).
func (a *App) AnswerPlanned(id int64, answer string, from, to int64, name string, gmcGoes, rcGoes bool) (Period, error) {
	a.mu.Lock()
	var q RcAwayAsk
	found := false
	for _, x := range a.loadRcAwayLocked() {
		if x.ID == id && x.Kind == askPlanned {
			q, found = x, true
		}
	}
	a.mu.Unlock()
	if !found {
		return Period{}, errors.New("essa pergunta já foi respondida")
	}
	var p Period
	switch answer {
	case "sai":
		if from == 0 || to == 0 {
			from, to = q.From, q.To
		}
		if !gmcGoes && !rcGoes {
			return p, errors.New("escolha pelo menos um aparelho que foi com você")
		}
		var err error
		if p, err = a.SavePeriod(plannedPeriod(q, from, to, name, gmcGoes, rcGoes)); err != nil {
			return p, err
		}
	case "naosai":
	default:
		return p, errors.New("resposta inválida")
	}
	a.mu.Lock()
	kept := a.rcAsks[:0]
	for _, x := range a.rcAsks {
		if x.ID != id {
			kept = append(kept, x)
		}
	}
	a.rcAsks = kept
	a.saveRcAwayLocked()
	a.mu.Unlock()
	a.emitRcAway()
	return p, nil
}

// mergePlanned junta as programadas de um backup (só as que faltam e ainda
// não venceram).
func (a *App) mergePlanned(list []PlannedTrip) {
	now := time.Now()
	a.mu.Lock()
	have := map[int64]bool{}
	for _, p := range a.loadPlannedLocked() {
		have[p.ID] = true
	}
	n := 0
	for _, p := range list {
		if p.ID == 0 || have[p.ID] || (!p.GMC && !p.RC) || now.Sub(time.UnixMilli(p.From)) > plannedForget {
			continue
		}
		have[p.ID] = true
		a.planned = append(a.planned, p)
		n++
	}
	if n > 0 {
		a.savePlannedLocked()
	}
	a.mu.Unlock()
	if n > 0 {
		a.emitPlanned()
	}
}

// plannedPairs são os minutos com os dois aparelhos, como no intercalPairs,
// mas aceitando também os minutos do GMC-800 baixados da memória: com o PC
// desligado na saída, é só deles que a razão sai.
func (a *App) plannedPairs(from, to time.Time) []intercal.Pair {
	if a.store == nil || !to.After(from) {
		return nil
	}
	rcs := a.rcRange(from, to)
	if len(rcs) == 0 {
		return nil
	}
	pts, err := a.store.Range(from, to)
	if err != nil {
		return nil
	}
	gm := make(map[int64]float64, len(pts))
	for _, p := range pts {
		if p.HasObs(store.ObsSaturation) || p.HasObs(store.ObsInterference) {
			continue
		}
		c := p.CPM
		if p.Has {
			c = p.Raw
		}
		gm[p.T.Truncate(time.Minute).Unix()] = c
	}
	out := make([]intercal.Pair, 0, len(rcs))
	for _, r := range rcs {
		if r.N < rcNMin || r.T.Before(from) || !r.T.Before(to) {
			continue
		}
		if c, ok := gm[r.T.Truncate(time.Minute).Unix()]; ok {
			out = append(out, intercal.Pair{T: r.T, Counts: c, USvh: r.USvh, CPS: r.CPS})
		}
	}
	return out
}

// plannedRefine refaz as perguntas de programadas com horário estimado
// depois de baixar o histórico do GMC-800 (a razão pode achar a saída agora).
func (a *App) plannedRefine() {
	a.mu.Lock()
	var todo []RcAwayAsk
	for _, q := range a.loadRcAwayLocked() {
		if q.Kind == askPlanned && q.Source == "plano" {
			todo = append(todo, q)
		}
	}
	a.mu.Unlock()
	changed := false
	for _, q := range todo {
		p := PlannedTrip{ID: q.Planned, Name: q.Name, From: q.PlanAt, GMC: q.GmcGoes, RC: q.RcGoes}
		n := a.plannedAsk(p, q.Dev, time.UnixMilli(q.To))
		if n.Source == q.Source && n.Missed == q.Missed && n.BackHint == q.BackHint {
			continue
		}
		n.ID = q.ID
		a.mu.Lock()
		for i := range a.rcAsks {
			if a.rcAsks[i].ID == q.ID {
				a.rcAsks[i] = n
				changed = true
			}
		}
		a.saveRcAwayLocked()
		a.mu.Unlock()
	}
	if changed {
		a.emitRcAway()
	}
}
