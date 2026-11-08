package app

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"

	"radlog/internal/core/levels"
)

// ---------- alertas do RadiaCode ----------
//
// Os mesmos três níveis do GMC, em µSv/h, com os limites do próprio aparelho:
//   - atenção: alarme 1 do RadiaCode (DR_LEV1);
//   - alarme:  alarme 2 do RadiaCode (DR_LEV2);
//   - perigo:  o perigo do app, convertido pela calibração do GMC.
// Sem os limites do aparelho (não deu para ler), usa as faixas do app
// convertidas. Sai do nível abaixo de 90% do limite, como no GMC. Os alertas
// ficam no mesmo histórico, marcados com Device = "radiacode"; neles CPM e
// Peak guardam µSv/h.

const devRC = "radiacode"

// rcAlertState são os alertas abertos do RadiaCode (protegido por a.mu).
type rcAlertState struct {
	ids                     alertIDs
	attnOn, alarmOn, dangOn bool
}

// RcLimits são os limites dos alertas do RadiaCode, em µSv/h.
type RcLimits struct {
	Attn   float64 `json:"attn"`
	Alarm  float64 `json:"alarm"`
	Danger float64 `json:"danger"`
	Device bool    `json:"device"` // atenção e alarme vêm do aparelho
}

// rcLimitsLocked calcula os limites (precisa de a.rc.mu; lê a.mu por dentro).
func (a *App) rcLimits() RcLimits {
	a.mu.Lock()
	cal := a.convCalLocked()
	lv := a.settings.Levels.Norm()
	alarmCPM := a.settings.AlarmCPM
	a.mu.Unlock()
	if alarmCPM <= 0 {
		alarmCPM = levels.WarningCPM
	}
	l := RcLimits{Attn: cal.USv(lv.AttnEnter()), Alarm: cal.USv(float64(alarmCPM)), Danger: cal.USv(lv.DangerEnter())}
	a.rc.mu.Lock()
	al := a.rc.st.Alarms
	a.rc.mu.Unlock()
	if al != nil && al.DoseRate1 > 0 && al.DoseRate2 > al.DoseRate1 {
		l.Attn, l.Alarm, l.Device = al.DoseRate1, al.DoseRate2, true
	}
	if l.Danger <= l.Alarm {
		l.Danger = l.Alarm * 10
	}
	return l
}

// usvTxt escreve µSv/h com casas suficientes (0,452 · 1,31 · 12,3).
func usvTxt(v float64) string {
	switch {
	case v >= 100:
		return fmtBR(v, 0)
	case v >= 10:
		return fmtBR(v, 1)
	case v >= 1:
		return fmtBR(v, 2)
	}
	return fmtBR(v, 3)
}

// rcAlertText é o texto do alerta do RadiaCode.
func rcAlertText(level string, usv float64, l RcLimits) string {
	switch level {
	case "perigo":
		return fmt.Sprintf("Perigo no RadiaCode: %s µSv/h (a partir de %s)", usvTxt(usv), usvTxt(l.Danger))
	case "alarme":
		if l.Device {
			return fmt.Sprintf("Alarme no RadiaCode: %s µSv/h (alarme 2 do aparelho: %s)", usvTxt(usv), usvTxt(l.Alarm))
		}
		return fmt.Sprintf("Alarme no RadiaCode: %s µSv/h (limite %s)", usvTxt(usv), usvTxt(l.Alarm))
	}
	if l.Device {
		return fmt.Sprintf("Atenção no RadiaCode: %s µSv/h (alarme 1 do aparelho: %s)", usvTxt(usv), usvTxt(l.Attn))
	}
	return fmt.Sprintf("Atenção no RadiaCode: %s µSv/h (a partir de %s)", usvTxt(usv), usvTxt(l.Attn))
}

// rcAlertStep recebe cada leitura ao vivo do RadiaCode (µSv/h).
func (a *App) rcAlertStep(now time.Time, usv float64) {
	l := a.rcLimits()
	a.mu.Lock()
	set := a.settings
	s := &a.rcAlert
	step := func(on *bool, enabled bool, enter float64) (started, ended bool) {
		was := *on
		switch {
		case !enabled:
			*on = false
		case usv >= enter:
			*on = true
		case usv < enter*0.9:
			*on = false
		}
		return *on && !was, was && !*on
	}
	attnStarted, attnEnded := step(&s.attnOn, set.AttentionEnabled, l.Attn)
	alarmStarted, alarmEnded := step(&s.alarmOn, set.AlarmEnabled, l.Alarm)
	dangStarted, dangEnded := step(&s.dangOn, true, l.Danger)
	attnOn, alarmOn := s.attnOn, s.alarmOn
	a.mu.Unlock()

	a.stepAlerts(&a.rcAlert.ids, alertStep{
		Now: now, CPM: usv, Device: devRC, RcLim: l,
		AttnStarted: attnStarted, AttnEnded: attnEnded, AttnOn: attnOn,
		AlarmStarted: alarmStarted, AlarmEnded: alarmEnded, AlarmOn: alarmOn,
		DangerStarted: dangStarted, DangerEnded: dangEnded,
	})
	val := usvTxt(usv) + " µSv/h no RadiaCode"
	switch {
	case dangStarted:
		a.flagWindowText("PERIGO", val, false, true)
	case alarmStarted:
		a.mu.Lock()
		raise := a.settings.AlarmRaise
		a.mu.Unlock()
		a.flagWindowText("ALARME", val, false, raise)
	}
	if dangEnded && alarmOn {
		a.flagWindowText("ALARME", val, false, false)
	}
	if alarmEnded || dangEnded {
		a.alarmEnded()
	}
}

