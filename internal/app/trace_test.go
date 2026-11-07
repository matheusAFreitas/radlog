package app

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"radlog/internal/gmc"
	"radlog/internal/store"
)

func TestTraceMinutes(t *testing.T) {
	a := newTestApp(t)
	m := time.Now().Truncate(time.Minute).Add(-3 * time.Minute)
	lv := DefaultLevels().norm()
	// sem correção
	a.storeMinute(m, 27, 27, lv)
	// com correção, saturação e interferência no minuto
	lv.DeadTimeOn = true
	a.mu.Lock()
	a.markMinuteLocked(m.Add(time.Minute+10*time.Second), store.ObsSaturation)
	a.markMinuteLocked(m.Add(time.Minute+20*time.Second), store.ObsInterference)
	a.markMinuteLocked(m.Add(time.Minute+30*time.Second), store.ObsSaturation) // repetida
	a.mu.Unlock()
	a.storeMinute(m.Add(time.Minute), 33333, 30000, lv)
	pts, _ := a.store.Range(m, m.Add(5*time.Minute))
	if len(pts) != 2 {
		t.Fatalf("pontos: %+v", pts)
	}
	if p := pts[0]; !p.Has || p.DTOn || p.Raw != 27 || p.Corr != 27 || p.TauUs != 0 || p.Obs != "" {
		t.Fatalf("sem correção: %+v", p)
	}
	if p := pts[1]; !p.DTOn || p.Raw != 30000 || p.Corr != 33333 || p.CPM != 33333 || p.TauUs != 200 || p.Obs != "sat+interf" {
		t.Fatalf("com correção: %+v", p)
	}
	if len(a.minuteObs) != 0 {
		t.Fatalf("marcas velhas deveriam sair: %v", a.minuteObs)
	}
	// histórico do contador: bruto e corrigido, e marca de "veio de segundos"
	base := time.Date(2026, 11, 2, 10, 0, 0, 0, time.Local)
	var recs []gmc.Record
	for s := 1; s <= 60; s++ {
		recs = append(recs, gmc.Record{Time: base.Add(time.Duration(s) * time.Second), Count: 500, Unit: "CPS"})
	}
	recs = append(recs, gmc.Record{Time: base.Add(2 * time.Minute), Count: 25, Unit: "CPM"})
	for _, p := range recordsToPoints(recs, lv) {
		switch {
		case p.T.Equal(base):
			if !p.Has || p.Raw != 30000 || p.Corr != 33333 || !p.HasObs(store.ObsSeconds) || !p.DTOn {
				t.Fatalf("de segundos: %+v", p)
			}
		default:
			if !p.Has || p.Raw != 25 || p.HasObs(store.ObsSeconds) {
				t.Fatalf("CPM: %+v", p)
			}
		}
	}
}

func TestConfigHistory(t *testing.T) {
	a := newTestApp(t)
	a.cfgRecord("app")
	h := a.GetConfigHistory()
	if len(h) != 6 || h[0].Source != "início do registro" || h[0].From != "" {
		t.Fatalf("inicial: %+v", h)
	}
	time.Sleep(3 * time.Millisecond)
	// abrir de novo sem mudar nada: nada novo
	a.cfgRecord("app")
	if len(a.GetConfigHistory()) != 6 {
		t.Fatalf("repetiu o estado inicial")
	}
	// mudar o tempo morto e o alarme pelas Configurações
	s := a.GetSettings()
	s.Levels.DeadTimeOn = true
	s.AlarmCPM = 120
	a.SaveSettings(s)
	h = a.GetConfigHistory()
	if len(h) != 8 {
		t.Fatalf("mudanças: %+v", h[:3])
	}
	got := map[string]ConfigChange{}
	for _, c := range h[:2] {
		got[c.Field] = c
	}
	if c := got["Tempo morto"]; c.From != "desligado" || c.To != "ligado, τ = 200 µs" || c.Source != "você" {
		t.Fatalf("tempo morto: %+v", c)
	}
	if c := got["Alarme"]; c.From != "a partir de 100 CPM" || c.To != "a partir de 120 CPM" {
		t.Fatalf("alarme: %+v", c)
	}
	// calibração nova lida do contador e modo da memória
	a.mu.Lock()
	a.cal = gmc.Calibration{Points: []gmc.CalPoint{{CPM: 1818, USv: 10}}, Source: "contador"}
	a.devSaveMode = "a cada minuto"
	a.mu.Unlock()
	a.cfgRecord("contador")
	h = a.GetConfigHistory()
	if len(h) != 10 || !strings.Contains(h[0].To+h[1].To, "0,0055") {
		t.Fatalf("calibração: %+v", h[:2])
	}
	// backup leva o histórico; outro PC junta sem repetir
	var buf bytes.Buffer
	if err := a.writeBackup(&buf); err != nil {
		t.Fatal(err)
	}
	b := newTestApp(t)
	time.Sleep(3 * time.Millisecond) // (no mesmo milissegundo seriam linhas iguais, e iguais não repetem)
	b.cfgRecord("app")               // o estado inicial do outro PC
	zp := filepath.Join(t.TempDir(), "b.zip")
	os.WriteFile(zp, buf.Bytes(), 0o644)
	for i := 0; i < 2; i++ {
		if _, err := b.mergeZip(zp, true); err != nil {
			t.Fatal(err)
		}
	}
	if n := len(b.GetConfigHistory()); n != 16 {
		t.Fatalf("no outro PC: %d", n)
	}
	// em ordem de tempo no arquivo
	raw, _ := os.ReadFile(b.cfgHistPath())
	var prev int64
	for _, l := range strings.Split(strings.TrimSpace(string(raw)), "\n") {
		var c ConfigChange
		json.Unmarshal([]byte(l), &c)
		if c.T < prev {
			t.Fatalf("fora de ordem")
		}
		prev = c.T
	}
}

