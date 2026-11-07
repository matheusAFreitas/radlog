package app

import (
	"math"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"radlog/internal/gmc"
	"radlog/internal/radiacode"
	"radlog/internal/store"
)

func testApp(t *testing.T) (*App, string) {
	t.Helper()
	dir := t.TempDir()
	st, err := store.Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	a := NewApp()
	a.dataDir, a.store, a.cal = dir, st, gmc.DefaultCalibration()
	a.settings = defaultSettings()
	a.settings.NoMeasureReport = true // o PDF automático é testado à parte (não escreve depois do teste)
	return a, dir
}

// minutos de 'from' com os valores dados
func series(from time.Time, vals ...float64) []store.Point {
	var out []store.Point
	for i, v := range vals {
		out = append(out, store.Point{T: from.Add(time.Duration(i) * time.Minute), CPM: v, Src: store.SrcDevice})
	}
	return out
}

func TestRetroAlerts(t *testing.T) {
	a, dir := testApp(t)
	day := time.Date(2026, 10, 20, 0, 0, 0, 0, time.Local)
	var pts []store.Point
	pts = append(pts, series(day.Add(10*time.Hour), 20, 21, 60, 66, 72, 58, 47, 30, 20)...) // atenção 10:02–10:07
	pts = append(pts, series(day.Add(14*time.Hour), 22, 130, 145, 90, 20)...)               // alarme 14:01–14:04
	pts = append(pts, series(day.Add(18*time.Hour), 49, 44, 20)...)                         // nunca chega a 50

	added, err := a.store.ImportNew(pts)
	if err != nil || len(added) != len(pts) {
		t.Fatalf("import: %d (%v)", len(added), err)
	}
	if n := a.retroAlerts(added); n != 2 {
		t.Fatalf("achou %d alertas, queria 2", n)
	}
	al := a.GetEvents(kindAlert) // do mais novo para o mais velho
	if len(al) != 2 {
		t.Fatalf("alertas: %+v", al)
	}
	alarm, attn := al[0], al[1]
	at := func(h, m int) int64 {
		return day.Add(time.Duration(h)*time.Hour + time.Duration(m)*time.Minute).UnixMilli()
	}
	if attn.Level != "atencao" || !attn.Retro || attn.Time != at(10, 2) || attn.Peak != 72 || attn.PeakAt != at(10, 4) || attn.End != at(10, 7) {
		t.Fatalf("atenção: %+v", attn)
	}
	if !strings.HasPrefix(attn.Text, "Atenção (histórico do contador): pico de 72 CPM às 10:04") {
		t.Fatalf("texto: %q", attn.Text)
	}
	if alarm.Level != "alarme" || alarm.Time != at(14, 1) || alarm.Peak != 145 || alarm.End != at(14, 4) || alarm.Active {
		t.Fatalf("alarme: %+v", alarm)
	}

	// baixar de novo não repete (os minutos já existem) e, mesmo forçando,
	// o mesmo episódio não entra duas vezes
	again, _ := a.store.ImportNew(pts)
	if len(again) != 0 || a.retroAlerts(pts) != 0 {
		t.Fatal("repetiu alertas")
	}

	// um alerta ao vivo depois fica na frente; tudo volta igual do arquivo
	a.logAlert("Atenção: 55 CPM (acima de 50)")
	b := NewApp()
	b.dataDir = dir
	b.events = b.loadEvents()
	got := b.GetEvents(kindAlert)
	if len(got) != 3 || got[0].Retro || !got[1].Retro || got[1].End != at(14, 4) || got[2].PeakAt != at(10, 4) || got[2].Level != "atencao" {
		t.Fatalf("depois de recarregar: %+v", got)
	}
}

func TestRetroScanOnce(t *testing.T) {
	a, dir := testApp(t)
	day := time.Date(2026, 9, 3, 8, 0, 0, 0, time.Local)
	if _, err := a.store.Import(series(day, 20, 80, 85, 20)); err != nil {
		t.Fatal(err)
	}
	a.store.Add(store.Point{T: day.Add(time.Hour), CPM: 200, Src: store.SrcLive}) // ao vivo: já foi anotado na hora
	a.scanRetroOnce()
	if al := a.GetEvents(kindAlert); len(al) != 1 || al[0].Peak != 85 {
		t.Fatalf("alertas: %+v", al)
	}
	if _, err := os.Stat(filepath.Join(dir, "alertas-retro.ok")); err != nil {
		t.Fatal("não marcou que já conferiu")
	}
	a.scanRetroOnce()
	if al := a.GetEvents(kindAlert); len(al) != 1 {
		t.Fatalf("conferiu de novo: %+v", al)
	}
}

