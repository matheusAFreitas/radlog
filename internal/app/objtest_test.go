package app

import (
	"testing"
	"time"
)

// 2.89.0: "Estou testando um objeto" cala atenção e alarme do aparelho
// escolhido; o perigo continua valendo.
func TestObjectTestQuiet(t *testing.T) {
	a := newTestApp(t)
	if _, err := a.TestObject("outro"); err == nil {
		t.Fatal("aparelho desconhecido devia dar erro")
	}
	until, err := a.TestObject(devRC)
	if err != nil || until <= time.Now().UnixMilli() {
		t.Fatalf("TestObject: %v %v", until, err)
	}
	if m := a.ObjectTests(); m[devRC] != until || len(m) != 1 {
		t.Fatalf("ObjectTests: %+v", m)
	}
	l := a.rcLimits()
	now := time.Now()
	a.rcAlertStep(now, l.Alarm*1.5)
	a.mu.Lock()
	attn, alarm := a.rcAlert.attnOn, a.rcAlert.alarmOn
	a.mu.Unlock()
	if attn || alarm {
		t.Fatalf("testando: atenção %v alarme %v", attn, alarm)
	}
	a.rcAlertStep(now.Add(time.Second), l.Danger*1.2)
	a.mu.Lock()
	dang := a.rcAlert.dangOn
	a.mu.Unlock()
	if !dang {
		t.Fatal("o perigo continua valendo")
	}
	a.rcAlertStep(now.Add(2*time.Second), 0)
	a.StopTestObject(devRC)
	if len(a.ObjectTests()) != 0 {
		t.Fatal("parar devolve os alertas")
	}
	a.rcAlertStep(now.Add(3*time.Second), l.Alarm*1.5)
	a.mu.Lock()
	alarm = a.rcAlert.alarmOn
	a.mu.Unlock()
	if !alarm {
		t.Fatal("depois de parar, o alarme volta")
	}
	// passou o tempo: deixa de calar sozinho
	a.mu.Lock()
	a.objTest = map[string]time.Time{"gmc": now.Add(-time.Second)}
	q := a.objTestQuietLocked("gmc", now)
	a.mu.Unlock()
	if q {
		t.Fatal("venceu: não cala mais")
	}
}
