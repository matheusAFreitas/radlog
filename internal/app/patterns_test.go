package app

import (
	"fmt"
	"math"
	"math/rand"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"radlog/internal/store"
)

func TestPatternsDayNight(t *testing.T) {
	start := time.Date(2026, 9, 1, 0, 0, 0, 0, time.Local)
	r := rand.New(rand.NewSource(3))
	var pts []store.Point
	for d := 0; d < 30; d++ {
		for m := 0; m < 1440; m++ {
			tm := start.AddDate(0, 0, d).Add(time.Duration(m) * time.Minute)
			lvl := 20.0
			if !isDaytime(tm) {
				lvl = 24 // noite 20% acima (radônio)
			}
			pts = append(pts, store.Point{T: tm, CPM: poissonR(r, lvl)})
		}
	}
	var p Patterns
	p.Unit = "CPM"
	fillPatterns(&p, pointsVals(pts))
	if p.Minutes != 30*1440 || len(p.Hours) != 24 || len(p.Weekdays) != 7 || len(p.Months) != 1 {
		t.Fatalf("tamanhos: %d %d %d %d", p.Minutes, len(p.Hours), len(p.Weekdays), len(p.Months))
	}
	if p.Hours[3].Median < 23 || p.Hours[12].Median > 21 || isDaytime(time.Date(2026, 1, 1, p.PeakHour, 0, 0, 0, time.Local)) {
		t.Fatalf("horas: 3h %v, 12h %v, pico %dh", p.Hours[3].Median, p.Hours[12].Median, p.PeakHour)
	}
	// 29 dias com as duas metades inteiras (a noite do último dia fica sem as horas depois da meia-noite)
	if p.DNDays < 29 || math.Abs(p.DNDiff-4) > 0.3 || math.Abs(p.DNPct-20) > 2 || !strings.Contains(p.DNText, "acima do dia") {
		t.Fatalf("dia × noite: %d dias, %.2f (%.1f%%) — %s", p.DNDays, p.DNDiff, p.DNPct, p.DNText)
	}
	if w := p.Weekdays[2]; w.Days < 4 || math.Abs(w.Mean-22) > 0.3 {
		t.Fatalf("terça: %+v", w)
	}

	// sem diferença: o mesmo nível dia e noite
	var flat []store.Point
	for d := 0; d < 20; d++ {
		for m := 0; m < 1440; m++ {
			flat = append(flat, store.Point{T: start.AddDate(0, 0, d).Add(time.Duration(m) * time.Minute), CPM: poissonR(r, 20)})
		}
	}
	var q Patterns
	q.Unit = "CPM"
	fillPatterns(&q, pointsVals(flat))
	if !strings.Contains(q.DNText, "Sem diferença clara") {
		t.Fatalf("plano: %s", q.DNText)
	}
	// poucos dias
	var z Patterns
	fillPatterns(&z, pointsVals(pts[:1440]))
	if z.DNDays > 1 || !strings.Contains(z.DNText, "Poucos dias") {
		t.Fatalf("1 dia: %s", z.DNText)
	}
}

func TestPatternsAndHeatmapRadiacode(t *testing.T) {
	a := newTestApp(t)
	now := time.Now().Truncate(time.Minute)
	start := now.Add(-48 * time.Hour)
	dir := a.rcDir()
	os.MkdirAll(dir, 0o755)
	files := map[string]*strings.Builder{}
	for m := start; m.Before(now); m = m.Add(time.Minute) {
		k := m.Format("2006-01") + ".csv"
		if files[k] == nil {
			files[k] = &strings.Builder{}
		}
		fmt.Fprintf(files[k], "%s,%.2f,%.4f,%d\n", m.Format(time.RFC3339), 2.0, 0.1, 30)
	}
	for k, b := range files {
		os.WriteFile(filepath.Join(dir, k), []byte(b.String()), 0o644)
	}
	p, err := a.GetPatterns(30, devRC)
	if err != nil {
		t.Fatal(err)
	}
	if p.Unit != "µSv/h" || p.Minutes != 48*60 || p.Hours[5].Median != 0.1 || p.First == 0 {
		t.Fatalf("padrões RC: %s %d %v %d", p.Unit, p.Minutes, p.Hours[5].Median, p.First)
	}
	h, err := a.GetHeatmapDev(7, devRC)
	if err != nil {
		t.Fatal(err)
	}
	if h.Device != devRC || h.Minutes != 48*60 || h.Max == nil || math.Abs(*h.Max-0.1) > 1e-9 {
		t.Fatalf("mapa RC: %s %d %v", h.Device, h.Minutes, h.Max)
	}
	// o do GMC continua vazio
	g, _ := a.GetHeatmapDev(7, "")
	if g.Minutes != 0 || g.Device != "" {
		t.Fatalf("mapa GMC: %d", g.Minutes)
	}
}

