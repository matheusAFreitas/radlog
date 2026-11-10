package app

import (
	"encoding/json"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"testing"
	"time"

	"radlog/internal/gmc"
	"radlog/internal/store"
)

func TestAlertDetail(t *testing.T) {
	a := NewApp()
	a.dataDir = t.TempDir()
	a.settings = defaultSettings()
	st, err := store.Open(a.dataDir)
	if err != nil {
		t.Fatal(err)
	}
	a.store = st

	// 10 min de leituras por segundo; alerta de 4 min no meio, pico 130
	t0 := time.Now().Truncate(time.Second).Add(-10 * time.Minute)
	for i := 0; i < 600; i++ {
		v := 25.0
		if i >= 200 && i < 440 {
			v = 110
		}
		if i == 300 {
			v = 130
		}
		a.live.add(t0.Add(time.Duration(i)*time.Second), v)
	}
	var mins []store.Point
	for m := 0; m < 10; m++ {
		mins = append(mins, store.Point{T: t0.Truncate(time.Minute).Add(time.Duration(m) * time.Minute), CPM: 30})
	}
	if _, err := st.ImportNew(mins); err != nil {
		t.Fatal(err)
	}
	start := t0.Add(200 * time.Second)
	id := a.addEvent(Event{Time: start.Add(123 * time.Millisecond).UnixMilli(), Kind: kindAlert, Level: "alarme", CPM: 110, Peak: 130,
		PeakAt: t0.Add(300 * time.Second).UnixMilli(), Text: "Alarme: 110 CPM (limite 100)"}, true)
	a.finishAlerts([]int64{id}, t0.Add(440*time.Second), false)

	key := filepath.Join(a.alertDir(), alertKey(*a.eventLocked(id)))
	if _, err := os.Stat(key); err != nil {
		t.Fatalf("não gravou os segundos do alerta: %v", err)
	}

	d, err := a.GetAlertDetail(id)
	if err != nil {
		t.Fatal(err)
	}
	k := gmc.DefaultCalibration().USv(100) / 100 // µSv/h por CPM (reta)
	if d.Unit != "usv" || d.LimitCPM != 100 || math.Abs(d.Limit-100*k) > 1e-9 || !d.Detailed || len(d.Seconds) < 300 || len(d.Minutes) == 0 {
		t.Fatalf("detalhe: limite %v detalhado %v segundos %d minutos %d", d.Limit, d.Detailed, len(d.Seconds), len(d.Minutes))
	}
	peak := 0.0
	for _, p := range d.Seconds {
		peak = max(peak, p.CPM)
	}
	if math.Abs(peak-130*k) > 1e-9 || math.Abs(d.Event.PeakUSv-130*k) > 1e-9 {
		t.Fatalf("pico %v", peak)
	}
	if d.From > start.Add(-2*time.Minute+time.Second).UnixMilli() {
		t.Fatal("janela sem o antes do alerta")
	}

	// depois de reabrir o app (memória vazia), continua detalhado pelo arquivo
	a.live.reset()
	d2, err := a.GetAlertDetail(id)
	if err != nil || !d2.Detailed || len(d2.Seconds) < 300 {
		t.Fatalf("sem o arquivo: %v %v %d", err, d2.Detailed, len(d2.Seconds))
	}
	// reabrindo o app de verdade: os eventos voltam do CSV sem os ms
	b := NewApp()
	b.dataDir, b.store, b.settings = a.dataDir, a.store, a.settings
	b.events = b.loadEvents()
	var rid2 int64
	for _, e := range b.events {
		if e.Level == "alarme" {
			rid2 = e.ID
		}
	}
	d4, err := b.GetAlertDetail(rid2)
	if err != nil || len(d4.Seconds) < 300 {
		t.Fatalf("depois de reabrir: %v, %d segundos", err, len(d4.Seconds))
	}
	// arquivo com o nome antigo (ms) também é achado
	ev := *a.eventLocked(id)
	data, _ := os.ReadFile(filepath.Join(a.alertDir(), alertKey(ev)))
	_ = os.Remove(filepath.Join(a.alertDir(), alertKey(ev)))
	_ = os.WriteFile(filepath.Join(a.alertDir(), fmt.Sprintf("%d-%s.json", ev.Time, ev.Level)), data, 0o644)
	if d5, _ := b.GetAlertDetail(rid2); len(d5.Seconds) < 300 {
		t.Fatal("não achou o arquivo com o nome antigo")
	}

	// alerta do contador (só minutos): janela maior, sem segundos
	old := time.Now().Add(-48 * time.Hour).Truncate(time.Minute)
	rid := a.addEvent(Event{Time: old.UnixMilli(), End: old.Add(time.Minute).UnixMilli(), Kind: kindAlert, Level: "atencao",
		Peak: 52, Retro: true, Text: "Atenção: 52 CPM (a partir de 50)"}, true)
	d3, err := a.GetAlertDetail(rid)
	if err != nil {
		t.Fatal(err)
	}
	if len(d3.Seconds) != 0 || d3.Detailed || d3.LimitCPM != 50 || d3.To-d3.From < (20*time.Minute).Milliseconds() {
		t.Fatalf("retro: %+v", d3)
	}
	if _, err := a.GetAlertDetail(99999); err == nil {
		t.Fatal("deveria dar erro")
	}
}

