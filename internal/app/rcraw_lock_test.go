package app

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

// 23/11 (Android dev.649, o PC junta a nuvem igual): a nuvem juntando um
// AAAA-MM.csv pegava rc.mu dentro do fileM, e a tela (GetRadiacode) pega
// fileM dentro do rc.mu: travou. Com rc.mu ocupado, rcMergeFile tem de
// soltar o fileM antes de esperar por ele.
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
