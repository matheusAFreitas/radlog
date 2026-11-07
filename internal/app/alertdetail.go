package app

import (
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"time"
)

// ---------- gráfico detalhado de um alerta (tela Alertas) ----------
//
// As leituras por segundo só ficam na memória por 1 hora. Para o gráfico de
// um alerta continuar detalhado depois, o app grava os segundos do alerta em
// "alertas-detalhe/" (veja "gravação dos segundos", abaixo).

const (
	alertPre      = 2 * time.Minute // folga em volta do alerta na tela (e no RadiaCode, o que é guardado)
	alertMaxSecs  = 3 * time.Hour   // RadiaCode: alerta muito longo guarda no máximo isto
	alertMaxShown = 1500            // pontos por segundo mandados à tela
)

// AlertPoint é uma leitura no gráfico do alerta.
type AlertPoint struct {
	T   int64   `json:"t"` // ms
	CPM float64 `json:"cpm"`
}

// AlertDetail é o que a tela precisa para desenhar o gráfico de um alerta.
type AlertDetail struct {
	Event     Event        `json:"event"`
	From      int64        `json:"from"`
	To        int64        `json:"to"`
	End       int64        `json:"end"`       // fim do alerta (ou agora, se ativo)
	Seconds   []AlertPoint `json:"seconds"`   // leituras por segundo (pode vir vazio)
	Minutes   []AlertPoint `json:"minutes"`   // um ponto por minuto (sempre que houver)
	Limit     float64      `json:"limit"`     // limite deste alerta (do texto dele)
	Attention float64      `json:"attention"` // níveis atuais, para referência
	Alarm     float64      `json:"alarm"`
	Danger    float64      `json:"danger"`
	Detailed  bool         `json:"detailed"` // tem os segundos de todo o alerta
	// "radiacode": valores, limites e níveis em µSv/h
	Device string `json:"device,omitempty"`
	// o episódio inteiro (alertas que se encostam), com mínimo e média (2.5)
	Episode *AlertEp `json:"episode,omitempty"`

	allSecs []AlertPoint // segundos sem afinar (para a estatística)
}

// AlertEp é o episódio de um alerta.
type AlertEp struct {
	From      int64        `json:"from"`
	To        int64        `json:"to"`
	Level     string       `json:"level"`
	Alerts    int          `json:"alerts"`
	Peak      float64      `json:"peak"`
	Breaks    []alertBreak `json:"breaks"`              // trechos sem leitura no meio
	EndReason string       `json:"endReason,omitempty"` // terminou desconectado
	// estatística das leituras de dentro do episódio
	N      int     `json:"n"`
	Src    string  `json:"src"` // "segundos" ou "minutos"
	Min    float64 `json:"min"`
	Mean   float64 `json:"mean"`
	Median float64 `json:"median"`
	Max    float64 `json:"max"`
}

// episodeOf acha o episódio do alerta (mesmo aparelho, demo e origem).
func (a *App) episodeOf(e Event, now time.Time) (alertEpisode, bool) {
	a.mu.Lock()
	var list []Event
	for _, x := range a.events {
		if x.Kind == kindAlert && x.Device == e.Device && x.Demo == e.Demo && x.Retro == e.Retro {
			c := x
			c.Active = a.isActiveLocked(x.ID)
			list = append(list, c)
		}
	}
	a.mu.Unlock()
	for _, ep := range alertEpisodes(list, now) {
		for _, id := range ep.IDs {
			if id == e.ID {
				return ep, true
			}
		}
	}
	return alertEpisode{}, false
}

// fillEpisode põe no detalhe o episódio e a estatística de dentro dele:
// com os segundos do alerta inteiro (um alerta só), usa os segundos; senão,
// os minutos gravados.
func (a *App) fillEpisode(d *AlertDetail) {
	ep, ok := a.episodeOf(d.Event, time.Now())
	if !ok {
		return
	}
	if d.Event.Active {
		ep.Until = d.End
	}
	out := &AlertEp{From: ep.Start, To: ep.Until, Level: ep.Level, Alerts: len(ep.IDs), Peak: ep.Peak, Breaks: ep.Breaks, EndReason: ep.EndReason}
	if out.Breaks == nil {
		out.Breaks = []alertBreak{}
	}
	var vs []float64
	if len(ep.IDs) == 1 && d.Detailed {
		out.Src = "segundos"
		for _, p := range d.allSecs {
			if p.T >= ep.Start && p.T <= ep.Until {
				vs = append(vs, p.CPM)
			}
		}
	}
	if len(vs) == 0 {
		out.Src = "minutos"
		from, to := time.UnixMilli(ep.Start).Truncate(time.Minute), time.UnixMilli(ep.Until)
		if d.Device == devRC {
			for _, p := range a.rcRange(from, to) {
				vs = append(vs, p.USvh)
			}
		} else if pts, err := a.points(from, to); err == nil {
			for _, p := range pts {
				vs = append(vs, p.CPM)
			}
		}
	}
	if len(vs) > 0 {
		sort.Float64s(vs)
		var sum float64
		for _, v := range vs {
			sum += v
		}
		out.N, out.Min, out.Max = len(vs), vs[0], vs[len(vs)-1]
		out.Mean, out.Median = sum/float64(len(vs)), percentile(vs, .5)
	}
	d.Episode = out
}

