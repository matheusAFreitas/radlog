package app

import (
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"testing"
)

// Pedido que chega bem quando a tarefa termina a fila não pode ficar parado
// (QA da 2.78.0): depois do flushWrites, cada arquivo tem a última versão.
func TestWriteLaterNoLostWrite(t *testing.T) {
	a := newTestApp(t)
	dir := t.TempDir()
	for round := 0; round < 300; round++ {
		var wg sync.WaitGroup
		for f := 0; f < 3; f++ {
			wg.Add(1)
			go func(f int) {
				defer wg.Done()
				p := filepath.Join(dir, fmt.Sprintf("f%d.json", f))
				a.writeLater(p, []byte(fmt.Sprint(round)), 0o600, "teste")
			}(f)
		}
		wg.Wait()
		a.flushWrites()
		for f := 0; f < 3; f++ {
			b, err := os.ReadFile(filepath.Join(dir, fmt.Sprintf("f%d.json", f)))
			if err != nil || string(b) != fmt.Sprint(round) {
				t.Fatalf("rodada %d, f%d: %q %v", round, f, b, err)
			}
		}
	}
	a.later.mu.Lock()
	defer a.later.mu.Unlock()
	if len(a.later.order) != 0 || a.later.running {
		t.Fatalf("fila sobrando: %v running=%v", a.later.order, a.later.running)
	}
}
