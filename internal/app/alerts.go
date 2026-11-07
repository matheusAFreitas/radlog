package app

import (
	"encoding/csv"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"

	"radlog/internal/store"
)

// ---------- registro de eventos e alertas ----------
//
// eventos.csv: data, texto, tipo, nível, CPM, pico, hora do pico, fim, retro.
// Arquivos de versões antigas só têm as 3 primeiras colunas (ou 2).

func parseEventRow(rec []string) (Event, bool) {
	if len(rec) < 2 {
		return Event{}, false
	}
	t, err := time.Parse(time.RFC3339, rec[0])
	if err != nil {
		return Event{}, false
	}
	ev := Event{Time: t.UnixMilli(), Text: rec[1], Kind: kindEvent}
	col := func(i int) string {
		if i < len(rec) {
			return rec[i]
		}
		return ""
	}
	switch {
	case col(2) == kindAlert:
		ev.Kind = kindAlert
	case len(rec) < 3 && strings.HasPrefix(rec[1], "Alarme"):
		ev.Kind = kindAlert // arquivos de versões antigas, sem a coluna do tipo
	}
	if ev.Kind != kindAlert {
		return ev, true
	}
	ev.Level = col(3)
	if ev.Level == "" {
		ev.Level = "atencao"
		if strings.HasPrefix(ev.Text, "Alarme") {
			ev.Level = "alarme"
		}
		if strings.HasPrefix(ev.Text, "Perigo") {
			ev.Level = "perigo"
		}
	}
	ev.CPM, _ = strconv.ParseFloat(col(4), 64)
	ev.Peak, _ = strconv.ParseFloat(col(5), 64)
	if t, err := time.Parse(time.RFC3339, col(6)); err == nil {
		ev.PeakAt = t.UnixMilli()
	}
	if t, err := time.Parse(time.RFC3339, col(7)); err == nil {
		ev.End = t.UnixMilli()
	}
	ev.Retro = col(8) == "1"
	ev.Device = col(9)
	return ev, true
}

func eventRow(e Event) []string {
	ms := func(v int64) string {
		if v == 0 {
			return ""
		}
		return time.UnixMilli(v).Format(time.RFC3339)
	}
	num := func(v float64) string {
		if v == 0 {
			return ""
		}
		return strconv.FormatFloat(v, 'f', -1, 64)
	}
	if e.Kind != kindAlert {
		return []string{ms(e.Time), e.Text, e.Kind}
	}
	retro := ""
	if e.Retro {
		retro = "1"
	}
	row := []string{ms(e.Time), e.Text, e.Kind, e.Level, num(e.CPM), num(e.Peak), ms(e.PeakAt), ms(e.End), retro, e.Device}
	for len(row) > 3 && row[len(row)-1] == "" {
		row = row[:len(row)-1]
	}
	return row
}

// addEvent põe o evento na lista (em ordem de data), grava no fim do
// eventos.csv se persist e avisa a tela. Devolve o ID.
func (a *App) addEvent(ev Event, persist bool) int64 {
	ev.Demo = !persist
	a.mu.Lock()
	a.evSeq++
	ev.ID = a.evSeq
	a.events = append(a.events, ev)
	sorted := true
	if n := len(a.events); n > 1 && a.events[n-2].Time > ev.Time {
		sort.SliceStable(a.events, func(i, j int) bool { return a.events[i].Time < a.events[j].Time })
		sorted = false
	}
	if len(a.events) > maxEvents {
		a.events = a.events[len(a.events)-maxEvents:]
	}
	a.mu.Unlock()
	if persist && a.dataDir != "" {
		if sorted {
			if f, err := os.OpenFile(a.eventsPath(), os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o644); err == nil {
				w := csv.NewWriter(f)
				_ = w.Write(eventRow(ev))
				w.Flush()
				f.Close()
			}
		} else {
			a.saveEvents()
		}
	}
	a.emit("event", ev)
	return ev.ID
}

