package app

import (
	"errors"
	"sync/atomic"
	"testing"
	"time"

	"radlog/internal/gmc"
	"radlog/internal/store"
)

// minutos do GMC-800 vindos da memória, já no arquivo
func gmcMem(t *testing.T, a *App, from time.Time, mins int, cpm float64) []store.Point {
	t.Helper()
	var pts []store.Point
	for i := 0; i < mins; i++ {
		pts = append(pts, store.Point{T: from.Add(time.Duration(i) * time.Minute), CPM: cpm, Src: store.SrcDevice})
	}
	added, err := a.store.ImportNew(pts)
	if err != nil || len(added) != mins {
		t.Fatalf("import: %d %v", len(added), err)
	}
	return added
}

func TestGmcAwayAsks(t *testing.T) {
	a := newTestApp(t)
	t0 := time.Now().Add(-20 * time.Hour).Truncate(time.Minute)

	// dois trechos e um curto demais (10 min junta sem perguntar)
	pts := gmcMem(t, a, t0, 120, 20)
	pts = append(pts, gmcMem(t, a, t0.Add(3*time.Hour), 10, 20)...)
	pts = append(pts, gmcMem(t, a, t0.Add(5*time.Hour), 45, 40)...)
	if n := a.gmcAwayAsks(pts, true); n != 2 {
		t.Fatalf("perguntas: %d", n)
	}
	l := a.GetRcAwayAsks()
	if len(l) != 2 || l[0].Kind != askGmc || l[0].From != t0.UnixMilli() || l[0].To != t0.Add(120*time.Minute).UnixMilli() {
		t.Fatalf("lista: %+v", l)
	}
	if v := l[1]; v.GmcMinutes != 45 || v.GmcUSv <= 0 || v.GmcMaxUSv <= 0 || v.GmcDoseUSv <= 0 {
		t.Fatalf("resumo do GMC: %+v", v)
	}
	// de novo, a mesma coisa: não duplica
	if n := a.gmcAwayAsks(pts, true); n != 0 || len(a.GetRcAwayAsks()) != 2 {
		t.Fatal("duplicou")
	}

	// "Estava em casa" tira a pergunta; "Saí de casa…" marca com os aparelhos escolhidos
	if _, err := a.AnswerAway(l[0].ID, "casa", 0, 0, "", false, false); err != nil {
		t.Fatal(err)
	}
	if _, err := a.AnswerAway(l[1].ID, "sai", 0, 0, "Mercado", false, false); err == nil {
		t.Fatal("aceitou saída sem aparelho")
	}
	p, err := a.AnswerAway(l[1].ID, "sai", 0, 0, "Mercado", true, false)
	if err != nil || p.ID == 0 || p.GMC != WhereMe || p.RC != WhereHome || p.Name != "Mercado" || p.From != l[1].From || p.To != l[1].To {
		t.Fatalf("trecho: %+v %v", p, err)
	}
	if l := a.GetRcAwayAsks(); len(l) != 0 {
		t.Fatalf("sobrou: %+v", l)
	}
	if _, err := a.AnswerAway(l[1].ID, "casa", 0, 0, "", false, false); err == nil {
		t.Fatal("respondeu duas vezes")
	}
	// o trecho marcado não volta a ser perguntado
	if n := a.gmcAwayAsks(pts, true); n != 1 {
		t.Fatalf("depois de responder: %d", n) // só o de "em casa" (a resposta não fica guardada)
	}
}

func TestGmcAwayPartial(t *testing.T) {
	a := newTestApp(t)
	t0 := time.Now().Add(-20 * time.Hour).Truncate(time.Minute)
	first := gmcMem(t, a, t0, 60, 20)
	half := gmcMem(t, a, t0.Add(2*time.Hour), 30, 20) // o resto deste trecho não chegou
	// interrompido: pergunta o primeiro, o último espera
	if n := a.gmcAwayAsks(append(first, half...), false); n != 1 {
		t.Fatalf("interrompido: %d", n)
	}
	if d, _ := a.readCounterSync(); d.AskFrom != t0.Add(2*time.Hour).UnixMilli() {
		t.Fatalf("perguntarDesde: %d", d.AskFrom)
	}
	// o download seguinte, completo, traz o resto: uma pergunta do trecho todo
	rest := gmcMem(t, a, t0.Add(2*time.Hour+30*time.Minute), 30, 20)
	if n := a.gmcAwayAsks(rest, true); n != 1 {
		t.Fatalf("completo: %d", n)
	}
	l := a.GetRcAwayAsks()
	if len(l) != 2 || l[1].From != t0.Add(2*time.Hour).UnixMilli() || l[1].To != t0.Add(3*time.Hour).UnixMilli() {
		t.Fatalf("lista: %+v", l)
	}
	if d, _ := a.readCounterSync(); d.AskFrom != 0 {
		t.Fatal("não limpou perguntarDesde")
	}
}