func waitFor(t *testing.T, what string, cond func() bool) {
	t.Helper()
	for i := 0; i < 60; i++ {
		if cond() {
			return
		}
		time.Sleep(100 * time.Millisecond)
	}
	t.Fatalf("esperando: %s", what)
}

func TestActiveAlerts(t *testing.T) {
	a, _ := testApp(t)
	active := func() []Event { return a.GetActiveAlerts() }

	a.StartDemo(60, false)
	waitFor(t, "atenção ativa", func() bool { l := active(); return len(l) == 1 && l[0].Level == "atencao" })
	a.StartDemo(150, false)
	waitFor(t, "alarme e atenção ativos", func() bool { l := active(); return len(l) == 2 && l[0].Level == "alarme" })
	if l := active(); l[0].Peak != 150 || l[1].Peak != 150 || !l[0].Active {
		t.Fatalf("pico: %+v", l)
	}
	a.StartDemo(70, false)
	waitFor(t, "só atenção", func() bool { l := active(); return len(l) == 1 && l[0].Level == "atencao" })
	a.StartDemo(20, false)
	waitFor(t, "nada ativo", func() bool { return len(active()) == 0 })
	for _, e := range a.GetEvents(kindAlert) {
		if e.End == 0 || e.Active || !e.Demo {
			t.Fatalf("não fechou: %+v", e)
		}
	}

	// pula direto para o alarme e desce para a faixa de atenção: a atenção é
	// anotada quando o alarme acaba
	a.StartDemo(150, false)
	waitFor(t, "alarme direto", func() bool { l := active(); return len(l) == 1 && l[0].Level == "alarme" })
	a.StartDemo(70, false)
	waitFor(t, "atenção depois do alarme", func() bool { l := active(); return len(l) == 1 && l[0].Level == "atencao" && l[0].CPM == 70 })
	a.StopDemo()
	if len(active()) != 0 || len(a.GetEvents(kindAlert)) != 0 {
		t.Fatal("o demo deixou alertas")
	}
}

func TestHeatmapAlerts(t *testing.T) {
	a, _ := testApp(t)
	now := time.Now()
	y, m, d := now.Date()
	today := time.Date(y, m, d, 0, 0, 0, 0, time.Local)
	at := func(day, h, min int) time.Time {
		return today.AddDate(0, 0, day).Add(time.Duration(h)*time.Hour + time.Duration(min)*time.Minute)
	}
	// atenção ontem das 06:50 às 07:10 (pega duas horas) e alarme hoje 01:55
	a.addEvent(Event{Time: at(-1, 6, 50).UnixMilli(), End: at(-1, 7, 10).UnixMilli(), Kind: kindAlert, Level: "atencao", CPM: 55, Peak: 61, Text: "Atenção: 55 CPM (acima de 50)"}, true)
	a.addEvent(Event{Time: at(0, 1, 55).UnixMilli(), End: at(0, 1, 56).UnixMilli(), Kind: kindAlert, Level: "alarme", CPM: 120, Peak: 130, Retro: true, Text: "Alarme (histórico do contador)"}, true)
	a.addEvent(Event{Time: at(-30, 1, 0).UnixMilli(), Kind: kindAlert, Level: "alarme", Text: "Alarme: fora do período"}, true)
	a.logEvent("Contador conectado")

	h, err := a.GetHeatmap(7)
	if err != nil {
		t.Fatal(err)
	}
	if len(h.Alerts) != 3 {
		t.Fatalf("horas marcadas: %v", h.Alerts)
	}
	if l := h.Alerts["5-6"]; len(l) != 1 || l[0].Level != "atencao" || l[0].Peak != 61 {
		t.Fatalf("ontem 6h: %+v", l)
	}
	if l := h.Alerts["5-7"]; len(l) != 1 {
		t.Fatalf("ontem 7h: %+v", l)
	}
	if l := h.Alerts["6-1"]; len(l) != 1 || l[0].Level != "alarme" || !l[0].Retro {
		t.Fatalf("hoje 1h: %+v", l)
	}
}

