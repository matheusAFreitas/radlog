package app

import (
	"testing"
	"time"

	"radlog/internal/store"
)

// plannedData grava RC102 e GMC-800 juntos de from por mins minutos; ratio é
// a razão de contagens RC102/GMC (28 em casa)
func plannedData(t *testing.T, a *App, from time.Time, mins int, ratio float64, rcSrc string) {
	t.Helper()
	const gmcCPM = 13.0
	usv := gmcCPM * ratio / 3600 // o rcWriteTest grava CPS = usv*60
	rcWriteTest(t, a, from, mins, usv, rcSrc)
	pts := make([]store.Point, mins)
	for i := range pts {
		pts[i] = store.Point{T: from.Add(time.Duration(i) * time.Minute), CPM: gmcCPM, Src: "historico"}
	}
	if _, err := a.store.ImportNew(pts); err != nil {
		t.Fatal(err)
	}
}

func nearMs(a, b int64, tol time.Duration) bool {
	d := time.Duration(a-b) * time.Millisecond
	return d >= -tol && d <= tol
}

func TestPlannedSave(t *testing.T) {
	a := newTestApp(t)
	future := time.Now().Add(10 * time.Hour).Truncate(time.Minute).UnixMilli()
	if _, err := a.SavePlannedTrip(PlannedTrip{Name: "x", From: future}); err == nil {
		t.Fatal("aceitou sem aparelho")
	}
	if _, err := a.SavePlannedTrip(PlannedTrip{From: time.Now().Add(-3 * time.Hour).UnixMilli(), RC: true}); err == nil {
		t.Fatal("aceitou horário que já passou")
	}
	p, err := a.SavePlannedTrip(PlannedTrip{Name: "  Escritório ", From: future, RC: true})
	if err != nil || p.ID == 0 || p.Name != "Escritório" {
		t.Fatalf("salvar: %+v %v", p, err)
	}
	// a desconexão na janela marca o começo; fora dela, não
	a.plannedDisconnected(devRC, time.UnixMilli(future).Add(-2*time.Hour))
	if l := a.GetPlannedTrips(); len(l) != 1 || l[0].Left != 0 {
		t.Fatalf("desconexão fora da janela contou: %+v", l)
	}
	a.plannedDisconnected("gmc", time.UnixMilli(future).Add(5*time.Minute))
	if l := a.GetPlannedTrips(); l[0].Left != 0 {
		t.Fatal("o GMC-800 não vai e contou")
	}
	a.plannedDisconnected(devRC, time.UnixMilli(future).Add(6*time.Minute))
	if l := a.GetPlannedTrips(); l[0].Left == 0 || l[0].LeftDev != devRC {
		t.Fatalf("não marcou a desconexão: %+v", l)
	}
	if err := a.NotLeaving(p.ID); err != nil || a.GetPlannedTrips()[0].Left != 0 {
		t.Fatal("\"Não estou saindo\" não limpou")
	}
	// sobrevive a reabrir o app
	b := NewApp()
	b.dataDir, b.settings = a.dataDir, defaultSettings()
	if l := b.GetPlannedTrips(); len(l) != 1 || l[0].Name != "Escritório" {
		t.Fatalf("depois de reabrir: %+v", l)
	}
	if err := a.DeletePlannedTrip(p.ID); err != nil || len(a.GetPlannedTrips()) != 0 {
		t.Fatal("cancelar não apagou")
	}
	// vencida há mais de uma semana some sozinha
	a.mu.Lock()
	a.planned = []PlannedTrip{{ID: 1, Name: "velha", From: time.Now().Add(-8 * 24 * time.Hour).UnixMilli(), RC: true}}
	a.mu.Unlock()
	if len(a.GetPlannedTrips()) != 0 {
		t.Fatal("programada vencida ficou")
	}
}

