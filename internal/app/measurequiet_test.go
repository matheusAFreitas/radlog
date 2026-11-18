package app

import (
	"math"
	"testing"
	"time"
)

// 2.81: com o fundo da casa e WaitObj, a medição espera em "fundo-pronto"
// (a pessoa procura o ponto) e só depois começa o objeto.
func TestMeasureWaitObj(t *testing.T) {
	a := newTestApp(t)
	fakeGMC(t, a, 25)
	st, err := a.StartMeasure(MeasureOpts{Name: "pedra", Auto: true, HomeBg: true, WaitObj: true, MaxSec: 600, Pct: 20})
	if err != nil {
		t.Fatal(err)
	}
	if st.Phase != "fundo-pronto" || st.Bg == nil || st.Bg.Source != "casa" {
		t.Fatalf("início: %+v", st)
	}
	a.feedMeasure(0, 4) // procurando: não conta
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
	// q: cala tudo (inclusive Perigo); act: cala atenção e alarme
	q := func(dev string) bool {
		a.mu.Lock()
		defer a.mu.Unlock()
		return a.measureQuietLocked(dev, now)
	}
	act := func() bool { a.mu.Lock(); defer a.mu.Unlock(); return a.measureActiveLocked(now) }
	set := func(m MeasureState) { a.mu.Lock(); a.measure = m; a.mu.Unlock() }

	if q("gmc") || q(devRC) || act() {
		t.Fatal("sem medição não cala")
	}
	set(MeasureState{Phase: "objeto", Device: "gmc"})
	if !q("gmc") || q(devRC) || !act() {
		t.Fatal("GMC no objeto: ele cala tudo; o RC102 só atenção e alarme (2.100.2)")
	}
	set(MeasureState{Phase: "objeto", Device: devRC})
	if q("gmc") || !q(devRC) || !act() {
		t.Fatal("RC102 no objeto: o Perigo do GMC continua valendo (2.100.2)")
	}
	set(MeasureState{Phase: "fundo", Device: devRC, Mode: "ambos", Other: &MeasureSide{Device: "gmc", Role: "objeto"}})
	if !q("gmc") || !q(devRC) {
		t.Fatal("os dois no objeto: os dois calam")
	}
	set(MeasureState{Phase: "objeto", Device: "gmc", Mode: "voce", Other: &MeasureSide{Device: devRC, Role: "voce"}})
	if !q("gmc") || q(devRC) || !act() {
		t.Fatal("o que fica com a pessoa: Perigo continua")
	}
	set(MeasureState{Phase: "", Device: "gmc"})
	if q("gmc") || act() {
		t.Fatal("sem medição não cala")
	}
	set(MeasureState{Phase: "fundo-pronto", Device: "gmc", readyAt: now.Add(-10 * time.Minute)})
	if !q("gmc") || !act() {
		t.Fatal("procurando o ponto: cala")
	}
	set(MeasureState{Phase: "fundo-pronto", Device: "gmc", readyAt: now.Add(-measQuietIdle - time.Second)})
	if q("gmc") || act() {
		t.Fatal("esqueceu a medição aberta: volta a valer")
	}
	set(MeasureState{Phase: "pronto", Device: "gmc", Obj: &MeasurePhase{End: now.Add(-20 * time.Second).UnixMilli()}})
	if !q("gmc") || !act() {
		t.Fatal("logo depois do resultado: ainda cala")
	}
	set(MeasureState{Phase: "pronto", Device: "gmc", Obj: &MeasurePhase{End: now.Add(-2 * time.Minute).UnixMilli()}})
	if q("gmc") || act() {
		t.Fatal("um minuto depois do resultado: volta")
	}
}