func TestDangerLevel(t *testing.T) {
	a, _ := testApp(t)
	a.settings.AlarmEnabled = false // o perigo vale mesmo com o alarme desligado
	active := func() []Event { return a.GetActiveAlerts() }

	a.StartDemo(1200, false)
	waitFor(t, "perigo ativo", func() bool { l := active(); return len(l) >= 1 && l[0].Level == "perigo" })
	if l := active(); len(l) != 1 { // pulou do fundo para o perigo: só o perigo abre
		t.Fatalf("ativos: %+v", l)
	}
	if l := active(); l[0].Text != "[demo] Perigo: 1200 CPM (a partir de 1000)" {
		t.Fatalf("texto: %q", l[0].Text)
	}
	// 950 ainda é perigo (só sai abaixo de 900)
	a.StartDemo(950, false)
	time.Sleep(1500 * time.Millisecond)
	if l := active(); len(l) == 0 || l[0].Level != "perigo" || l[0].Peak != 1200 {
		t.Fatalf("saiu cedo do perigo: %+v", l)
	}
	a.StartDemo(800, false)
	waitFor(t, "fim do perigo", func() bool { l := active(); return len(l) == 1 && l[0].Level == "atencao" })
	a.StopDemo()

	if level(999, DefaultLevels()) != "alerta" || level(1000, DefaultLevels()) != "perigo" {
		t.Fatal("level")
	}
	on := false
	if s, _ := dangerStep(&on, 1000, DefaultLevels()); !s || !on {
		t.Fatal("não entrou")
	}
	if _, e := dangerStep(&on, 901, DefaultLevels()); e || !on {
		t.Fatal("saiu acima de 900")
	}
	if _, e := dangerStep(&on, 899, DefaultLevels()); !e || on {
		t.Fatal("não saiu")
	}
}

func TestRetroDanger(t *testing.T) {
	a, _ := testApp(t)
	day := time.Date(2026, 10, 21, 3, 0, 0, 0, time.Local)
	pts := series(day, 20, 400, 1500, 300, 20)
	added, _ := a.store.ImportNew(pts)
	if n := a.retroAlerts(added); n != 1 {
		t.Fatalf("achou %d", n)
	}
	if al := a.GetEvents(kindAlert); al[0].Level != "perigo" || al[0].Peak != 1500 || !strings.HasPrefix(al[0].Text, "Perigo (histórico do contador)") {
		t.Fatalf("%+v", al[0])
	}
}

func TestSaturation(t *testing.T) {
	t0 := time.Date(2026, 10, 28, 11, 0, 0, 0, time.Local)
	at := func(s int) time.Time { return t0.Add(time.Duration(s) * time.Second) }
	var st satState
	// fundo, depois sobe em 3 s até o fim da escala
	for s := 0; s < 30; s++ {
		st.step(at(s), 0.28, 0.3, DefaultLevels())
	}
	st.step(at(30), 5, 50, DefaultLevels())
	st.step(at(31), 300, 1500, DefaultLevels())
	if on, _ := st.step(at(32), 1200, 1900, DefaultLevels()); on {
		t.Fatal("saturou antes do fim da escala")
	}
	if on, started := st.step(at(33), 2100, 2400, DefaultLevels()); !on || !started || st.reason != "limite" {
		t.Fatalf("limite: %v %v %q", on, started, st.reason)
	}
	r := st.riseFrom(at(33), 0.5)
	if r == nil || r.Longer || r.FromUSv != 0.28 || r.Seconds != 4 {
		t.Fatalf("subida: %+v", r)
	}
	// o tubo trava: a taxa do segundo despenca, mas o aviso fica 60 s
	if on, _ := st.step(at(34), 1500, 30, DefaultLevels()); !on || st.reason != "queda" {
		t.Fatalf("queda: %v %q", st.on, st.reason)
	}
	if on, _ := st.step(at(90), 2, 0.3, DefaultLevels()); !on {
		t.Fatal("soltou antes de 60 s")
	}
	if on, _ := st.step(at(95), 0.3, 0.3, DefaultLevels()); on {
		t.Fatal("não soltou depois de 60 s")
	}
	// subida lenta (andando até a fonte): sem leitura normal nos últimos 3 min
	var slow satState
	for s := 0; s < 300; s++ {
		slow.step(at(s), 1+float64(s)/10, 1+float64(s)/10, DefaultLevels())
	}
	if r := slow.riseFrom(at(300), 0.5); r == nil || !r.Longer || r.Seconds < 170 {
		t.Fatalf("subida lenta: %+v", r)
	}
}

