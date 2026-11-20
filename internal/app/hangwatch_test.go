package app

import (
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"
)

func TestWdCheck(t *testing.T) {
	var mu sync.Mutex
	locks := []*wdLock{{name: "x", mu: &mu}}
	t0 := time.Now()
	if wdCheck(locks, t0) != nil {
		t.Fatal("solta não é travamento")
	}
	mu.Lock()
	if wdCheck(locks, t0) != nil || wdCheck(locks, t0.Add(wdLimit-time.Second)) != nil {
		t.Fatal("cedo demais")
	}
	mu.Unlock()
	// soltou no meio: conta de novo
	if wdCheck(locks, t0.Add(wdLimit)) != nil {
		t.Fatal("soltou e ainda acusou")
	}
	mu.Lock()
	defer mu.Unlock()
	wdCheck(locks, t0.Add(wdLimit+time.Second))
	if l := wdCheck(locks, t0.Add(2*wdLimit+time.Second)); l == nil || l.name != "x" {
		t.Fatal("presa e não acusou")
	}
}

func TestWdStuckCall(t *testing.T) {
	done := WatchCall("Teste")
	if wdStuckCall(time.Now()) != nil {
		t.Fatal("chamada nova não é travamento")
	}
	if l := wdStuckCall(time.Now().Add(wdLimit + time.Second)); l == nil || l.name != "chamada Teste" {
		t.Fatalf("não acusou: %+v", l)
	}
	done()
	if wdStuckCall(time.Now().Add(wdLimit+time.Second)) != nil {
		t.Fatal("terminou e ainda acusou")
	}
}

func TestWdFireAndReport(t *testing.T) {
	a := NewApp()
	a.dataDir = t.TempDir()
	exited := false
	old := wdExit
	wdExit = func() { exited = true }
	defer func() { wdExit = old }()
	var mu sync.Mutex
	mu.Lock()
	stuck := make(chan struct{})
	go func() { a.wdBlocked(&mu); close(stuck) }()
	time.Sleep(50 * time.Millisecond)
	a.wdFire(&wdLock{name: "trava teste", held: time.Now()})
	if !exited {
		t.Fatal("não fechou")
	}
	if _, err := os.Stat(filepath.Join(a.dataDir, wdDumpFile)); err != nil {
		t.Fatal("sem retrato")
	}
	a.wdReport()
	mu.Unlock()
	<-stuck
	b, _ := os.ReadFile(a.logPath())
	log := string(b)
	if !strings.Contains(log, "o app tinha travado") || !strings.Contains(log, "app.(*App).wdBlocked") {
		t.Fatalf("resumo faltando no log:\n%s", log)
	}
	if _, err := os.Stat(filepath.Join(a.dataDir, "travamento-anterior.txt")); err != nil {
		t.Fatal("não guardou o anterior")
	}
}

// wdBlocked só existe para o teste ter uma goroutine do RadLog presa.
func (a *App) wdBlocked(mu *sync.Mutex) { mu.Lock(); mu.Unlock() }
