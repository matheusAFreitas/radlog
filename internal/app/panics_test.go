package app

import (
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

func TestPanicBackoff(t *testing.T) {
	want := map[int]time.Duration{1: 5 * time.Second, 2: 10 * time.Second, 3: 20 * time.Second, 7: 5 * time.Minute, 50: 5 * time.Minute}
	for n, d := range want {
		if got := panicBackoff(n); got != d {
			t.Errorf("panicBackoff(%d) = %s, quero %s", n, got, d)
		}
	}
}

func TestRunLoopRestartsAfterPanic(t *testing.T) {
	a := newTestApp(t)
	var runs int32
	var waits []time.Duration
	a.runLoop("testeReinicia", func() {
		if atomic.AddInt32(&runs, 1) < 3 {
			panic("pacote cortado")
		}
		// terceira vez: termina normalmente, o laço não reinicia mais
	}, func(d time.Duration) { waits = append(waits, d) })
	if runs != 3 {
		t.Fatalf("rodou %d vezes, quero 3", runs)
	}
	if len(waits) != 2 || waits[0] != 5*time.Second || waits[1] != 10*time.Second {
		t.Fatalf("esperas %v", waits)
	}
	b, _ := os.ReadFile(filepath.Join(a.dataDir, "log.txt"))
	if !strings.Contains(string(b), "ERRO interno em testeReinicia: pacote cortado") || !strings.Contains(string(b), "goroutine") {
		t.Fatalf("log sem o panic e a pilha:\n%s", b)
	}
	n := 0
	for _, e := range a.events {
		if strings.Contains(e.Text, "Erro interno no app (testeReinicia)") {
			n++
		}
	}
	if n != 2 {
		t.Fatalf("%d eventos de erro interno, quero 2", n)
	}
}

func TestRunLoopGivesUp(t *testing.T) {
	a := newTestApp(t)
	var runs int32
	a.runLoop("testeDesiste", func() { atomic.AddInt32(&runs, 1); panic("sempre") }, func(time.Duration) {})
	if runs != panicMax {
		t.Fatalf("rodou %d vezes, quero %d", runs, panicMax)
	}
	last := a.events[len(a.events)-1].Text
	if !strings.Contains(last, "Erro interno repetido no app (testeDesiste)") {
		t.Fatalf("último evento: %q", last)
	}
}

func TestRunLoopStopsWhenClosing(t *testing.T) {
	a := newTestApp(t)
	a.closing.Store(true)
	var runs int32
	a.runLoop("testeFechando", func() { atomic.AddInt32(&runs, 1); panic("x") }, func(time.Duration) { t.Fatal("não devia esperar") })
	if runs != 1 {
		t.Fatalf("rodou %d vezes", runs)
	}
}

func TestGoSafeRecovers(t *testing.T) {
	a := newTestApp(t)
	done := make(chan struct{})
	a.goSafe("testeUmaVez", func() { defer close(done); panic("teste") })
	<-done
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		a.mu.Lock()
		n := len(a.events)
		a.mu.Unlock()
		if n > 0 {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatal("o panic não virou evento")
}