func TestSaturationKeepsDanger(t *testing.T) {
	a, _ := testApp(t)
	a.StartDemo(999999, false) // ~6.500 µSv/h com a calibração de fábrica
	waitFor(t, "saturado", func() bool {
		a.mu.Lock()
		defer a.mu.Unlock()
		return a.demo.sat.on && a.demo.dangerOn && a.demo.sat.rise != nil
	})
	a.StartDemo(20, false) // a taxa despenca: pode ser o tubo travado
	time.Sleep(2500 * time.Millisecond)
	a.mu.Lock()
	on, reason, danger := a.demo.sat.on, a.demo.sat.reason, a.demo.dangerOn
	a.mu.Unlock()
	if !on || reason != "queda" || !danger {
		t.Fatalf("depois da queda: sat %v %q perigo %v", on, reason, danger)
	}
	found := false
	for _, e := range a.GetEvents(kindEvent) {
		if strings.Contains(e.Text, "Geiger saturado") {
			found = true
		}
	}
	if !found {
		t.Fatal("não anotou a saturação")
	}
	a.StopDemo()
}

func TestCustomLevels(t *testing.T) {
	a, _ := testApp(t)
	a.settings.AlarmEnabled = false
	a.settings.Levels = Levels{AttentionCPM: 80, DangerCPM: 300}.norm()
	if a.settings.Levels.SnoozeMin != 5 || a.settings.Levels.SatMaxUSv != 2000 {
		t.Fatalf("padrões: %+v", a.settings.Levels)
	}
	a.StartDemo(70, false) // abaixo da atenção ajustada
	time.Sleep(1500 * time.Millisecond)
	if l := a.GetActiveAlerts(); len(l) != 0 {
		t.Fatalf("70 não deveria avisar com atenção em 80: %+v", l)
	}
	a.StartDemo(350, false)
	waitFor(t, "perigo em 300", func() bool { l := a.GetActiveAlerts(); return len(l) > 0 && l[0].Level == "perigo" })
	if l := a.GetActiveAlerts(); l[0].Text != "[demo] Perigo: 350 CPM (a partir de 300)" {
		t.Fatalf("texto: %q", l[0].Text)
	}
	a.StopDemo()

	// valores absurdos e zeros voltam para limites razoáveis
	n := Levels{AttentionCPM: -3, DangerCPM: 1, SnoozeMin: 999, SatHoldSec: 1}.norm()
	if n.AttentionCPM != 50 || n.DangerCPM != 50 || n.SnoozeMin != 60 || n.SatHoldSec != 5 {
		t.Fatalf("norm: %+v", n)
	}
	// SaveSettings normaliza
	s := a.SaveSettings(Settings{AlarmCPM: 100, Levels: Levels{DangerCPM: 2000}})
	if s.Levels.DangerCPM != 2000 || s.Levels.AttentionCPM != 50 {
		t.Fatalf("salvo: %+v", s.Levels)
	}
	if c, _ := a.GetChart("1h"); c.DangerCPM != 2000 || c.AttentionCPM != 50 {
		t.Fatalf("gráfico: %d %d", c.DangerCPM, c.AttentionCPM)
	}
}

func TestChartRange(t *testing.T) {
	a, _ := testApp(t)
	now := time.Now()
	cases := []struct {
		from, to time.Time
		step     time.Duration
	}{
		{now.Add(-10 * time.Minute), now, 5 * time.Second},                             // última hora: leituras por segundo
		{now.Add(-3*time.Hour - 10*time.Minute), now.Add(-3 * time.Hour), time.Minute}, // antigo: por minuto
		{now.Add(-7 * 24 * time.Hour), now, 15 * time.Minute},
		{now.Add(-30 * time.Second), now.Add(-20 * time.Second), 5 * time.Second}, // estreito: abre 2 min
	}
	for i, c := range cases {
		ch, err := a.GetChartRange(c.to.UnixMilli(), c.from.UnixMilli()) // invertido de propósito
		if err != nil {
			t.Fatal(err)
		}
		if got := time.Duration(ch.StepMs) * time.Millisecond; got != c.step {
			t.Fatalf("caso %d: passo %v, queria %v", i, got, c.step)
		}
		if len(ch.Values) > 730 || len(ch.Values) == 0 || ch.From >= ch.To {
			t.Fatalf("caso %d: %d pontos, %d–%d", i, len(ch.Values), ch.From, ch.To)
		}
	}
}

