package app

import (
	"errors"
	"sort"
	"time"

	"radlog/internal/gmc"
	"radlog/internal/store"
)

// ---------- histórico do GMC-800 sozinho ao conectar (2.83.0) ----------
//
// Spec ux/historico-automatico (ACE, 14/11). Toda vez que o GMC-800 conecta
// (ou já estava conectado quando o app abriu), o app espera uns segundos de
// conexão estável e baixa a memória dele pelo mesmo caminho do botão "Baixar
// histórico". Os trechos novos de 15 min ou mais (Configurações) viram a
// pergunta "Você estava fora de casa?" no Agora, junto das do RC102
// (fora-perguntas.json); se o RC102 também ficou sem conexão no mesmo
// trecho, vira uma pergunta só para os dois.

var (
	autoHistWait = 10 * time.Second // conexão estável antes de baixar
	autoHistGap  = 2 * time.Minute  // entre dois downloads automáticos (cabo ruim)
	autoHistPoll = 2 * time.Second  // de quanto em quanto confere se pode baixar
)

var (
	errCfgBusy      = errors.New("espere terminar a gravação da configuração")
	errDownloading  = errors.New("já estou baixando o histórico")
	errHistCanceled = errors.New("leitura cancelada")
)

// AutoHistory é o aviso do download automático para a tela (evento
// "history-auto").
type AutoHistory struct {
	OK       bool   `json:"ok"`
	Added    int    `json:"added"`
	From     int64  `json:"from"`
	To       int64  `json:"to"`
	Alerts   int    `json:"alerts"`
	Asks     int    `json:"asks"`
	Canceled bool   `json:"canceled"`
	Partial  bool   `json:"partial"` // parou no meio, mas guardou o que chegou
	Err      string `json:"err"`
}

// autoHistoryOnConnect começa o download automático da conexão d.
func (a *App) autoHistoryOnConnect(d *gmc.Device) {
	if !a.autoHistoryOn() {
		return
	}
	a.goSafe("autoHistory", func() { a.autoHistory(d) })
}

func (a *App) autoHistoryOn() bool {
	a.mu.Lock()
	defer a.mu.Unlock()
	return !a.settings.NoAutoHistory
}

// autoHistory espera a conexão ficar estável (e a gravação da configuração
// ou outro download terminar) e baixa. Cancelado, só tenta de novo na
// próxima conexão.
func (a *App) autoHistory(d *gmc.Device) {
	same := func() bool {
		a.mu.Lock()
		defer a.mu.Unlock()
		return a.dev == d && !a.settings.NoAutoHistory
	}
	time.Sleep(autoHistWait)
	for {
		if !same() {
			return
		}
		a.mu.Lock()
		wait := time.Until(a.autoHistAt.Add(autoHistGap))
		a.mu.Unlock()
		if wait <= 0 && !a.cfgBusy.Load() && !a.downloading.Load() {
			dl := a.downloadHistory
			if a.histDownload != nil {
				dl = a.histDownload // testes
			}
			res, err := dl()
			if errors.Is(err, errCfgBusy) || errors.Is(err, errDownloading) {
				time.Sleep(autoHistPoll) // começou outra coisa entre a conferência e o download
				continue
			}
			a.mu.Lock()
			a.autoHistAt = time.Now()
			a.mu.Unlock()
			a.autoHistoryDone(res, err)
			return
		}
		if wait < autoHistPoll {
			wait = autoHistPoll
		}
		time.Sleep(min(wait, autoHistPoll))
	}
}

// autoHistoryDone avisa a tela de como foi.
func (a *App) autoHistoryDone(res ImportResult, err error) {
	ev := AutoHistory{Added: res.Added, From: res.NewFrom, To: res.NewTo, Alerts: res.Alerts, Asks: res.Asks, Canceled: res.Canceled}
	switch {
	case res.Canceled:
	case err != nil:
		ev.Err = err.Error()
	case res.Partial != "":
		ev.Partial, ev.Err = true, res.Partial
	default:
		ev.OK = true
	}
	if ev.OK && res.Added == 0 {
		a.logf("histórico automático: nada novo na memória do GMC-800")
	}
	a.emit("history-auto", ev)
}

// ---------- a pergunta "fora de casa?" dos trechos novos ----------

const (
	askGmc  = "gmc"   // só o GMC-800 ficou sem conexão (o RC102 estava no PC ou não é usado)
	askBoth = "ambos" // os dois ficaram sem conexão no mesmo trecho
)

func (a *App) getHistAskFrom() int64 {
	if d, ok := a.readCounterSync(); ok && a.dataDir != "" {
		return d.AskFrom
	}
	a.mu.Lock()
	defer a.mu.Unlock()
	return a.histAskFrom
}

func (a *App) setHistAskFrom(v int64) {
	a.mu.Lock()
	a.histAskFrom = v
	a.mu.Unlock()
	if a.dataDir == "" {
		return
	}
	d, _ := a.readCounterSync()
	d.AskFrom = v
	a.writeCounterSync(d)
}