// saveEvents regrava o eventos.csv inteiro (sem os do demo).
func (a *App) saveEvents() {
	if a.dataDir == "" {
		return
	}
	a.mu.Lock()
	rows := make([][]string, 0, len(a.events))
	for _, e := range a.events {
		if !e.Demo {
			rows = append(rows, eventRow(e))
		}
	}
	a.mu.Unlock()
	tmp := a.eventsPath() + ".tmp"
	f, err := os.Create(tmp)
	if err != nil {
		return
	}
	w := csv.NewWriter(f)
	_ = w.WriteAll(rows)
	if f.Close() == nil && w.Error() == nil {
		_ = os.Rename(tmp, a.eventsPath())
	}
}

// ---------- alertas ao vivo ----------

// alertIDs são os alertas abertos de uma fonte (contador real ou demo).
type alertIDs struct{ attn, alarm, danger int64 }

// alertStep é o que mudou numa leitura.
type alertStep struct {
	Now           time.Time
	CPM           float64
	AlarmCPM      int
	Lv            Levels
	Demo          bool
	AttnStarted   bool
	AttnEnded     bool
	AttnOn        bool
	AlarmStarted  bool
	AlarmEnded    bool
	AlarmOn       bool // alarme do usuário ligado e acima do limite agora
	DangerStarted bool
	DangerEnded   bool
	// RadiaCode: CPM é µSv/h e os limites vêm do RcLim
	Device string
	RcLim  RcLimits
}

func (a *App) openAlert(level string, s alertStep) int64 {
	var text string
	switch level {
	case "perigo":
		text = fmt.Sprintf("Perigo: %.0f CPM (a partir de %d)", s.CPM, s.Lv.DangerCPM)
	case "alarme":
		text = fmt.Sprintf("Alarme: %.0f CPM (limite %d)", s.CPM, s.AlarmCPM)
	default:
		text = fmt.Sprintf("Atenção: %.0f CPM (a partir de %d)", s.CPM, s.Lv.AttentionCPM)
	}
	if s.Device == devRC {
		text = rcAlertText(level, s.CPM, s.RcLim)
	}
	ms := s.Now.UnixMilli()
	ev := Event{Time: ms, Kind: kindAlert, Level: level, CPM: s.CPM, Peak: s.CPM, PeakAt: ms, Text: text, Device: s.Device}
	if s.Demo {
		ev.Text = demoPrefix + text
	}
	return a.addEvent(ev, !s.Demo)
}

// stepAlerts abre, atualiza o pico e fecha os alertas de uma fonte.
// ids aponta para a.alerts ou a.demo.alerts (protegidos por a.mu).
func (a *App) stepAlerts(ids *alertIDs, s alertStep) {
	changed := false
	// Pulou vários níveis no mesmo segundo: abre só o mais alto (ele já anota
	// o episódio). Os de baixo abrem depois, se o valor voltar só até eles.
	if s.AttnStarted && !s.AlarmStarted && !s.DangerStarted {
		id := a.openAlert("atencao", s)
		a.mu.Lock()
		ids.attn = id
		a.mu.Unlock()
		changed = true
	}
	if s.AlarmStarted && !s.DangerStarted {
		id := a.openAlert("alarme", s)
		a.mu.Lock()
		ids.alarm = id
		a.mu.Unlock()
		changed = true
	}
	if s.DangerStarted {
		id := a.openAlert("perigo", s)
		a.mu.Lock()
		ids.danger = id
		a.mu.Unlock()
		changed = true
	}

	a.mu.Lock()
	for _, id := range []int64{ids.attn, ids.alarm, ids.danger} {
		if e := a.eventLocked(id); e != nil && s.CPM > e.Peak {
			e.Peak, e.PeakAt = s.CPM, s.Now.UnixMilli()
			changed = true
		}
	}
	var closeIDs []int64
	if s.AlarmEnded && ids.alarm != 0 {
		closeIDs = append(closeIDs, ids.alarm)
		ids.alarm = 0
	}
	if s.AttnEnded && ids.attn != 0 {
		closeIDs = append(closeIDs, ids.attn)
		ids.attn = 0
	}
	if s.DangerEnded && ids.danger != 0 {
		closeIDs = append(closeIDs, ids.danger)
		ids.danger = 0
	}
	// saiu de um nível mas continua num de baixo que não tinha sido anotado
	// (o valor pulou direto para cima): abre esse agora
	reopenAlarm := s.DangerEnded && s.AlarmOn && ids.alarm == 0
	reopen := (s.AlarmEnded || s.DangerEnded) && s.AttnOn && ids.attn == 0 && ids.alarm == 0 && !reopenAlarm
	a.mu.Unlock()

	if len(closeIDs) > 0 {
		a.finishAlerts(closeIDs, s.Now, s.Demo)
		changed = true
	}
	if reopenAlarm {
		id := a.openAlert("alarme", s)
		a.mu.Lock()
		ids.alarm = id
		a.mu.Unlock()
		changed = true
	}
	if reopen {
		id := a.openAlert("atencao", s)
		a.mu.Lock()
		ids.attn = id
		a.mu.Unlock()
		changed = true
	}
	if changed {
		a.emitActive()
	}
}