func TestAlertEpisodeAcrossDisconnect(t *testing.T) {
	t0 := time.Date(2026, 11, 7, 10, 0, 0, 0, time.Local).UnixMilli()
	min := int64(60000)
	mk := func(id, start, end int64, lvl, reason string) Event {
		return Event{ID: id, Kind: kindAlert, Time: start, End: end, Level: lvl, Peak: 60, EndReason: reason}
	}
	// desconectou com o alerta aberto e voltou alto 7 min depois: um episódio
	evs := []Event{mk(1, t0, t0+5*min, "atencao", endDisconnected), mk(2, t0+12*min, t0+20*min, "alarme", "")}
	eps := alertEpisodes(evs, time.Now())
	if len(eps) != 1 || eps[0].Level != "alarme" || len(eps[0].Breaks) != 1 || eps[0].Breaks[0].From != t0+5*min || eps[0].Breaks[0].To != t0+12*min || eps[0].EndReason != "" {
		t.Fatalf("episódio com desconexão: %+v", eps)
	}
	// voltou depois de 20 min: dois episódios, o primeiro termina desconectado
	evs[1].Time, evs[1].End = t0+25*min, t0+30*min
	eps = alertEpisodes(evs, time.Now())
	if len(eps) != 2 || eps[0].EndReason != endDisconnected {
		t.Fatalf("longe demais: %+v", eps)
	}
	// app fechado não emenda
	evs = []Event{mk(1, t0, t0+5*min, "atencao", endAppClosed), mk(2, t0+8*min, t0+20*min, "atencao", "")}
	if eps = alertEpisodes(evs, time.Now()); len(eps) != 2 {
		t.Fatalf("app fechado emendou: %+v", eps)
	}
	// o motivo vai e volta do eventos.csv
	e := mk(1, t0, t0+5*min, "atencao", endDisconnected)
	e.Text = "Atenção: 60 CPM (a partir de 50)"
	if back, ok := parseEventRow(eventRow(e)); !ok || back.EndReason != endDisconnected || back.End != e.End {
		t.Fatalf("csv: %+v", back)
	}
}

func TestDisconnectMarksAlerts(t *testing.T) {
	a := newTestApp(t)
	now := time.Now().Truncate(time.Minute)
	var pts []store.Point
	for i := 30; i >= 1; i-- {
		cpm := 20.0
		if i <= 10 {
			cpm = 60 + float64(i)
		}
		pts = append(pts, store.Point{T: now.Add(-time.Duration(i) * time.Minute), CPM: cpm, Src: store.SrcLive})
	}
	a.store.Import(pts)
	id := a.addEvent(Event{Time: now.Add(-10 * time.Minute).UnixMilli(), Kind: kindAlert, Level: "atencao", CPM: 70, Peak: 70, PeakAt: now.Add(-10 * time.Minute).UnixMilli(), Text: "Atenção: 70 CPM (a partir de 50)"}, true)
	a.mu.Lock()
	a.alerts.attn = id
	a.mu.Unlock()
	a.disconnect("")
	a.mu.Lock()
	e := *a.eventLocked(id)
	a.mu.Unlock()
	if e.EndReason != endDisconnected || e.End == 0 {
		t.Fatalf("alerta depois de desconectar: %+v", e)
	}
	d, err := a.GetAlertDetail(id)
	if err != nil {
		t.Fatal(err)
	}
	ep := d.Episode
	if ep == nil || ep.Src != "minutos" || ep.N != 10 || ep.Min != 61 || ep.Max != 70 || math.Abs(ep.Mean-65.5) > 1e-9 || ep.EndReason != endDisconnected {
		t.Fatalf("episódio: %+v", ep)
	}
}