func TestThinMaxKeepsPeak(t *testing.T) {
	var in []AlertPoint
	for i := 0; i < 5000; i++ {
		in = append(in, AlertPoint{int64(i), 20})
	}
	in[3333].CPM = 999
	out := thinMax(in, 1500)
	if len(out) > 1500 {
		t.Fatalf("%d pontos", len(out))
	}
	found := false
	for _, p := range out {
		found = found || p.CPM == 999
	}
	if !found {
		t.Fatal("perdeu o pico")
	}
}

// O pico do alerta (leitura de segundo) entra no máximo de 24 h e no "máx"
// do gráfico, para bater com o que o alerta diz.
func TestAlertPeakInStats(t *testing.T) {
	a, _ := testApp(t)
	now := time.Now()
	m := now.Truncate(time.Minute).Add(-30 * time.Minute)
	if _, err := a.store.ImportNew([]store.Point{{T: m, CPM: 51}, {T: m.Add(time.Minute), CPM: 30}}); err != nil {
		t.Fatal(err)
	}
	peakAt := m.Add(23 * time.Second).UnixMilli()
	a.addEvent(Event{Time: m.UnixMilli(), End: m.Add(time.Minute).UnixMilli(), Kind: kindAlert, Level: "atencao", CPM: 50, Peak: 52, PeakAt: peakAt, Text: "Atenção: 50 CPM (a partir de 50)"}, true)
	a.addEvent(Event{Time: m.UnixMilli(), End: m.Add(time.Minute).UnixMilli(), Kind: kindAlert, Level: "alarme", CPM: 50, Peak: 52, PeakAt: peakAt, Text: "Alarme: 50 CPM (limite 50)"}, true)
	s, err := a.GetStats()
	if err != nil {
		t.Fatal(err)
	}
	if s.Max24h == nil || *s.Max24h != 52 || s.Max24hAt != peakAt {
		t.Fatalf("máximo 24 h: %v às %v", s.Max24h, s.Max24hAt)
	}
	c, err := a.GetChartRange(m.Add(-10*time.Minute).UnixMilli(), m.Add(10*time.Minute).UnixMilli())
	if err != nil {
		t.Fatal(err)
	}
	if c.Max == nil || *c.Max != 52 || len(c.Peaks) != 1 || c.Peaks[0].Level != "alarme" {
		t.Fatalf("gráfico: máx %v picos %+v", c.Max, c.Peaks)
	}
}

// Zoom num trecho antigo com alerta: o gráfico usa os segundos gravados do
// alerta (passo < 1 min) e o resto fica com o valor do minuto.
func TestOldZoomUsesAlertSeconds(t *testing.T) {
	a, _ := testApp(t)
	base := time.Now().Add(-5 * time.Hour).Truncate(time.Minute)
	var mins []store.Point
	for m := -20; m < 20; m++ {
		mins = append(mins, store.Point{T: base.Add(time.Duration(m) * time.Minute), CPM: 30})
	}
	if _, err := a.store.ImportNew(mins); err != nil {
		t.Fatal(err)
	}
	// segundos gravados do alerta (como o app grava ao terminar)
	start := base.Add(10 * time.Second)
	id := a.addEvent(Event{Time: start.UnixMilli(), End: start.Add(5 * time.Second).UnixMilli(), Kind: kindAlert, Level: "atencao",
		CPM: 50, Peak: 50, PeakAt: start.UnixMilli(), Text: "Atenção: 50 CPM (a partir de 50)"}, true)
	var pts []AlertPoint
	for s := -120; s < 125; s++ {
		v := 30.0
		if s >= 0 && s <= 5 {
			v = 50
		}
		pts = append(pts, AlertPoint{start.Add(time.Duration(s) * time.Second).UnixMilli(), v})
	}
	b, _ := json.Marshal(pts)
	_ = os.MkdirAll(a.alertDir(), 0o755)
	_ = os.WriteFile(filepath.Join(a.alertDir(), alertKey(*a.eventLocked(id))), b, 0o644)

	c, err := a.GetChartRange(base.Add(-15*time.Minute).UnixMilli(), base.Add(15*time.Minute).UnixMilli())
	if err != nil {
		t.Fatal(err)
	}
	if c.StepMs >= 60000 || !c.FineAlertsOnly {
		t.Fatalf("passo %d ms, fino %v", c.StepMs, c.FineAlertsOnly)
	}
	top := 0.0
	for _, v := range c.Values {
		if v != nil {
			top = max(top, *v)
		}
	}
	if top < 49 {
		t.Fatalf("o pico do alerta não apareceu na linha: máx da linha %v", top)
	}
	// sem alerta no trecho: continua 1 ponto por minuto
	c2, _ := a.GetChartRange(base.Add(-60*time.Minute).UnixMilli(), base.Add(-30*time.Minute).UnixMilli())
	if c2.StepMs < 60000 {
		t.Fatalf("sem alerta, passo %d", c2.StepMs)
	}
}