// closeAlerts fecha tudo que estiver aberto (contador desconectado, demo
// desligado).
func (a *App) closeAlerts(ids *alertIDs, now time.Time, demo bool) {
	a.mu.Lock()
	var list []int64
	for _, id := range []int64{ids.attn, ids.alarm, ids.danger} {
		if id != 0 {
			list = append(list, id)
		}
	}
	*ids = alertIDs{}
	a.mu.Unlock()
	if len(list) > 0 {
		a.finishAlerts(list, now, demo)
		a.emitActive()
	}
}

func (a *App) finishAlerts(ids []int64, now time.Time, demo bool) {
	a.mu.Lock()
	for _, id := range ids {
		if e := a.eventLocked(id); e != nil && e.End == 0 {
			e.End = now.UnixMilli()
		}
	}
	a.mu.Unlock()
	if !demo {
		a.saveEvents()
		// os do RadiaCode guardam as leituras dele
		var gmc []int64
		var rc []Event
		a.mu.Lock()
		for _, id := range ids {
			if e := a.eventLocked(id); e != nil && e.Device == devRC {
				if e.End != 0 {
					rc = append(rc, *e)
				}
			} else {
				gmc = append(gmc, id)
			}
		}
		a.mu.Unlock()
		if len(gmc) > 0 {
			a.saveAlertSeconds(gmc)
		}
		if len(rc) > 0 && a.dataDir != "" {
			a.saveRcAlertSeconds(rc)
		}
	}
	a.emit("event", Event{Kind: kindAlert})
}

func (a *App) eventLocked(id int64) *Event {
	if id == 0 {
		return nil
	}
	for i := len(a.events) - 1; i >= 0; i-- {
		if a.events[i].ID == id {
			return &a.events[i]
		}
	}
	return nil
}

func (a *App) isActiveLocked(id int64) bool {
	if id == 0 {
		return false
	}
	if id == a.alerts.attn || id == a.alerts.alarm || id == a.alerts.danger {
		return true
	}
	if r := a.rcAlert.ids; id == r.attn || id == r.alarm || id == r.danger {
		return true
	}
	return a.demo.Active && (id == a.demo.alerts.attn || id == a.demo.alerts.alarm || id == a.demo.alerts.danger)
}

