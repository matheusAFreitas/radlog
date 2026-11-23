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

var wdTestN int

// wdBlocked só existe para o teste ter uma goroutine do RadLog presa.
func (a *App) wdBlocked(mu *sync.Mutex) {
	mu.Lock()
	wdTestN++ // algo dentro da trava (staticcheck SA2001)
	mu.Unlock()
}

// 23/11 (dev.649): a nuvem juntando um AAAA-MM.csv pegava rc.mu dentro do
// fileM, e a tela (GetRadiacode) pega fileM dentro do rc.mu: travou. Com
// rc.mu ocupado, rcMergeFile tem de soltar o fileM antes de esperar por ele.
func TestRcMergeFileLockOrder(t *testing.T) {
	a := NewApp()
	a.dataDir = t.TempDir()
	csv := rcHeader + "\n" + time.Now().Truncate(time.Minute).Format(time.RFC3339) + ",1\n"
	a.rc.mu.Lock()
	done := make(chan int)
	go func() { done <- a.rcMergeFile(rcDirName+"/"+time.Now().Format("2006-01")+".csv", []byte(csv)) }()
	deadline := time.Now().Add(2 * time.Second)
	for {
		if _, err := os.Stat(filepath.Join(a.rcDir(), time.Now().Format("2006-01")+".csv")); err == nil {
			break
		}
		if time.Now().After(deadline) {
			a.rc.mu.Unlock()
			t.Fatal("não gravou com rc.mu ocupado")
		}
		time.Sleep(10 * time.Millisecond)
	}
	time.Sleep(20 * time.Millisecond)
	if !a.rc.fileM.TryLock() {
		a.rc.mu.Unlock()
		t.Fatal("fileM preso enquanto espera rc.mu (ordem das travas invertida)")
	}
	a.rc.fileM.Unlock()
	a.rc.mu.Unlock()
	if n := <-done; n != 1 {
		t.Fatalf("entraram %d minutos", n)
	}
}
