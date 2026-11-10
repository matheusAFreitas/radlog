package app

import (
	"bufio"
	"encoding/json"
	"errors"
	"math"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"radlog/internal/core/intercal"
)

// ---------- "o RadiaCode estava fora de casa?" (2.38.0) ----------
//
// O normal é o RadiaCode com você e o app supõe que você estava em casa: o
// que ele mede entra no fundo da casa dele. Quando ele some do PC por um
// tempo (o mesmo mínimo da pergunta do GMC, Configurações) e volta, o app
// pergunta onde ele estava, já com o que a memória do aparelho trouxe. A
// resposta "comigo, fora de casa" vira um trecho de Fora de casa com o GMC em
// casa, o RadiaCode com você e "eu não estava em casa": as leituras saem do
// fundo da casa e do padrão por hora e continuam na dose.
//
// As perguntas sem resposta ficam em fora-perguntas.json até você responder
// (ou até o trecho ser marcado por outro caminho).

const (
	rcAwayFile    = "fora-perguntas.json"
	rcAwayWait    = 90 * time.Second    // conectado há isso: a memória já chegou
	rcAwayMaxGap  = 14 * 24 * time.Hour // mais que isso não é "saí de casa"
	rcAwayKeep    = 30 * 24 * time.Hour // pergunta mais velha some sozinha
	rcAwayMaxList = 20
	rcAwayLook    = 35 * 24 * time.Hour // até onde procura o último minuto gravado
)

// RcAwayAsk é uma pergunta pendente: o RadiaCode ficou sem conexão de From a To.
type RcAwayAsk struct {
	ID   int64 `json:"id"`
	From int64 `json:"from"` // ms: fim do último minuto gravado antes de sumir
	To   int64 `json:"to"`   // ms: quando voltou a conectar
}

// RcAwayView é a pergunta com o resumo do trecho para a tela.
type RcAwayView struct {
	RcAwayAsk
	MemMinutes int     `json:"memMinutes"` // minutos que vieram da memória do aparelho
	AvgUSv     float64 `json:"avgUSv"`
	MaxUSv     float64 `json:"maxUSv"`
	MaxAt      int64   `json:"maxAt"`
	DoseUSv    float64 `json:"doseUSv"`
	GmcUSv     float64 `json:"gmcUSv"` // média do GMC no mesmo trecho (0 = sem)
	// razão de contagens RadiaCode/GMC no trecho e a normal dos dias de antes
	Ratio       float64 `json:"ratio"`
	NormalRatio float64 `json:"normalRatio"`
	Hint        string  `json:"hint"` // "fora", "casa" ou "" (sem dica)
}

func (a *App) rcAwayPath() string { return filepath.Join(a.dataDir, rcAwayFile) }

func (a *App) loadRcAwayLocked() []RcAwayAsk {
	if a.dataDir == "" {
		return a.rcAsks
	}
	if a.rcAsksLoaded {
		return a.rcAsks
	}
	a.rcAsksLoaded = true
	b, err := os.ReadFile(a.rcAwayPath())
	if err == nil {
		var list []RcAwayAsk
		if json.Unmarshal(b, &list) == nil {
			a.rcAsks = list
		}
	}
	return a.rcAsks
}

func (a *App) saveRcAwayLocked() {
	if a.dataDir == "" {
		return
	}
	b, _ := json.MarshalIndent(a.rcAsks, "", "  ")
	tmp := a.rcAwayPath() + ".tmp"
	if os.WriteFile(tmp, b, 0o644) == nil {
		_ = os.Rename(tmp, a.rcAwayPath())
	}
}

// rcLastMinuteBefore é o fim do último minuto do RadiaCode gravado antes de t
// (zero = nenhum nas últimas semanas).
func (a *App) rcLastMinuteBefore(t time.Time) time.Time {
	pts := a.rcRange(t.Add(-rcAwayLook), t)
	if len(pts) == 0 {
		return time.Time{}
	}
	return pts[len(pts)-1].T.Add(time.Minute)
}

// rcAwayGap decide se um sumiço de from a to vale a pergunta.
func rcAwayGap(from, to time.Time, minLen time.Duration) bool {
	if from.IsZero() || !to.After(from) {
		return false
	}
	d := to.Sub(from)
	return d >= minLen && d <= rcAwayMaxGap
}