// GetActiveAlerts devolve os alertas acontecendo agora (o mais grave primeiro).
func (a *App) GetActiveAlerts() []Event {
	a.mu.Lock()
	defer a.mu.Unlock()
	out := []Event{}
	r := a.rcAlert.ids
	ids := []int64{a.alerts.danger, a.alerts.alarm, a.alerts.attn, r.danger, r.alarm, r.attn}
	if a.demo.Active {
		ids = append([]int64{a.demo.alerts.danger, a.demo.alerts.alarm, a.demo.alerts.attn}, ids...)
	}
	for _, id := range ids {
		if e := a.eventLocked(id); e != nil {
			c := *e
			c.Active = true
			out = append(out, c)
		}
	}
	return out
}

func (a *App) emitActive() { a.emit("alerts", a.GetActiveAlerts()) }

// ---------- alertas retroativos (histórico do contador) ----------
//
// Quando o histórico baixado traz minutos que o app não viu (estava fechado
// ou desconectado), procura neles os episódios acima de 50 CPM (ou do limite
// do alarme, se for menor) e anota um alerta com a data e a hora em que
// aconteceram. Um episódio termina quando o valor cai abaixo de 45 CPM ou
// quando falta dado por mais de 70 minutos.

const retroGap = 70 * time.Minute

type episode struct {
	start, last, peakAt time.Time
	first, peak         float64
	step                time.Duration
}

func findEpisodes(pts []store.Point, alarmCPM int, lv Levels) []episode {
	enter, exit := lv.attnEnter(), lv.attnExit()
	if alarmCPM > 0 && float64(alarmCPM) < enter {
		enter = float64(alarmCPM)
		exit = enter * 0.9
	}
	var out []episode
	var cur *episode
	var prev time.Time
	for _, p := range pts {
		if cur != nil && p.T.Sub(prev) > retroGap {
			out = append(out, *cur)
			cur = nil
		}
		if cur == nil && p.CPM >= enter {
			cur = &episode{start: p.T, last: p.T, peakAt: p.T, first: p.CPM, peak: p.CPM, step: time.Minute}
		} else if cur != nil {
			if p.CPM < exit {
				out = append(out, *cur)
				cur = nil
			} else {
				if d := p.T.Sub(cur.last); d > 0 && d <= retroGap {
					cur.step = d
				}
				cur.last = p.T
				if p.CPM > cur.peak {
					cur.peak, cur.peakAt = p.CPM, p.T
				}
			}
		}
		prev = p.T
	}
	if cur != nil {
		out = append(out, *cur)
	}
	return out
}

// retroAlerts anota os episódios achados nos pontos novos. Devolve quantos.
func (a *App) retroAlerts(pts []store.Point) int {
	if len(pts) == 0 {
		return 0
	}
	a.mu.Lock()
	alarmCPM := a.settings.AlarmCPM
	lv := a.settings.Levels.norm()
	have := map[int64]bool{}
	for _, e := range a.events {
		if e.Kind == kindAlert && e.Retro {
			have[e.Time] = true
		}
	}
	a.mu.Unlock()
	if alarmCPM <= 0 {
		alarmCPM = warningCPM
	}
	sorted := append([]store.Point(nil), pts...)
	sort.Slice(sorted, func(i, j int) bool { return sorted[i].T.Before(sorted[j].T) })

	n := 0
	for _, ep := range findEpisodes(sorted, alarmCPM, lv) {
		start := ep.start.UnixMilli()
		if have[start] {
			continue
		}
		ev := Event{
			Time: start, Kind: kindAlert, Retro: true,
			CPM: ep.first, Peak: ep.peak, PeakAt: ep.peakAt.UnixMilli(),
			End: ep.last.Add(ep.step).UnixMilli(),
		}
		if ep.peak >= lv.dangerEnter() {
			ev.Level = "perigo"
			ev.Text = fmt.Sprintf("Perigo (histórico do contador): pico de %.0f CPM às %s (a partir de %d)", ep.peak, ep.peakAt.Format("15:04"), lv.DangerCPM)
		} else if ep.peak >= float64(alarmCPM) {
			ev.Level = "alarme"
			ev.Text = fmt.Sprintf("Alarme (histórico do contador): pico de %.0f CPM às %s (limite %d)", ep.peak, ep.peakAt.Format("15:04"), alarmCPM)
		} else {
			ev.Level = "atencao"
			ev.Text = fmt.Sprintf("Atenção (histórico do contador): pico de %.0f CPM às %s (a partir de %d)", ep.peak, ep.peakAt.Format("15:04"), lv.AttentionCPM)
		}
		a.mu.Lock()
		a.evSeq++
		ev.ID = a.evSeq
		a.events = append(a.events, ev)
		a.mu.Unlock()
		have[start] = true
		n++
	}
	if n > 0 {
		a.mu.Lock()
		sort.SliceStable(a.events, func(i, j int) bool { return a.events[i].Time < a.events[j].Time })
		if len(a.events) > maxEvents {
			a.events = a.events[len(a.events)-maxEvents:]
		}
		a.mu.Unlock()
		a.saveEvents()
		a.emit("event", Event{Kind: kindAlert})
	}
	return n
}

