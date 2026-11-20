package app

import (
	"bufio"
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"time"
)

// ---------- vigia de travamento (celular) ----------
//
// No dev.617 (19/11) o "Sincronizar agora" ficou em "Sincronizando" e a aba
// Ajustes parou de responder até reinstalar; no celular não dá para ver
// onde travou. A vigia tenta pegar as travas principais de
// tempos em tempos; se uma delas fica presa em todas as tentativas por
// wdLimit, grava onde cada goroutine estava (travamento.txt) e fecha o
// processo. O serviço em primeiro plano (START_STICKY) sobe de novo e, no
// início, o resumo do travamento vai para o log.txt, que o ACE copia.

const (
	wdEvery    = 15 * time.Second
	wdLimit    = 5 * time.Minute
	wdDumpFile = "travamento.txt"
	wdDumpMax  = 4 << 20
)

// wdExit fecha o processo (troca nos testes).
var wdExit = func() { os.Exit(3) }

type wdLock struct {
	name string
	mu   *sync.Mutex
	held time.Time // desde quando está presa (zero = soltou na última vez)
}

func (a *App) wdLocks() []*wdLock {
	return []*wdLock{
		{name: "trava app", mu: &a.mu},
		{name: "trava rc", mu: &a.rc.mu},
		{name: "trava rc.arquivo", mu: &a.rc.fileM},
		{name: "trava nuvem", mu: &a.cloud.mu},
		{name: "trava log", mu: &logMu},
	}
}

// wdCheck tenta cada trava e devolve a primeira presa há wdLimit ou mais.
func wdCheck(locks []*wdLock, now time.Time) *wdLock {
	for _, l := range locks {
		if l.mu.TryLock() {
			l.mu.Unlock()
			l.held = time.Time{}
			continue
		}
		if l.held.IsZero() {
			l.held = now
		} else if now.Sub(l.held) >= wdLimit {
			return l
		}
	}
	return nil
}

// Chamadas da tela em andamento (o Kotlin roda cada uma numa thread; uma
// presa para sempre também é travamento).
var (
	wdCallsMu sync.Mutex
	wdCalls   = map[int]wdCall{}
	wdCallN   int
)

type wdCall struct {
	name string
	at   time.Time
}

// WatchCall marca o início de uma chamada da tela; a função devolvida marca
// o fim.
func WatchCall(name string) func() {
	wdCallsMu.Lock()
	wdCallN++
	id := wdCallN
	wdCalls[id] = wdCall{name, time.Now()}
	wdCallsMu.Unlock()
	return func() {
		wdCallsMu.Lock()
		delete(wdCalls, id)
		wdCallsMu.Unlock()
	}
}

// wdStuckCall devolve a chamada mais antiga em andamento há wdLimit ou mais.
func wdStuckCall(now time.Time) *wdLock {
	wdCallsMu.Lock()
	defer wdCallsMu.Unlock()
	var old *wdCall
	for _, c := range wdCalls {
		if now.Sub(c.at) >= wdLimit && (old == nil || c.at.Before(old.at)) {
			c := c
			old = &c
		}
	}
	if old == nil {
		return nil
	}
	return &wdLock{name: "chamada " + old.name, held: old.at}
}

func (a *App) watchdogLoop() {
	locks := a.wdLocks()
	for !a.closing.Load() {
		time.Sleep(wdEvery)
		now := time.Now()
		l := wdCheck(locks, now)
		if l == nil {
			l = wdStuckCall(now)
		}
		if l != nil {
			a.wdFire(l)
			return
		}
	}
}

// wdFire grava o retrato das goroutines e fecha o processo. Não usa o
// logf direto: a trava presa pode ser a do log.
func (a *App) wdFire(l *wdLock) {
	buf := make([]byte, wdDumpMax)
	buf = buf[:runtime.Stack(buf, true)]
	head := fmt.Sprintf("%s travou: %s presa desde %s (versão %s)\n\n",
		time.Now().Format("2006-01-02 15:04:05"), l.name, l.held.Format("15:04:05"), runVersion())
	_ = os.WriteFile(filepath.Join(a.dataDir, wdDumpFile), append([]byte(head), buf...), 0o644)
	done := make(chan struct{})
	go func() {
		a.logf("ERRO motor travado (%s presa desde %s): reiniciando o app", l.name, l.held.Format("15:04:05"))
		close(done)
	}()
	select {
	case <-done:
	case <-time.After(2 * time.Second):
	}
	wdExit()
}

// wdReport, no início, passa o resumo de um travamento anterior para o log
// e renomeia o arquivo (fica o último em travamento-anterior.txt).
func (a *App) wdReport() {
	p := filepath.Join(a.dataDir, wdDumpFile)
	b, err := os.ReadFile(p)
	if err != nil {
		return
	}
	_ = os.Rename(p, filepath.Join(a.dataDir, "travamento-anterior.txt"))
	head, _, _ := strings.Cut(string(b), "\n")
	a.logf("ERRO o app tinha travado: %s", head)
	for _, s := range wdSummary(b) {
		a.logf("  travamento: %s", s)
	}
}

// wdSummary tira do retrato as goroutines paradas esperando trava ou canal,
// com as funções do RadLog de cada uma (no máximo 60 linhas).
func wdSummary(dump []byte) []string {
	var out []string
	sc := bufio.NewScanner(bytes.NewReader(dump))
	sc.Buffer(make([]byte, 64<<10), 1<<20)
	var cur []string
	keep := false
	flush := func() {
		if keep && len(cur) > 0 {
			out = append(out, strings.Join(cur, " ← "))
		}
		cur, keep = nil, false
	}
	for sc.Scan() {
		t := sc.Text()
		switch {
		case strings.HasPrefix(t, "goroutine "):
			flush()
			st := t
			if i := strings.Index(t, "["); i >= 0 {
				st = strings.TrimSuffix(t[i+1:], "]:")
			}
			keep = strings.Contains(st, "sync.Mutex") || strings.Contains(st, "chan ") ||
				strings.Contains(st, "select") || strings.Contains(st, "semacquire") || strings.Contains(st, "IO wait")
			cur = []string{st}
		case strings.HasPrefix(t, "radlog/") && len(cur) < 7:
			fn := t
			if i := strings.LastIndex(fn, "("); i > 0 {
				fn = fn[:i]
			}
			cur = append(cur, strings.TrimPrefix(fn, "radlog/internal/"))
		}
	}
	flush()
	// só as que passam pelo RadLog
	var res []string
	for _, s := range out {
		if strings.Contains(s, " ← ") {
			res = append(res, s)
		}
		if len(res) == 60 {
			break
		}
	}
	return res
}