func TestGmcAwayJoinsRc(t *testing.T) {
	a := newTestApp(t)
	t0 := time.Now().Add(-20 * time.Hour).Truncate(time.Minute)
	now := time.Now()
	// o RC102 perguntou primeiro: vira uma pergunta só
	a.mu.Lock()
	a.addRcAwayLocked(RcAwayAsk{ID: t0.Add(5 * time.Minute).UnixMilli(), From: t0.Add(5 * time.Minute).UnixMilli(), To: t0.Add(70 * time.Minute).UnixMilli()}, now)
	a.mu.Unlock()
	a.gmcAwayAsks(gmcMem(t, a, t0, 60, 20), true)
	l := a.GetRcAwayAsks()
	if len(l) != 1 || l[0].Kind != askBoth || l[0].From != t0.UnixMilli() || l[0].To != t0.Add(70*time.Minute).UnixMilli() {
		t.Fatalf("juntas: %+v", l)
	}
	// o GMC-800 primeiro e o RC102 depois: também
	t1 := t0.Add(5 * time.Hour)
	a.gmcAwayAsks(gmcMem(t, a, t1, 60, 20), true)
	a.mu.Lock()
	ok := a.addRcAwayLocked(RcAwayAsk{ID: t1.Add(10 * time.Minute).UnixMilli(), From: t1.Add(10 * time.Minute).UnixMilli(), To: t1.Add(50 * time.Minute).UnixMilli()}, now)
	a.mu.Unlock()
	l = a.GetRcAwayAsks()
	if !ok || len(l) != 2 || l[1].Kind != askBoth || l[1].From != t1.UnixMilli() || l[1].To != t1.Add(60*time.Minute).UnixMilli() {
		t.Fatalf("juntas (GMC antes): %+v", l)
	}
	// a pergunta da saída programada no mesmo tempo ganha
	t2 := t0.Add(10 * time.Hour)
	a.mu.Lock()
	a.addRcAwayLocked(RcAwayAsk{ID: t2.UnixMilli(), From: t2.UnixMilli(), To: t2.Add(time.Hour).UnixMilli(), Kind: askPlanned}, now)
	a.mu.Unlock()
	if n := a.gmcAwayAsks(gmcMem(t, a, t2, 60, 20), true); n != 0 {
		t.Fatal("perguntou por cima da saída programada")
	}
	// "Saí de casa…" com os dois aparelhos
	p, err := a.AnswerAway(l[0].ID, "sai", 0, 0, "", true, true)
	if err != nil || p.GMC != WhereMe || p.RC != WhereMe {
		t.Fatalf("trecho: %+v %v", p, err)
	}
}

func TestAutoHistoryGate(t *testing.T) {
	a := newTestApp(t)
	oldW, oldG, oldP := autoHistWait, autoHistGap, autoHistPoll
	autoHistWait, autoHistGap, autoHistPoll = 5*time.Millisecond, 300*time.Millisecond, 5*time.Millisecond
	t.Cleanup(func() { autoHistWait, autoHistGap, autoHistPoll = oldW, oldG, oldP })
	var calls atomic.Int32
	a.histDownload = func() (ImportResult, error) {
		calls.Add(1)
		return ImportResult{Added: 61, Complete: true}, nil
	}
	d := &gmc.Device{}
	a.dev = d

	a.autoHistory(d)
	if calls.Load() != 1 {
		t.Fatalf("não baixou: %d", calls.Load())
	}
	// reconectou logo: espera os 2 min (aqui 300 ms) desde o último
	start := time.Now()
	a.autoHistory(d)
	if calls.Load() != 2 || time.Since(start) < 250*time.Millisecond {
		t.Fatalf("não esperou o intervalo: %v", time.Since(start))
	}
	// gravando a configuração: espera terminar
	a.autoHistAt = time.Time{}
	a.cfgBusy.Store(true)
	go func() { time.Sleep(50 * time.Millisecond); a.cfgBusy.Store(false) }()
	a.autoHistory(d)
	if calls.Load() != 3 {
		t.Fatal("não baixou depois da configuração")
	}
	// desconectou (ou outra conexão) antes de baixar: não baixa
	a.autoHistAt = time.Time{}
	a.mu.Lock()
	a.dev = &gmc.Device{}
	a.mu.Unlock()
	a.autoHistory(d)
	// desligado nas Configurações: não começa
	a.mu.Lock()
	a.dev = d
	a.settings.NoAutoHistory = true
	a.mu.Unlock()
	a.autoHistory(d)
	if calls.Load() != 3 {
		t.Fatalf("baixou sem poder: %d", calls.Load())
	}
	// erro de leitura: avisa a tela
	a.settings.NoAutoHistory = false
	a.histDownload = func() (ImportResult, error) { return ImportResult{}, errors.New("o contador não respondeu") }
	var got []AutoHistory
	a.emitHook = func(name string, v any) {
		if name == "history-auto" {
			got = append(got, v.(AutoHistory))
		}
	}
	a.autoHistAt = time.Time{}
	a.autoHistory(d)
	a.histDownload = func() (ImportResult, error) { return ImportResult{Canceled: true}, errHistCanceled }
	a.autoHistAt = time.Time{}
	a.autoHistory(d)
	if len(got) != 2 || got[0].OK || got[0].Err == "" || !got[1].Canceled || got[1].Err != "" {
		t.Fatalf("avisos: %+v", got)
	}
}