func (a *App) alertDir() string { return filepath.Join(a.dataDir, "alertas-detalhe") }

// alertKey é o nome do arquivo dos segundos do alerta. Usa a hora em
// segundos: o eventos.csv grava a hora sem os milissegundos, então depois de
// reabrir o app a hora do alerta perde os ms (até a 0.23.0 o nome usava ms e
// o arquivo não era mais achado).
func alertKey(e Event) string { return fmt.Sprintf("%d-%s.json", e.Time/1000, e.Level) }

// readAlertSeconds lê os segundos gravados do alerta (nome novo ou o antigo,
// com ms, que começa com os mesmos dígitos).
func (a *App) readAlertSeconds(e Event) []AlertPoint {
	names := []string{filepath.Join(a.alertDir(), alertKey(e))}
	if old, _ := filepath.Glob(filepath.Join(a.alertDir(), fmt.Sprintf("%d???-%s.json", e.Time/1000, e.Level))); len(old) > 0 {
		names = append(names, old...)
	}
	for _, n := range names {
		b, err := os.ReadFile(n)
		if err != nil {
			continue
		}
		var pts []AlertPoint
		if json.Unmarshal(b, &pts) == nil && len(pts) > 0 {
			return pts
		}
	}
	return nil
}

var limitRe = regexp.MustCompile(`(?:a partir de|acima de|limite) (\d+)`)

func alertLimit(e Event, lv Levels, alarm int) float64 {
	if m := limitRe.FindStringSubmatch(e.Text); m != nil {
		v, _ := strconv.Atoi(m[1])
		return float64(v)
	}
	switch e.Level {
	case "perigo":
		return float64(lv.DangerCPM)
	case "alarme":
		return float64(alarm)
	}
	return float64(lv.AttentionCPM)
}

// ---------- gravação dos segundos (2.1) ----------
//
// Enquanto um alerta do GMC está aberto, o app grava os segundos dele de
// minuto em minuto (juntando com o que já gravou, porque a memória só guarda
// 1 hora): desde 10 min antes do disparo até a contagem voltar ao fundo
// (dentro de 2σ por 5 min seguidos), no máximo 30 min depois do fim.

const (
	alertKeepPre  = 10 * time.Minute // segundos guardados antes do disparo
	alertKeepPost = 30 * time.Minute // depois do fim: até voltar ao fundo, no máximo isto
	alertCalmFor  = 5 * time.Minute  // "voltou ao fundo": tanto tempo dentro de 2σ
	alertKeepMax  = 8 * time.Hour    // alerta muito longo: guarda no máximo isto
)

// saveAlertSeconds é chamado quando alertas do GMC terminam: grava já e
// continua gravando o "depois" até a contagem voltar ao fundo.
func (a *App) saveAlertSeconds(ids []int64) {
	if a.dataDir == "" {
		return
	}
	a.mu.Lock()
	if a.alertWatch == nil {
		a.alertWatch = map[int64]bool{}
	}
	for _, id := range ids {
		a.alertWatch[id] = true
	}
	a.mu.Unlock()
	a.alertSecondsTick(time.Now())
}

// alertSecondsLoop grava os segundos dos alertas abertos (e dos que acabaram
// de terminar) a cada minuto.
func (a *App) alertSecondsLoop() {
	t := time.NewTicker(time.Minute)
	defer t.Stop()
	for range t.C {
		if a.closing.Load() {
			return
		}
		a.alertSecondsTick(time.Now())
	}
}