// rcAlertLevel é o nível mais alto aberto agora ("" = nenhum).
func (a *App) rcAlertLevelLocked() string {
	s := a.rcAlert
	switch {
	case s.dangOn:
		return "perigo"
	case s.alarmOn:
		return "alarme"
	case s.attnOn:
		return "atencao"
	}
	return ""
}

// rcAlertsOff fecha os alertas do RadiaCode (desconectou).
func (a *App) rcAlertsOff() {
	a.mu.Lock()
	a.rcAlert.attnOn, a.rcAlert.alarmOn, a.rcAlert.dangOn = false, false, false
	a.mu.Unlock()
	reason := endRcDisconnected
	if a.closing.Load() {
		reason = endAppClosed
	}
	a.closeAlerts(&a.rcAlert.ids, time.Now(), false, reason)
	a.alarmEnded()
}

// rcAlertKey é o arquivo dos segundos de um alerta do RadiaCode.
func rcAlertKey(e Event) string { return fmt.Sprintf("%d-%s-rc.json", e.Time/1000, e.Level) }

// rcAlertPoints são as leituras ao vivo do RadiaCode (µSv/h) entre from e to.
func (a *App) rcAlertPoints(from, to time.Time) []AlertPoint {
	a.rc.mu.Lock()
	defer a.rc.mu.Unlock()
	var out []AlertPoint
	for _, s := range a.rc.ring {
		if !s.t.Before(from) && s.t.Before(to) {
			out = append(out, AlertPoint{T: s.t.UnixMilli(), CPM: s.usv})
		}
	}
	return out
}

// saveRcAlertSeconds grava as leituras do alerta do RadiaCode (como o
// saveAlertSeconds do GMC).
func (a *App) saveRcAlertSeconds(evs []Event) {
	write := func() {
		for _, e := range evs {
			from := time.UnixMilli(e.Time).Add(-alertPre)
			to := time.UnixMilli(e.End).Add(alertPre)
			if to.Sub(from) > alertMaxSecs {
				from = to.Add(-alertMaxSecs)
			}
			pts := a.rcAlertPoints(from, to)
			if len(pts) == 0 {
				continue
			}
			if b, err := json.Marshal(pts); err == nil {
				_ = os.MkdirAll(a.alertDir(), 0o755)
				_ = os.WriteFile(filepath.Join(a.alertDir(), rcAlertKey(e)), b, 0o644)
			}
		}
	}
	write()
	time.AfterFunc(alertPre+5*time.Second, write)
}

func (a *App) readRcAlertSeconds(e Event) []AlertPoint {
	b, err := os.ReadFile(filepath.Join(a.alertDir(), rcAlertKey(e)))
	if err != nil {
		return nil
	}
	var pts []AlertPoint
	if json.Unmarshal(b, &pts) != nil {
		return nil
	}
	return pts
}

// rcAlertDetail monta o gráfico de um alerta do RadiaCode (µSv/h).
func (a *App) rcAlertDetail(e Event, end time.Time) AlertDetail {
	l := a.rcLimits()
	now := time.Now()
	start := time.UnixMilli(e.Time)
	d := AlertDetail{Event: e, End: end.UnixMilli(), Device: devRC,
		Attention: l.Attn, Alarm: l.Alarm, Danger: l.Danger, Limit: rcAlertLimit(e, l)}
	var secs []AlertPoint
	if !e.Demo && a.dataDir != "" {
		secs = a.readRcAlertSeconds(e)
	}
	if len(secs) == 0 {
		secs = a.rcAlertPoints(start.Add(-alertPre), end.Add(alertPre))
	}
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
	d.Minutes = []AlertPoint{}
	for _, p := range a.rcRange(from.Truncate(time.Minute), to) {
		d.Minutes = append(d.Minutes, AlertPoint{p.T.UnixMilli(), p.USvh})
	}
	if d.Seconds == nil {
		d.Seconds = []AlertPoint{}
	}
	return d
}

var rcLimitRe = regexp.MustCompile(`([0-9]+(?:,[0-9]+)?)\)$`)

// rcAlertLimit tira o limite do texto do alerta (o da hora); senão, o atual.
func rcAlertLimit(e Event, l RcLimits) float64 {
	if m := rcLimitRe.FindStringSubmatch(e.Text); m != nil {
		if v, err := strconv.ParseFloat(strings.Replace(m[1], ",", ".", 1), 64); err == nil && v > 0 {
			return v
		}
	}
	switch e.Level {
	case "perigo":
		return l.Danger
	case "alarme":
		return l.Alarm
	}
	return l.Attn
}