// rcAwayDetect cria a pergunta de uma reconexão (from = fim do último minuto
// gravado antes dela, to = começo dela).
func (a *App) rcAwayDetect(from, to time.Time) {
	if a.isDemo() {
		return
	}
	a.mu.Lock()
	minLen := time.Duration(normAwayAsk(a.settings.AwayAskMin)) * time.Minute
	a.mu.Unlock()
	if !rcAwayGap(from, to, minLen) {
		return
	}
	ask := RcAwayAsk{ID: from.UnixMilli(), From: from.UnixMilli(), To: to.UnixMilli()}
	a.mu.Lock()
	added := a.addRcAwayLocked(ask, time.Now())
	a.mu.Unlock()
	if added {
		a.logf("RadiaCode: ficou %s sem conexão (%s a %s); o app pergunta na tela Agora se ele estava fora de casa",
			fmtMinutes(int(to.Sub(from)/time.Minute)), from.Format("02/01 15:04"), to.Format("02/01 15:04"))
		a.emitRcAway()
	}
}

// addRcAwayLocked guarda a pergunta, se o trecho ainda não está marcado nem
// perguntado. (a.mu travado)
func (a *App) addRcAwayLocked(ask RcAwayAsk, now time.Time) bool {
	list := a.loadRcAwayLocked()
	for _, p := range a.periods {
		if ask.From < periodEnd(p) && p.From < ask.To {
			return false
		}
	}
	for _, q := range list {
		if ask.From < q.To && q.From < ask.To {
			return false
		}
	}
	list = append(list, ask)
	a.rcAsks = pruneRcAway(list, now)
	a.saveRcAwayLocked()
	return true
}

// pruneRcAway tira as perguntas velhas e deixa só as últimas, em ordem.
func pruneRcAway(list []RcAwayAsk, now time.Time) []RcAwayAsk {
	out := list[:0:0]
	for _, q := range list {
		if now.Sub(time.UnixMilli(q.To)) <= rcAwayKeep {
			out = append(out, q)
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].From < out[j].From })
	if len(out) > rcAwayMaxList {
		out = out[len(out)-rcAwayMaxList:]
	}
	return out
}

// pendingRcAwayLocked são as perguntas ainda abertas: some a que já foi
// marcada por outro caminho (Fora de casa, Onde está). (a.mu travado)
func (a *App) pendingRcAwayLocked(now time.Time) []RcAwayAsk {
	list := pruneRcAway(a.loadRcAwayLocked(), now)
	out := list[:0:0]
	for _, q := range list {
		covered := false
		for _, p := range a.periods {
			covered = covered || (q.From < periodEnd(p) && p.From < q.To)
		}
		if !covered {
			out = append(out, q)
		}
	}
	if len(out) != len(a.rcAsks) {
		a.rcAsks = out
		a.saveRcAwayLocked()
	}
	return append([]RcAwayAsk(nil), out...)
}

// GetRcAwayAsks devolve as perguntas abertas com o resumo de cada trecho.
func (a *App) GetRcAwayAsks() []RcAwayView {
	now := time.Now()
	a.mu.Lock()
	list := a.pendingRcAwayLocked(now)
	a.mu.Unlock()
	out := make([]RcAwayView, 0, len(list))
	for _, q := range list {
		out = append(out, a.rcAwayView(q))
	}
	return out
}

// rcAwayView monta o resumo do trecho: o que o RadiaCode trouxe, o GMC no
// mesmo tempo e a dica pela razão de contagens.
func (a *App) rcAwayView(q RcAwayAsk) RcAwayView {
	v := RcAwayView{RcAwayAsk: q}
	from, to := time.UnixMilli(q.From), time.UnixMilli(q.To)
	rcs := a.rcRange(from, to)
	v.MemMinutes = len(rcs)
	var sum float64
	for _, p := range rcs {
		sum += p.USvh
		v.DoseUSv += p.USvh / 60
		if p.USvh > v.MaxUSv {
			v.MaxUSv, v.MaxAt = p.USvh, p.T.UnixMilli()
		}
	}
	if len(rcs) > 0 {
		v.AvgUSv = sum / float64(len(rcs))
	}
	if a.store != nil {
		if pts, err := a.store.Range(from, to); err == nil && len(pts) > 0 {
			var s float64
			for _, p := range pts {
				s += p.CPM
			}
			a.mu.Lock()
			v.GmcUSv = a.convCalLocked().USv(s / float64(len(pts)))
			a.mu.Unlock()
		}
	}
	v.Ratio, _ = countRatio(a.intercalPairs(from, to, false))
	// o normal: os 3 dias antes, sem os trechos em que o RadiaCode estava fora
	away := a.rcAwayChecker()
	before := a.intercalPairs(from.Add(-72*time.Hour), from, false)
	kept := before[:0]
	for _, p := range before {
		if !away(p.T) {
			kept = append(kept, p)
		}
	}
	v.NormalRatio, _ = countRatio(kept)
	v.Hint = rcAwayHint(v.Ratio, v.NormalRatio)
	return v
}