// scanRetroOnce procura, uma única vez, alertas nos dados do contador que já
// tinham sido baixados antes desta versão.
func (a *App) scanRetroOnce() {
	if a.store == nil || a.dataDir == "" {
		return
	}
	mark := filepath.Join(a.dataDir, "alertas-retro.ok")
	if _, err := os.Stat(mark); err == nil {
		return
	}
	first := a.store.Earliest()
	if !first.IsZero() {
		pts, err := a.store.Range(first, time.Now().Add(time.Minute))
		if err != nil {
			return
		}
		var dev []store.Point
		for _, p := range pts {
			if p.Src == store.SrcDevice {
				dev = append(dev, p)
			}
		}
		if n := a.retroAlerts(dev); n > 0 {
			a.logEvent(fmt.Sprintf("Histórico já baixado conferido: %d %s", n, plural(n, "alerta encontrado", "alertas encontrados")))
		}
	}
	_ = os.WriteFile(mark, []byte(time.Now().Format(time.RFC3339)+"\n"), 0o644)
}

func plural(n int, one, many string) string {
	if n == 1 {
		return one
	}
	return many
}

// ---------- episódios ----------

// alertEpisode junta os alertas que se encostam no tempo (atenção que virou
// alarme ou perigo e voltou para atenção): um episódio só, com o nível mais
// alto. Os alertas continuam gravados separados; é só para contar e listar.
type alertEpisode struct {
	Level string
	Start int64
	Until int64
	Peak  float64
	IDs   []int64
}

var levelRank = map[string]int{"atencao": 1, "alarme": 2, "perigo": 3}

func alertEpisodes(evs []Event, now time.Time) []alertEpisode {
	list := append([]Event(nil), evs...)
	sort.Slice(list, func(i, j int) bool { return list[i].Time < list[j].Time })
	var out []alertEpisode
	var demo, retro bool
	var dev string
	for _, e := range list {
		end := e.End
		if e.Active {
			end = now.UnixMilli()
		}
		if end < e.Time {
			end = e.Time
		}
		lvl := e.Level
		if lvl == "" {
			lvl = "atencao"
		}
		if n := len(out); n > 0 && e.Demo == demo && e.Retro == retro && e.Device == dev && e.Time <= out[n-1].Until+60000 {
			ep := &out[n-1]
			if levelRank[lvl] > levelRank[ep.Level] {
				ep.Level = lvl
			}
			if end > ep.Until {
				ep.Until = end
			}
			if e.Peak > ep.Peak {
				ep.Peak = e.Peak
			}
			ep.IDs = append(ep.IDs, e.ID)
			continue
		}
		demo, retro, dev = e.Demo, e.Retro, e.Device
		out = append(out, alertEpisode{Level: lvl, Start: e.Time, Until: end, Peak: e.Peak, IDs: []int64{e.ID}})
	}
	return out
}