// PC desligado: o RC102 saiu com você, a saída sai da razão RC/GMC e a volta
// é a reconexão, com a dica de que chegou antes
func TestPlannedByRatio(t *testing.T) {
	a := newTestApp(t)
	plan := time.Now().Add(-12 * time.Hour).Truncate(time.Hour)
	left, home, conn := plan.Add(11*time.Minute), plan.Add(8*time.Hour), plan.Add(10*time.Hour)
	start := plan.Add(-74 * time.Hour)
	plannedData(t, a, start, int(left.Sub(start)/time.Minute), 28, "")
	plannedData(t, a, left, int(home.Sub(left)/time.Minute), 40, rcMemSrc)
	plannedData(t, a, home, int(conn.Sub(home)/time.Minute), 28, rcMemSrc)
	a.mu.Lock()
	a.planned = []PlannedTrip{{ID: 7, Name: "Escritório", From: plan.UnixMilli(), RC: true}}
	a.mu.Unlock()
	// uma pergunta normal já feita do mesmo trecho é trocada pela da programada
	a.mu.Lock()
	a.addRcAwayLocked(RcAwayAsk{ID: 1, From: plan.Add(-6 * time.Hour).UnixMilli(), To: conn.UnixMilli()}, time.Now())
	a.mu.Unlock()

	a.rcAwayDetect(plan.Add(-6*time.Hour), conn)
	l := a.GetRcAwayAsks()
	if len(l) != 1 || l[0].Kind != askPlanned {
		t.Fatalf("perguntas: %+v", l)
	}
	q := l[0]
	if q.Source != "razao" || q.Estimated || q.Missed || !nearMs(q.From, left.UnixMilli(), 5*time.Minute) || q.To != conn.UnixMilli() {
		t.Fatalf("saída: %s %v %v, achou %s (quer %s)", q.Source, q.Estimated, q.Missed, time.UnixMilli(q.From).Format("15:04"), left.Format("15:04"))
	}
	if !nearMs(q.BackHint, home.UnixMilli(), 5*time.Minute) {
		t.Fatalf("dica da volta: %v (quer %s)", time.UnixMilli(q.BackHint).Format("15:04"), home.Format("15:04"))
	}
	if len(a.GetPlannedTrips()) != 0 {
		t.Fatal("a programada continuou depois de virar pergunta")
	}
	// "Não saí" e desfazer devolve a pergunta
	if p, err := a.AnswerPlanned(q.ID, "naosai", 0, 0, "", false, true); err != nil || p.ID != 0 {
		t.Fatal(err)
	}
	if err := a.UndoRcAway(q.RcAwayAsk, 0); err != nil || len(a.GetRcAwayAsks()) != 1 {
		t.Fatal("desfazer não devolveu a pergunta")
	}
	// "Saí": o trecho com a volta da dica e o nome da programada
	p, err := a.AnswerPlanned(q.ID, "sai", q.From, q.BackHint, "", false, true)
	if err != nil {
		t.Fatal(err)
	}
	if p.Name != "Escritório" || p.Planned != 7 || p.Estimated || p.GMC != WhereHome || p.RC != WhereMe || !p.NotHome || p.To != q.BackHint {
		t.Fatalf("trecho: %+v", p)
	}
	if len(a.GetRcAwayAsks()) != 0 {
		t.Fatal("pergunta ficou")
	}
	// editar o nome na lista não perde a marca de programada
	p.Planned = 0
	p.Name = "Trabalho"
	if p2, _ := a.SavePeriod(p); p2.Planned != 7 {
		t.Fatalf("perdeu a marca: %+v", p2)
	}
}