// countRatio é a razão de contagens RadiaCode/GMC de uns minutos (0 = poucos).
func countRatio(ps []intercal.Pair) (float64, int) {
	var g, c float64
	for _, p := range ps {
		g += p.Counts
		c += p.CPS * 60
	}
	if len(ps) < rcAwayMinPairs || g <= 0 {
		return 0, len(ps)
	}
	return c / g, len(ps)
}

const rcAwayMinPairs = 10

// rcAwayHint compara a razão do trecho com a normal: longe dela, o RadiaCode
// mediu outro lugar; perto, parece que ficou em casa.
func rcAwayHint(ratio, normal float64) string {
	if ratio <= 0 || normal <= 0 {
		return ""
	}
	d := math.Abs(ratio/normal - 1)
	switch {
	case d >= 0.20:
		return "fora"
	case d <= 0.08:
		return "casa"
	}
	return ""
}

// AnswerRcAway responde uma pergunta: "fora" (comigo, fora de casa) marca o
// trecho de from a to; "casa" e "naosei" só tiram a pergunta. Devolve o
// trecho criado (ID 0 se nenhum).
func (a *App) AnswerRcAway(id int64, answer string, from, to int64, name string) (Period, error) {
	a.mu.Lock()
	list := a.loadRcAwayLocked()
	idx := -1
	for i, q := range list {
		if q.ID == id {
			idx = i
		}
	}
	a.mu.Unlock()
	if idx < 0 {
		return Period{}, errors.New("essa pergunta já foi respondida")
	}
	var p Period
	switch answer {
	case "fora":
		if from == 0 || to == 0 {
			from, to = list[idx].From, list[idx].To
		}
		var err error
		p, err = a.SavePeriod(Period{From: from, To: to, Name: name, GMC: WhereHome, RC: WhereMe, NotHome: true})
		if err != nil {
			return p, err
		}
	case "casa", "naosei":
	default:
		return p, errors.New("resposta inválida")
	}
	a.mu.Lock()
	kept := a.rcAsks[:0]
	for _, q := range a.rcAsks {
		if q.ID != id {
			kept = append(kept, q)
		}
	}
	a.rcAsks = kept
	a.saveRcAwayLocked()
	a.mu.Unlock()
	a.emitRcAway()
	return p, nil
}

// UndoRcAway desfaz uma resposta: apaga o trecho criado (se houve) e devolve
// a pergunta.
func (a *App) UndoRcAway(ask RcAwayAsk, periodID int64) error {
	if periodID != 0 {
		if err := a.DeletePeriod(periodID); err != nil {
			return err
		}
	}
	a.mu.Lock()
	a.addRcAwayLocked(ask, time.Now())
	a.mu.Unlock()
	a.emitRcAway()
	return nil
}

func (a *App) emitRcAway() { a.emit("rcaway", a.GetRcAwayAsks()) }

// rcLiveMinutes conta os minutos do RadiaCode gravados ao vivo (conectado ao
// PC) em [from, to); os que vieram da memória do aparelho não contam.
func (a *App) rcLiveMinutes(from, to time.Time) int {
	a.rc.fileM.Lock()
	defer a.rc.fileM.Unlock()
	seen := map[int64]bool{}
	for m := time.Date(from.Year(), from.Month(), 1, 0, 0, 0, 0, from.Location()); m.Before(to); m = m.AddDate(0, 1, 0) {
		f, err := os.Open(filepath.Join(a.rcDir(), m.Format("2006-01")+".csv"))
		if err != nil {
			continue
		}
		sc := bufio.NewScanner(f)
		for sc.Scan() {
			p := strings.Split(sc.Text(), ",")
			if len(p) < 4 || (len(p) >= 5 && p[4] == rcMemSrc) {
				continue
			}
			t, err := time.Parse(time.RFC3339, p[0])
			if err != nil || t.Before(from) || !t.Before(to) {
				continue
			}
			seen[t.Unix()/60] = true
		}
		f.Close()
	}
	return len(seen)
}
