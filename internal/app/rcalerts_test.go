package app

import (
	"encoding/csv"
	"os"
	"strings"
	"testing"
	"time"

	"radlog/internal/radiacode"
)

func TestRcAlerts(t *testing.T) {
	a := newTestApp(t)
	a.settings.AttentionEnabled, a.settings.AlarmEnabled = true, true
	a.rc.st.Alarms = &radiacode.AlarmLimits{DoseRate1: 0.4, DoseRate2: 1.2}
	l := a.rcLimits()
	if !l.Device || l.Attn != 0.4 || l.Alarm != 1.2 || l.Danger <= l.Alarm {
		t.Fatalf("limites: %+v", l)
	}
	t0 := time.Now().Add(-10 * time.Minute)
	for i, v := range []float64{0.15, 0.45, 0.6, 1.5, 0.9, 0.39, 0.35, 0.2} {
		now := t0.Add(time.Duration(i) * 10 * time.Second)
		a.rc.mu.Lock()
		a.rc.ring = append(a.rc.ring, rcSample{now, v, v * 80})
		a.rc.mu.Unlock()
		a.rcAlertStep(now, v)
		if i == 3 {
			a.mu.Lock()
			lvl := a.rcAlertLevelLocked()
			a.mu.Unlock()
			if lvl != "alarme" || len(a.GetActiveAlerts()) != 2 {
				t.Fatalf("no pico: %q %+v", lvl, a.GetActiveAlerts())
			}
		}
	}
	if act := a.GetActiveAlerts(); len(act) != 0 {
		t.Fatalf("ficou ativo: %+v", act)
	}
	var att, alm *Event
	a.mu.Lock()
	for i := range a.events {
		e := &a.events[i]
		if e.Kind != kindAlert {
			continue
		}
		if e.Device != devRC {
			t.Fatalf("sem aparelho: %+v", e)
		}
		switch e.Level {
		case "atencao":
			att = e
		case "alarme":
			alm = e
		}
	}
	a.mu.Unlock()
	if att == nil || alm == nil || att.Peak != 1.5 || !strings.Contains(att.Text, "alarme 1 do aparelho: 0,400") || !strings.Contains(alm.Text, "Alarme no RC102: 1,50 µSv/h") || alm.End == 0 {
		t.Fatalf("alertas: %+v %+v", att, alm)
	}
	// 0,39 ainda está acima de 90% de 0,40: a atenção só fecha no 0,35
	if att.End != t0.Add(60*time.Second).UnixMilli() {
		t.Fatalf("fim da atenção: %v", time.UnixMilli(att.End).Sub(t0))
	}
	// o CSV guarda o aparelho
	f, _ := os.Open(a.eventsPath())
	rows, _ := csv.NewReader(f).ReadAll()
	f.Close()
	found := false
	for _, r := range rows {
		if e, ok := parseEventRow(r); ok && e.Kind == kindAlert {
			found = true
			if e.Device != devRC {
				t.Fatalf("csv: %v", r)
			}
		}
	}
	if !found {
		t.Fatal("csv sem alertas")
	}
	// detalhe em µSv/h
	d, err := a.GetAlertDetail(alm.ID)
	if err != nil || d.Device != devRC || d.Limit != 1.2 || len(d.Seconds) == 0 || d.Alarm != 1.2 {
		t.Fatalf("detalhe: %+v %v", d, err)
	}
	if v := rcAlertLimit(Event{Text: "Perigo no RC102: 12,3 µSv/h (a partir de 10,1)"}, l); v != 10.1 {
		t.Fatalf("limite do texto: %v", v)
	}
	// alertas do Radiacode não entram nos picos do gráfico do GMC
	if p := a.alertPeaks(t0.Add(-time.Hour), time.Now()); len(p) != 0 {
		t.Fatalf("picos: %+v", p)
	}
	// desligar o alerta de atenção: só alarme
	a.settings.AttentionEnabled = false
	a.rcAlertStep(time.Now(), 0.5)
	if len(a.GetActiveAlerts()) != 0 {
		t.Fatal("atenção desligada abriu alerta")
	}
}