// alertSecondsTick grava o que há de novo e diz adeus aos alertas cuja
// contagem já voltou ao fundo.
func (a *App) alertSecondsTick(now time.Time) {
	if a.dataDir == "" {
		return
	}
	bg, bgOK := a.backgroundCPM()
	a.mu.Lock()
	if a.alertWatch == nil {
		a.alertWatch = map[int64]bool{}
	}
	for _, id := range []int64{a.alerts.attn, a.alerts.alarm, a.alerts.danger} {
		if id != 0 {
			a.alertWatch[id] = true
		}
	}
	var evs []Event
	for id := range a.alertWatch {
		e := a.eventLocked(id)
		if e == nil || e.Demo || e.Device == devRC {
			delete(a.alertWatch, id)
			continue
		}
		c := *e
		c.Active = a.isActiveLocked(id)
		evs = append(evs, c)
	}
	thr := a.settings.Levels.norm().attnExit()
	a.mu.Unlock()
	if bgOK && bg > 0 {
		thr = bg + 2*math.Sqrt(bg)
	}
	for _, e := range evs {
		from := time.UnixMilli(e.Time).Add(-alertKeepPre)
		a.writeAlertSeconds(e, from, now)
		if e.Active || e.End == 0 {
			continue
		}
		end := time.UnixMilli(e.End)
		done := now.Sub(end) >= alertKeepPost
		if !done && now.Sub(end) >= alertCalmFor {
			a.mu.Lock()
			ss := a.live.between(now.Add(-alertCalmFor), now)
			a.mu.Unlock()
			calm := true
			for _, s := range ss {
				if s.cpm > thr {
					calm = false
					break
				}
			}
			// sem leitura (desconectou): não há mais o que gravar
			done = calm || len(ss) == 0
		}
		if done {
			a.mu.Lock()
			delete(a.alertWatch, e.ID)
			a.mu.Unlock()
		}
	}
}

// writeAlertSeconds junta os segundos da memória em [from, to) com os já
// gravados do alerta e grava.
func (a *App) writeAlertSeconds(e Event, from, to time.Time) {
	a.mu.Lock()
	ss := a.live.between(from, to)
	a.mu.Unlock()
	old := a.readAlertSeconds(e)
	if len(ss) == 0 && len(old) == 0 {
		return
	}
	seen := make(map[int64]bool, len(old)+len(ss))
	pts := make([]AlertPoint, 0, len(old)+len(ss))
	for _, p := range old {
		if !seen[p.T] {
			seen[p.T] = true
			pts = append(pts, p)
		}
	}
	for _, s := range ss {
		if t := s.t.UnixMilli(); !seen[t] {
			seen[t] = true
			pts = append(pts, AlertPoint{t, s.cpm})
		}
	}
	sort.Slice(pts, func(i, j int) bool { return pts[i].T < pts[j].T })
	if n := len(pts); n > 0 {
		cut := pts[n-1].T - alertKeepMax.Milliseconds()
		i := sort.Search(n, func(i int) bool { return pts[i].T >= cut })
		pts = pts[i:]
	}
	b, err := json.Marshal(pts)
	if err != nil {
		return
	}
	_ = os.MkdirAll(a.alertDir(), 0o755)
	path := filepath.Join(a.alertDir(), alertKey(e))
	if os.WriteFile(path+".tmp", b, 0o644) == nil {
		os.Rename(path+".tmp", path)
	}
}

// GetAlertDetail monta o gráfico do alerta de id dado.
func (a *App) GetAlertDetail(id int64) (AlertDetail, error) {
	a.mu.Lock()
	ep := a.eventLocked(id)
	if ep == nil || ep.Kind != kindAlert {
		a.mu.Unlock()
		return AlertDetail{}, errors.New("alerta não encontrado")
	}
	e := *ep
	e.Active = a.isActiveLocked(e.ID)
	lv, alarm := a.settings.Levels, a.settings.AlarmCPM
	a.mu.Unlock()

	now := time.Now()
	start := time.UnixMilli(e.Time)
	end := now
	if e.End != 0 && !e.Active {
		end = time.UnixMilli(e.End)
	}
	if e.Device == devRC {
		d := a.rcAlertDetail(e, end)
		a.fillEpisode(&d)
		return d, nil
	}
	d := AlertDetail{
		Event: e, End: end.UnixMilli(),
		Limit: alertLimit(e, lv, alarm), Attention: float64(lv.AttentionCPM),
		Alarm: float64(alarm), Danger: float64(lv.DangerCPM),
	}

	// segundos: arquivo gravado; senão, a memória (última hora) ou o demo
	var secs []AlertPoint
	if !e.Demo && a.dataDir != "" {
		secs = a.readAlertSeconds(e)
	}
	if len(secs) == 0 {
		for _, s := range a.liveSamples(start.Add(-alertPre), end.Add(alertPre)) {
			secs = append(secs, AlertPoint{s.t.UnixMilli(), s.cpm})
		}
	}

	// janela: com segundos, 2 min em volta (ou 1/4 do alerta, até 30 min);
	// só com minutos, mais contexto (10 min a 2 h)
	dur := end.Sub(start)
	pad := dur / 4
	if len(secs) > 0 {
		pad = min(max(pad, alertPre), 30*time.Minute)
	} else {
		pad = min(max(dur, 10*time.Minute), 2*time.Hour)
	}
	from, to := start.Add(-pad), end.Add(pad)
	if to.After(now) {
		to = now
	}
	d.From, d.To = from.UnixMilli(), to.UnixMilli()

	var in []AlertPoint
	for _, p := range secs {
		if p.T >= d.From && p.T < d.To {
			in = append(in, p)
		}
	}
	if len(in) > 0 {
		d.Detailed = in[0].T <= e.Time+5000 && in[len(in)-1].T >= d.End-5000
	}
	d.Seconds = thinMax(in, alertMaxShown)
	d.allSecs = in

	pts, err := a.points(from.Truncate(time.Minute), to)
	if err != nil {
		return d, err
	}
	for _, p := range pts {
		d.Minutes = append(d.Minutes, AlertPoint{p.T.UnixMilli(), p.CPM})
	}
	if d.Minutes == nil {
		d.Minutes = []AlertPoint{}
	}
	if d.Seconds == nil {
		d.Seconds = []AlertPoint{}
	}
	a.fillEpisode(&d)
	return d, nil
}