func TestAlertSecondsWindow(t *testing.T) {
	a := newTestApp(t)
	now := time.Now().Truncate(time.Second)
	// fundo de 25 CPM por 40 min, alerta das -20 às -15 min (80 CPM), depois volta
	start, end := now.Add(-20*time.Minute), now.Add(-15*time.Minute)
	for s := now.Add(-40 * time.Minute); s.Before(now); s = s.Add(time.Second) {
		v := 25.0
		if !s.Before(start) && s.Before(end) {
			v = 80
		}
		a.live.add(s, v)
	}
	a.mu.Lock()
	a.bgCPM, a.bgOK, a.bgAt = 25, true, time.Now()
	a.mu.Unlock()
	a.addEvent(Event{Time: start.UnixMilli(), End: end.UnixMilli(), Kind: kindAlert, Level: "atencao", CPM: 80, Text: "Atenção: 80 CPM (a partir de 50)"}, true)
	var e Event
	for _, x := range a.GetEvents(kindAlert) {
		if x.ID > e.ID {
			e = x
		}
	}
	a.saveAlertSeconds([]int64{e.ID})
	pts := a.readAlertSeconds(e)
	if len(pts) == 0 {
		t.Fatalf("sem segundos")
	}
	if first := time.UnixMilli(pts[0].T); first.After(start.Add(-alertKeepPre + time.Second)) {
		t.Fatalf("começa em %v, queria 10 min antes de %v", first, start)
	}
	if last := time.UnixMilli(pts[len(pts)-1].T); last.Before(now.Add(-2 * time.Second)) {
		t.Fatalf("termina em %v", last)
	}
	// já voltou ao fundo há mais de 5 min: sai da lista
	a.mu.Lock()
	_, still := a.alertWatch[e.ID]
	a.mu.Unlock()
	if still {
		t.Fatalf("deveria ter terminado")
	}
	// um alerta que acabou agora e a contagem ainda alta: continua gravando
	a.addEvent(Event{Time: now.Add(-2 * time.Minute).UnixMilli(), End: now.UnixMilli(), Kind: kindAlert, Level: "atencao", CPM: 80, Text: "Atenção: 80 CPM (a partir de 50)"}, true)
	for _, x := range a.GetEvents(kindAlert) {
		if x.ID > e.ID {
			e = x
		}
	}
	a.saveAlertSeconds([]int64{e.ID})
	a.mu.Lock()
	_, still = a.alertWatch[e.ID]
	a.mu.Unlock()
	if !still {
		t.Fatalf("acabou de terminar: tem que continuar gravando o depois")
	}
	// gravar de novo não duplica segundos
	n := len(a.readAlertSeconds(e))
	a.alertSecondsTick(time.Now())
	if m := len(a.readAlertSeconds(e)); m != n {
		t.Fatalf("duplicou: %d → %d", n, m)
	}
}