// a razão não mudou: a pergunta vem como "não aconteceu?", com o horário
// programado estimado
func TestPlannedMissed(t *testing.T) {
	a := newTestApp(t)
	plan := time.Now().Add(-12 * time.Hour).Truncate(time.Hour)
	conn := plan.Add(6 * time.Hour)
	start := plan.Add(-74 * time.Hour)
	plannedData(t, a, start, int(conn.Sub(start)/time.Minute), 28, "")
	a.mu.Lock()
	a.planned = []PlannedTrip{{ID: 9, Name: "Praia", From: plan.UnixMilli(), RC: true}}
	a.mu.Unlock()
	if !a.plannedOnConnect(devRC, conn) {
		t.Fatal("não tratou a programada")
	}
	l := a.GetRcAwayAsks()
	if len(l) != 1 || !l[0].Missed || !l[0].Estimated || l[0].Source != "plano" || l[0].From != plan.UnixMilli() {
		t.Fatalf("pergunta: %+v", l)
	}
	p, err := a.AnswerPlanned(l[0].ID, "sai", 0, 0, "", false, true)
	if err != nil || !p.Estimated || p.From != plan.UnixMilli() || p.To != conn.UnixMilli() {
		t.Fatalf("trecho estimado: %+v %v", p, err)
	}
}

// PC ligado: a desconexão na janela é a saída; uma queda curta do Bluetooth
// não conta; antes de passar a janela, ligar o PC não dispara a pergunta
func TestPlannedByDisconnect(t *testing.T) {
	a := newTestApp(t)
	plan := time.Now().Add(-12 * time.Hour).Truncate(time.Hour)
	a.mu.Lock()
	a.planned = []PlannedTrip{{ID: 3, Name: "Escritório", From: plan.UnixMilli(), RC: true}}
	a.mu.Unlock()
	if a.plannedOnConnect(devRC, plan.Add(-20*time.Minute)) {
		t.Fatal("conectou antes da saída e perguntou")
	}
	a.plannedDisconnected(devRC, plan.Add(-10*time.Minute))
	if a.plannedOnConnect(devRC, plan.Add(-8*time.Minute)) {
		t.Fatal("queda de 2 min virou saída")
	}
	if a.GetPlannedTrips()[0].Left != 0 {
		t.Fatal("a queda curta ficou marcada")
	}
	a.plannedDisconnected(devRC, plan.Add(6*time.Minute))
	conn := plan.Add(9 * time.Hour)
	if !a.plannedOnConnect(devRC, conn) {
		t.Fatal("não perguntou na volta")
	}
	l := a.GetRcAwayAsks()
	if len(l) != 1 || l[0].Source != "desconexao" || l[0].From != plan.Add(6*time.Minute).UnixMilli() || l[0].To != conn.UnixMilli() || l[0].Estimated {
		t.Fatalf("pergunta: %+v", l)
	}
	// trecho marcado à mão no mesmo horário ganha
	b := newTestApp(t)
	b.mu.Lock()
	b.planned = []PlannedTrip{{ID: 4, Name: "x", From: plan.UnixMilli(), RC: true, Left: plan.UnixMilli()}}
	b.mu.Unlock()
	if _, err := b.SavePeriod(Period{From: plan.Add(-time.Hour).UnixMilli(), To: plan.Add(2 * time.Hour).UnixMilli(), GMC: WhereHome, RC: WhereMe, NotHome: true}); err != nil {
		t.Fatal(err)
	}
	if !b.plannedOnConnect(devRC, conn) || len(b.GetRcAwayAsks()) != 0 || len(b.GetPlannedTrips()) != 0 {
		t.Fatal("a programada passou por cima do trecho marcado à mão")
	}
}

// o backup leva as programadas e junta só as que faltam
func TestPlannedBackup(t *testing.T) {
	a := newTestApp(t)
	b := newTestApp(t)
	future := time.Now().Add(5 * time.Hour).Truncate(time.Minute).UnixMilli()
	if _, err := a.SavePlannedTrip(PlannedTrip{Name: "Praia", From: future, GMC: true, RC: true}); err != nil {
		t.Fatal(err)
	}
	zp := t.TempDir() + "/b.zip"
	if err := a.writeBackupFile(zp); err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 2; i++ {
		if _, err := b.mergeZip(zp, true); err != nil {
			t.Fatal(err)
		}
	}
	if l := b.GetPlannedTrips(); len(l) != 1 || l[0].Name != "Praia" || !l[0].GMC {
		t.Fatalf("depois do backup: %+v", l)
	}
}