// 2.100.2: medindo com o GMC-800, o RC102 cala atenção e alarme, mas o
// Perigo dele continua valendo.
func TestMeasureOtherDanger(t *testing.T) {
	a := newTestApp(t)
	now := time.Now()
	a.mu.Lock()
	a.measure = MeasureState{Phase: "objeto", Device: "gmc"}
	a.mu.Unlock()
	l := a.rcLimits()
	a.rcAlertStep(now, l.Alarm*1.5)
	a.mu.Lock()
	attn, alarm := a.rcAlert.attnOn, a.rcAlert.alarmOn
	a.mu.Unlock()
	if attn || alarm {
		t.Fatalf("medindo: atenção %v alarme %v", attn, alarm)
	}
	a.rcAlertStep(now.Add(time.Second), l.Danger*1.2)
	a.mu.Lock()
	dang := a.rcAlert.dangOn
	a.mu.Unlock()
	if !dang {
		t.Fatal("o Perigo do outro aparelho continua valendo")
	}
	// o RC102 no objeto: aí cala tudo
	a.rcAlertStep(now.Add(2*time.Second), 0)
	a.mu.Lock()
	a.measure = MeasureState{Phase: "objeto", Device: devRC}
	a.mu.Unlock()
	a.rcAlertStep(now.Add(3*time.Second), l.Danger*1.2)
	a.mu.Lock()
	dang = a.rcAlert.dangOn
	a.mu.Unlock()
	if dang {
		t.Fatal("o aparelho no objeto cala o Perigo")
	}
}

// 2.100.2 (sal light): no RC102 a contagem não muda, mas a dose sobe muito
// acima da incerteza da própria dose: o resumo diz isso.
func TestMeasureDoseUp(t *testing.T) {
	bg := MeasurePhase{Counts: 41650, Seconds: 3420, CPM: 41650 / 57.0, ErrCPM: 3.58, USv: 0.169, ErrUSv: 0.0007}
	obj := MeasurePhase{Counts: 43645, Seconds: 3600, CPM: 43645 / 60.0, ErrCPM: 3.48, USv: 0.188, ErrUSv: 0.0012}
	r := compareMeasure(bg, obj, nil)
	if r.Level != "nada" || !r.DoseUp || r.DoseSigma < 10 || r.Verdict != "a contagem não mudou, mas a dose subiu" {
		t.Fatalf("sal: %+v", r)
	}
	// GMC (sem incerteza da dose): como antes
	bg.ErrUSv, obj.ErrUSv = 0, 0
	if r := compareMeasure(bg, obj, nil); r.DoseUp || r.NetErrUSv != 0 {
		t.Fatalf("GMC: %+v", r)
	}
	if e := meanErr(10, 50, 4); e < 1.443 || e > 1.444 { // √((50−25)/3 ÷ 4)
		t.Fatalf("meanErr %v", e)
	}
}

// 2.100.2: com "fica calado" no Preparar, o Perigo do outro aparelho também cala.
func TestMeasureQuietDangerOpt(t *testing.T) {
	a := newTestApp(t)
	now := time.Now()
	a.mu.Lock()
	a.measure = MeasureState{Phase: "objeto", Device: "gmc", QuietDanger: true}
	a.mu.Unlock()
	l := a.rcLimits()
	a.rcAlertStep(now, l.Danger*1.2)
	a.mu.Lock()
	dang := a.rcAlert.dangOn
	q := a.measureQuietLocked(devRC, now)
	a.mu.Unlock()
	if dang || !q {
		t.Fatalf("Perigo calado pela opção: dangOn %v quiet %v", dang, q)
	}
	// sem medição, a opção não vale
	a.mu.Lock()
	a.measure = MeasureState{QuietDanger: true}
	q = a.measureQuietLocked(devRC, now)
	a.mu.Unlock()
	if q {
		t.Fatal("sem medição nada cala")
	}
}

// 2.101.1: µSv/h suavizado (segundos seguidos parecidos): a incerteza vem
// das médias de blocos de 30 s, não dos segundos como se fossem independentes.
func TestMeasBlocks(t *testing.T) {
	var b measBlocks
	var sum, sq float64
	n := 0
	for k := 0; k < 8; k++ {
		v := 1.0 + float64(k%2)
		for i := 0; i < measBlkSec; i++ {
			b.add(v)
			sum, sq, n = sum+v, sq+v*v, n+1
		}
	}
	naive := meanErr(sum, sq, n)
	if e := b.err(sum, sq, n); e < 4*naive || math.Abs(e-meanErr(12, 20, 8)) > 1e-9 {
		t.Fatalf("blocos %v, segundos %v", e, naive)
	}
	var few measBlocks
	few.add(1)
	if e := few.err(10, 50, 4); math.Abs(e-meanErr(10, 50, 4)*math.Sqrt(measBlkSec)) > 1e-9 {
		t.Fatalf("poucos blocos: %v", e)
	}
}