func TestAwayPeriods(t *testing.T) {
	a, dir := testApp(t)
	now := time.Now().Truncate(time.Minute)
	var pts []store.Point
	for i := 0; i < 240; i++ { // 4 h em casa a 20 CPM, a última hora fora a 80
		cpm := 20.0
		if i >= 180 {
			cpm = 80
		}
		pts = append(pts, store.Point{T: now.Add(time.Duration(i-240) * time.Minute), CPM: cpm, Src: store.SrcDevice})
	}
	if _, err := a.store.Import(pts); err != nil {
		t.Fatal(err)
	}
	if bg, _ := a.backgroundCPM(); bg != 20 {
		t.Fatalf("fundo sem marcar: %v", bg)
	}
	p, err := a.SavePeriod(Period{From: now.Add(-60 * time.Minute).UnixMilli(), To: now.UnixMilli(), Name: "  "})
	if err != nil || p.ID != 1 || p.Name != "Fora de casa" {
		t.Fatalf("salvar: %+v %v", p, err)
	}
	// com 25% fora a 80 CPM, a mediana já era 20; o teste forte é o dia e o mapa
	days, _ := a.GetDaily(2) // 2 dias: perto da meia-noite as 4 h pegam ontem
	if len(days) == 0 || days[len(days)-1].CPM > 20.0001 && days[0].CPM > 20.0001 {
		t.Fatalf("fundo diário não tirou o fora de casa: %+v", days)
	}
	h, _ := a.GetHeatmap(2)
	awayMin := 0
	for _, row := range h.AwayCounts {
		for _, n := range row {
			awayMin += n
		}
	}
	if awayMin != 60 || *h.Max > 20.0001 {
		t.Fatalf("mapa: %d min fora, máx %v", awayMin, *h.Max)
	}
	// a dose continua contando tudo
	if c, _ := a.GetDose("6h", "publico"); c.MeasuredMinutes != 240 {
		t.Fatalf("dose: %d min", c.MeasuredMinutes)
	}
	// volta do disco, edita e apaga
	b := NewApp()
	b.dataDir = dir
	b.loadPeriods()
	if l := b.GetPeriods(); len(l) != 1 || l[0].Name != "Fora de casa" {
		t.Fatalf("disco: %+v", l)
	}
	p.Name = "Passeio"
	if p2, _ := a.SavePeriod(p); p2.ID != 1 || len(a.GetPeriods()) != 1 || a.GetPeriods()[0].Name != "Passeio" {
		t.Fatalf("editar: %+v", a.GetPeriods())
	}
	if _, err := a.SavePeriod(Period{From: 1000, To: 2000}); err == nil {
		t.Fatal("aceitou período de 1 s")
	}
	_ = a.DeletePeriod(1)
	if len(a.GetPeriods()) != 0 {
		t.Fatal("não apagou")
	}
}

func TestNewRanges(t *testing.T) {
	a, _ := testApp(t)
	base := time.Date(2026, 10, 27, 9, 0, 0, 0, time.Local)
	var pts []store.Point
	add := func(fromMin, n int) {
		for i := 0; i < n; i++ {
			pts = append(pts, store.Point{T: base.Add(time.Duration(fromMin+i) * time.Minute), CPM: 20, Src: store.SrcDevice})
		}
	}
	add(0, 90)   // 09:00–10:30
	add(93, 60)  // buraco de 3 min: continua o mesmo trecho
	add(300, 10) // 10 min: curto demais
	add(400, 45) // outro trecho
	r := a.newRanges(pts)
	if len(r) != 2 || r[0].Minutes != 150 || r[0].From != base.UnixMilli() || r[0].To != base.Add(153*time.Minute).UnixMilli() || r[1].Minutes != 45 {
		t.Fatalf("trechos: %+v", r)
	}
	// marcado como fora: não pergunta de novo
	_, _ = a.SavePeriod(Period{From: base.Add(400 * time.Minute).UnixMilli(), To: base.Add(445 * time.Minute).UnixMilli()})
	if r := a.newRanges(pts); len(r) != 1 {
		t.Fatalf("depois de marcar: %+v", r)
	}
	// tempo mínimo configurável: 20 min entra com o padrão (15), sai com 60
	add(600, 20)
	if r := a.newRanges(pts); len(r) != 2 || r[1].Minutes != 20 {
		t.Fatalf("20 min com o padrão: %+v", r)
	}
	a.settings.AwayAskMin = 60
	if r := a.newRanges(pts); len(r) != 1 {
		t.Fatalf("com 60 min: %+v", r)
	}
	if normAwayAsk(0) != 15 || normAwayAsk(2) != 5 || normAwayAsk(999) != 240 {
		t.Fatal("limites do tempo mínimo")
	}
}