// gmcAwayAsks transforma os minutos novos em perguntas. Com a leitura
// interrompida (complete false), o último trecho pode ter vindo pela metade:
// ele espera o próximo download completo (perguntarDesde). Devolve quantas
// perguntas foram criadas.
func (a *App) gmcAwayAsks(added []store.Point, complete bool) int {
	pts := added
	if askFrom := a.getHistAskFrom(); askFrom > 0 && a.store != nil {
		// junta o que o download interrompido já tinha trazido
		seen := map[int64]bool{}
		for _, p := range pts {
			seen[p.T.Unix()/60] = true
		}
		if old, err := a.store.Range(time.UnixMilli(askFrom), time.Now().Add(time.Minute)); err == nil {
			for _, p := range old {
				if p.Src == store.SrcDevice && !seen[p.T.Unix()/60] {
					seen[p.T.Unix()/60] = true
					pts = append(pts, p)
				}
			}
		}
	}
	if len(pts) == 0 {
		if complete {
			a.setHistAskFrom(0)
		}
		return 0
	}
	sort.Slice(pts, func(i, j int) bool { return pts[i].T.Before(pts[j].T) })
	if complete {
		a.setHistAskFrom(0)
	} else {
		// o último trecho contínuo fica para depois
		i := len(pts) - 1
		for i > 0 && pts[i].T.Sub(pts[i-1].T) <= newRangeGap {
			i--
		}
		a.setHistAskFrom(pts[i].T.UnixMilli())
		pts = pts[:i]
	}
	ranges := a.newRanges(pts)
	now := time.Now()
	n := 0
	for _, r := range ranges {
		ask := RcAwayAsk{ID: r.From, From: r.From, To: r.To, Kind: askGmc, RcHome: r.RcSuggest == WhereHome}
		a.mu.Lock()
		ok := a.addGmcAwayLocked(ask, now)
		a.mu.Unlock()
		if ok {
			n++
			a.logf("histórico: o GMC-800 trouxe %s sem conexão (%s a %s); o app pergunta na tela Agora se foi fora de casa",
				fmtMinutes(r.Minutes), time.UnixMilli(r.From).Format("02/01 15:04"), time.UnixMilli(r.To).Format("02/01 15:04"))
		}
	}
	if n > 0 {
		a.emitRcAway()
	}
	return n
}

// addGmcAwayLocked guarda a pergunta do trecho do GMC-800: some se o trecho
// já está marcado ou tem outra pergunta (a da saída programada ganha); com a
// do RC102 no mesmo tempo, as duas viram uma só. (a.mu travado)
func (a *App) addGmcAwayLocked(ask RcAwayAsk, now time.Time) bool {
	list := a.loadRcAwayLocked()
	for _, p := range a.periods {
		if ask.From < periodEnd(p) && p.From < ask.To {
			return false
		}
	}
	for i, q := range list {
		if ask.From < q.To && q.From < ask.To {
			if q.Kind != "" {
				return false
			}
			list[i] = joinAsks(q, ask)
			a.rcAsks = pruneRcAway(list, now)
			a.saveRcAwayLocked()
			return true
		}
	}
	a.rcAsks = pruneRcAway(append(list, ask), now)
	a.saveRcAwayLocked()
	return true
}

// joinAsks junta a pergunta do RC102 e a do GMC-800 do mesmo trecho.
func joinAsks(x, y RcAwayAsk) RcAwayAsk {
	j := x
	if y.From < j.From {
		j.From = y.From
	}
	if y.To > j.To {
		j.To = y.To
	}
	j.ID, j.Kind, j.RcHome = j.From, askBoth, false
	return j
}

// AnswerAway responde a pergunta do GMC-800 (ou dos dois): "sai" marca o
// trecho de from a to (zero = o da pergunta) com o nome e os aparelhos que
// foram; "casa" só tira a pergunta. Devolve o trecho criado (ID 0 se nenhum).
func (a *App) AnswerAway(id int64, answer string, from, to int64, name string, gmcGoes, rcGoes bool) (Period, error) {
	a.mu.Lock()
	var q RcAwayAsk
	found := false
	for _, x := range a.loadRcAwayLocked() {
		if x.ID == id && (x.Kind == askGmc || x.Kind == askBoth) {
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
		if !(from < to) {
			return p, errors.New("a volta precisa ser depois da saída")
		}
		var err error
		if p, err = a.SavePeriod(plannedPeriod(RcAwayAsk{}, from, to, name, gmcGoes, rcGoes)); err != nil {
			return p, err
		}
	case "casa":
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

// gmcAwayView completa o resumo do trecho com o GMC-800 (média, máximo,
// dose e alertas pela tela Resumo) e a dica pela razão, aceitando os minutos
// baixados da memória.
func (a *App) gmcAwayView(v *RcAwayView) {
	from, to := time.UnixMilli(v.From), time.UnixMilli(v.To)
	if s, err := a.GetSummary(v.From, v.To); err == nil {
		v.GmcMinutes = s.Minutes
		v.GmcUSv = s.AvgUSv
		v.GmcDoseUSv = s.Dose
		if s.MaxUSv != nil {
			v.GmcMaxUSv, v.GmcMaxAt = *s.MaxUSv, s.MaxAt
		}
		v.Alerts = s.Attention + s.Alarm + s.Danger
	}
	v.Ratio, _ = countRatio(a.plannedPairs(from, to))
	v.Hint = rcAwayHint(v.Ratio, v.NormalRatio)
}
