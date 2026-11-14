package app

import (
	"testing"
	"time"
)

// 2.81: com o fundo da casa e WaitObj, a medição espera em "fundo-pronto"
// (a pessoa procura o ponto) e só depois começa o objeto.
func TestMeasureWaitObj(t *testing.T) {
	a := newTestApp(t)
	a.demo.Active, a.demo.CPM = true, 25
	st, err := a.StartMeasure(MeasureOpts{Name: "pedra", Auto: true, HomeBg: true, WaitObj: true, MaxSec: 600, Pct: 20})
	if err != nil {
		t.Fatal(err)
	}
	if st.Phase != "fundo-pronto" || st.Bg == nil || st.Bg.Source != "casa" {
		t.Fatalf("início: %+v", st)
	}
	a.feedMeasure(0, 4, true) // procurando: não conta
	if st := a.GetMeasure(); st.Elapsed != 0 || st.Counts != 0 {
		t.Fatalf("contou antes do objeto: %+v", st)
	}
	st, err = a.StartMeasurePhase("objeto", 0, "")
	if err != nil || st.Phase != "objeto" || st.Seconds != 600 {
		t.Fatalf("objeto: %+v %v", st, err)
	}
}

func TestMeasureQuiet(t *testing.T) {
	a := newTestApp(t)
	now := time.Now()
	q := func(dev string) bool {
		a.mu.Lock()
		defer a.mu.Unlock()
		return a.measureQuietLocked(dev, now)
	}
	set := func(m MeasureState) { a.mu.Lock(); a.measure = m; a.mu.Unlock() }

	if q("gmc") || q(devRC) {
		t.Fatal("sem medição não cala")
	}
	set(MeasureState{Phase: "objeto", Device: "gmc"})
	if !q("gmc") || q(devRC) {
		t.Fatal("GMC no objeto: só ele cala")
	}
	set(MeasureState{Phase: "fundo", Device: devRC, Mode: "ambos", Other: &MeasureSide{Device: "gmc", Role: "objeto"}})
	if !q("gmc") || !q(devRC) {
		t.Fatal("os dois no objeto: os dois calam")
	}
	set(MeasureState{Phase: "objeto", Device: "gmc", Mode: "voce", Other: &MeasureSide{Device: devRC, Role: "voce"}})
	if !q("gmc") || q(devRC) {
		t.Fatal("o que fica com a pessoa continua com alarme")
	}
	set(MeasureState{Phase: "objeto", Device: "gmc", Demo: true})
	if q("gmc") {
		t.Fatal("medição no demo não cala o contador real")
	}
	set(MeasureState{Phase: "fundo-pronto", Device: "gmc", readyAt: now.Add(-10 * time.Minute)})
	if !q("gmc") {
		t.Fatal("procurando o ponto: cala")
	}
	set(MeasureState{Phase: "fundo-pronto", Device: "gmc", readyAt: now.Add(-measQuietIdle - time.Second)})
	if q("gmc") {
		t.Fatal("esqueceu a medição aberta: volta a valer")
	}
	set(MeasureState{Phase: "pronto", Device: "gmc", Obj: &MeasurePhase{End: now.Add(-20 * time.Second).UnixMilli()}})
	if !q("gmc") {
		t.Fatal("logo depois do resultado: ainda cala")
	}
	set(MeasureState{Phase: "pronto", Device: "gmc", Obj: &MeasurePhase{End: now.Add(-2 * time.Minute).UnixMilli()}})
	if q("gmc") {
		t.Fatal("um minuto depois do resultado: volta")
	}
}