func TestSavedMeasurements(t *testing.T) {
	a, dir := testApp(t)
	a.dev = nil
	a.demo.Active = false
	// simula um contador conectado só para poder medir (sem serial de verdade)
	a.measure = MeasureState{Phase: "fundo", Seconds: 3, Name: "banana", PhaseStart: time.Now().Add(-5 * time.Second).UnixMilli()}
	for i := 0; i < 3; i++ {
		a.feedMeasure(0, 1, false)
	}
	a.measure.Phase, a.measure.Elapsed, a.measure.Counts = "objeto", 0, 0
	a.measure.PhaseStart = time.Now().UnixMilli()
	for i := 0; i < 3; i++ {
		a.feedMeasure(0, 1, false)
	}
	st := a.GetMeasure()
	list := a.GetMeasurements()
	if st.Phase != "pronto" || st.SavedID == 0 || len(list) != 1 || list[0].Name != "banana" || list[0].Obj.End < list[0].Obj.Start || list[0].Bg.Start == 0 {
		t.Fatalf("salvar: %+v / %+v", st, list)
	}
	if sm := list[0].Samples; len(sm) != 6 || sm[0].Phase != "fundo" || sm[5].Phase != "objeto" || sm[5].T < sm[2].T || sm[3].CPS != 1 {
		t.Fatalf("segundos guardados: %+v", sm)
	}
	// os minutos com o objeto não entram no fundo
	in := time.UnixMilli(list[0].Obj.Start)
	if !a.measuringChecker()(in) || a.measuringChecker()(in.Add(-time.Hour)) {
		t.Fatal("checker")
	}
	if err := a.UpdateMeasurement(list[0].ID, "banana nanica", "com casca"); err != nil {
		t.Fatal(err)
	}
	b := NewApp()
	b.dataDir = dir
	b.loadMeasurements()
	if l := b.GetMeasurements(); len(l) != 1 || l[0].Name != "banana nanica" || l[0].Note != "com casca" {
		t.Fatalf("disco: %+v", l)
	}
	_ = a.DeleteMeasurement(list[0].ID)
	if len(a.GetMeasurements()) != 0 {
		t.Fatal("não apagou")
	}
	// medição no demo não é salva
	a.demo.Active = true
	a.measure = MeasureState{Phase: "objeto", Seconds: 1, Demo: true, Bg: &MeasurePhase{Counts: 1, Seconds: 1, CPM: 60, ErrCPM: 60}}
	a.feedMeasure(0, 1, true)
	if len(a.GetMeasurements()) != 0 {
		t.Fatal("salvou medição do demo")
	}
}

func TestRadiacodeMeasure(t *testing.T) {
	a, _ := testApp(t)
	a.settings.NoMeasureReport = true
	bg := &MeasurePhase{Counts: 600, Seconds: 60, CPM: 600, ErrCPM: 24.5, USv: 0.10, Source: "casa"}
	a.measure = MeasureState{Phase: "objeto", Seconds: 3, Device: "radiacode", Bg: bg, PhaseStart: time.Now().UnixMilli()}
	// o GMC não entra numa medição do RadiaCode
	a.feedMeasure(30, 5, false)
	if a.GetMeasure().Elapsed != 0 {
		t.Fatal("GMC entrou na medição do RadiaCode")
	}
	// 3 s de amostras de 0,5 s: 40 cps (20 contagens cada) e 0,3 µSv/h
	for i := 0; i < 3; i++ {
		a.rcMeasureSamples(radiacode.RawSamples{Period: 500 * time.Millisecond, CPS: []float64{40, 40}, DoseRate: []float64{0.3, 0.3}})
	}
	st := a.GetMeasure()
	list := a.GetMeasurements()
	if st.Phase != "pronto" || st.Obj == nil || st.Obj.Counts != 120 || math.Abs(st.Obj.USv-0.3) > 1e-9 || math.Abs(st.Result.NetUSv-0.2) > 1e-9 {
		t.Fatalf("medição: %+v obj %+v res %+v", st, st.Obj, st.Result)
	}
	if len(list) != 1 || list[0].Device != "radiacode" || len(list[0].Samples) != 3 || list[0].Samples[0].CPS != 40 {
		t.Fatalf("salva: %+v", list)
	}
}