// thinMax reduz a lista para no máximo n pontos sem perder os picos.
func thinMax(in []AlertPoint, n int) []AlertPoint {
	if len(in) <= n {
		return in
	}
	k := (len(in) + n - 1) / n
	out := make([]AlertPoint, 0, n)
	for i := 0; i < len(in); i += k {
		best := in[i]
		for j := i + 1; j < i+k && j < len(in); j++ {
			if in[j].CPM > best.CPM {
				best = in[j]
			}
		}
		out = append(out, best)
	}
	return out
}

// ChartPeak é o pico de um alerta, marcado como ponto no gráfico.
type ChartPeak struct {
	T     int64   `json:"t"` // ms
	CPM   float64 `json:"cpm"`
	Level string  `json:"level"`
	Retro bool    `json:"retro"`
}

// alertPeaks devolve os picos dos alertas em [from, to). O CPM do contador é
// a soma dos últimos 60 s, recalculada a cada segundo; o gráfico guarda o
// valor de cada minuto do relógio (de :00 a :59). O pico do alerta é a maior
// leitura de segundo e pode ficar um pouco acima do valor do minuto: ele entra
// no "máx" e aparece como ponto, para os números baterem.
func (a *App) alertPeaks(from, to time.Time) []ChartPeak {
	f, t := from.UnixMilli(), to.UnixMilli()
	rank := map[string]int{"atencao": 1, "alarme": 2, "perigo": 3}
	a.mu.Lock()
	demo := a.demo.Active
	byAt := map[int64]int{}
	var out []ChartPeak
	for _, e := range a.events {
		if e.Kind != kindAlert || e.Peak <= 0 || (e.Demo && !demo) || e.Device == devRC {
			continue
		}
		at := e.PeakAt
		if at == 0 {
			at = e.Time
		}
		if at < f || at >= t {
			continue
		}
		// atenção e alarme do mesmo pico: um ponto só, com o nível mais alto
		if i, ok := byAt[at]; ok {
			if rank[e.Level] > rank[out[i].Level] {
				out[i].Level = e.Level
			}
			out[i].CPM = max(out[i].CPM, e.Peak)
			continue
		}
		byAt[at] = len(out)
		out = append(out, ChartPeak{T: at, CPM: e.Peak, Level: e.Level, Retro: e.Retro})
	}
	a.mu.Unlock()
	if out == nil {
		out = []ChartPeak{}
	}
	return out
}

// alertSamples devolve as leituras por segundo gravadas dos alertas que
// caem em [from, to), em ordem.
func (a *App) alertSamples(from, to time.Time) []sample {
	if a.dataDir == "" {
		return nil
	}
	f, t := from.UnixMilli(), to.UnixMilli()
	a.mu.Lock()
	if a.demo.Active {
		a.mu.Unlock()
		return nil
	}
	var evs []Event
	for _, e := range a.events {
		if e.Kind != kindAlert || e.Demo || e.Retro || e.End == 0 || e.Device == devRC {
			continue
		}
		if e.End+alertKeepPost.Milliseconds() > f && e.Time-alertKeepPre.Milliseconds() < t {
			evs = append(evs, e)
		}
	}
	a.mu.Unlock()
	var out []sample
	seen := map[int64]bool{}
	for _, e := range evs {
		for _, p := range a.readAlertSeconds(e) {
			if p.T >= f && p.T < t && !seen[p.T] {
				seen[p.T] = true
				out = append(out, sample{time.UnixMilli(p.T), p.CPM})
			}
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].t.Before(out[j].t) })
	return out
}

// fineSamples junta as leituras por segundo da memória (última hora) com as
// gravadas dos alertas (mais antigas), sem repetir.
func (a *App) fineSamples(from, to time.Time) []sample {
	live := a.liveSamples(from, to)
	old := a.alertSamples(from, to)
	if len(old) == 0 {
		return live
	}
	if len(live) == 0 {
		return old
	}
	first := live[0].t
	out := make([]sample, 0, len(old)+len(live))
	for _, s := range old {
		if s.t.Before(first) {
			out = append(out, s)
		}
	}
	return append(out, live...)
}