func TestSpecDiff(t *testing.T) {
	s0 := &radiacode.Spectrum{Duration: 10 * time.Second, Counts: []uint32{1, 2, 3}}
	s1 := &radiacode.Spectrum{Duration: 25 * time.Second, Counts: []uint32{4, 2, 9}, A1: 3}
	d := specDiff(s0, s1)
	if d.ObjSec != 15 || d.Obj[0] != 3 || d.Obj[1] != 0 || d.Obj[2] != 6 || d.A[1] != 3 {
		t.Fatalf("diferença: %+v", d)
	}
	// zerado no meio: vale o segundo inteiro
	s1.Counts[1] = 1
	if d := specDiff(s0, s1); d.Obj[1] != 1 || d.ObjSec != 25 {
		t.Fatalf("zerado: %+v", d)
	}
}

func TestDualMeasure(t *testing.T) {
	a, _ := testApp(t)
	a.settings.NoMeasureReport = true
	rcBg := &MeasurePhase{Counts: 43200, Seconds: 3600, CPM: 720, ErrCPM: 3.5, USv: 0.10, Source: "casa"}
	gBg := &MeasurePhase{Counts: 1500, Seconds: 3600, CPM: 25, ErrCPM: 0.65, USv: 0.16, Source: "casa"}
	// os dois perto: o objeto quase não muda o RadiaCode e triplica o GMC (beta)
	a.measure = MeasureState{Phase: "objeto", Seconds: 60, Device: "radiacode", Mode: "ambos", Bg: rcBg, PhaseStart: time.Now().UnixMilli(),
		Other: &MeasureSide{Device: "gmc", Role: "objeto", Bg: gBg}}
	for i := 0; i < 60; i++ {
		a.rcMeasureSamples(radiacode.RawSamples{Period: 500 * time.Millisecond, CPS: []float64{12, 12}, DoseRate: []float64{0.1, 0.1}})
		cps := uint32(1)
		if i%3 == 0 {
			cps = 2 // ~80 CPM
		}
		a.feedMeasure(80, cps, false)
	}
	st := a.GetMeasure()
	if st.Phase != "pronto" || st.Other == nil || st.Other.Result == nil || st.Other.Obj == nil {
		t.Fatalf("medição: %+v other %+v", st, st.Other)
	}
	if st.Other.Result.Sigma < 3 || st.Result.Sigma > 2 || !strings.Contains(st.Other.Hint, "beta") {
		t.Fatalf("resultado: rc %+v gmc %+v dica %q", st.Result, st.Other.Result, st.Other.Hint)
	}
	list := a.GetMeasurements()
	if len(list) != 1 || list[0].Mode != "ambos" || list[0].Other == nil || list[0].Other.Result == nil {
		t.Fatalf("salva: %+v", list)
	}

	// um no objeto (GMC), outro com você (RadiaCode): dose da pessoa
	a.measure = MeasureState{Phase: "objeto", Seconds: 30, Device: "gmc", Mode: "voce", Bg: gBg, PhaseStart: time.Now().UnixMilli(),
		Other: &MeasureSide{Device: "radiacode", Role: "voce", BgUSv: 0.10}}
	for i := 0; i < 30; i++ {
		a.rcMeasureSamples(radiacode.RawSamples{Period: 500 * time.Millisecond, CPS: []float64{24, 24}, DoseRate: []float64{0.46, 0.46}})
		a.feedMeasure(30, 1, false)
	}
	st = a.GetMeasure()
	o := st.Other
	// 30 s a 0,46 µSv/h = 0,00383 µSv; acima do fundo (0,10): 0,003 µSv
	if st.Phase != "pronto" || o == nil || math.Abs(o.Dose-0.46*30/3600) > 1e-6 || math.Abs(o.Excess-0.36*30/3600) > 1e-6 || o.MaxUSv != 0.46 {
		t.Fatalf("com você: %+v", o)
	}
}
