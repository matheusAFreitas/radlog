package app

import (
	"encoding/json"
	"os"
	"path/filepath"
	"sync"

	"radlog/internal/store"
)

// writeFileAtomic grava data em path sem nunca deixar o arquivo pela metade:
// escreve num temporário ao lado, força a gravação no disco e só então
// troca o nome. Se a energia cair no meio, fica o arquivo antigo inteiro.
func writeFileAtomic(path string, data []byte, perm os.FileMode) error {
	f, err := os.CreateTemp(filepath.Dir(path), filepath.Base(path)+".*.tmp")
	if err != nil {
		return err
	}
	tmp := f.Name()
	_, err = f.Write(data)
	if err == nil {
		err = f.Sync()
	}
	if cerr := f.Close(); err == nil {
		err = cerr
	}
	if err == nil {
		err = os.Chmod(tmp, perm)
	}
	if err == nil {
		err = store.RenameRetry(tmp, path)
	}
	if err != nil {
		os.Remove(tmp)
	}
	return err
}

// readStateJSON lê um JSON de estado (saídas, anotações, medições) em v.
// Se o arquivo existe mas está estragado, guarda uma cópia em .ilegivel e
// avisa em Eventos, em vez de começar vazio em silêncio e sobrescrever o
// arquivo na próxima gravação. Devolve false se não leu.
func (a *App) readStateJSON(path, what string, v any) bool {
	b, err := os.ReadFile(path)
	if err != nil {
		return false
	}
	if jerr := json.Unmarshal(b, v); jerr != nil {
		bad := path + ".ilegivel"
		_ = os.WriteFile(bad, b, 0o644)
		a.logf("%s: não consegui ler o %s (%v); comecei vazio e guardei o arquivo em %s", what, filepath.Base(path), jerr, bad)
		return false
	}
	return true
}

// ---------- gravação fora da trava (auditoria 2.76.0, P2) ----------
//
// Algumas gravações (config.json, uso.json) acontecem com a.mu presa, a
// mesma trava que a leitura por segundo usa: num disco lento a leitura ao
// vivo esperava o disco. writeLater guarda os bytes (montados com a trava)
// e uma tarefa só grava depois, na ordem, sempre a versão mais nova de cada
// arquivo. flushWrites espera tudo ir para o disco (fechamento e testes).

type laterWrites struct {
	mu      sync.Mutex
	order   []string          // arquivos na ordem do primeiro pedido
	data    map[string][]byte // a versão mais nova de cada um
	perm    map[string]os.FileMode
	what    map[string]string // o que é cada arquivo, para o log
	running bool
	idle    *sync.Cond
}

func (a *App) writeLater(path string, data []byte, perm os.FileMode, what string) {
	w := &a.later
	w.mu.Lock()
	if w.data == nil {
		w.data, w.perm, w.what = map[string][]byte{}, map[string]os.FileMode{}, map[string]string{}
		w.idle = sync.NewCond(&w.mu)
	}
	if _, ok := w.data[path]; !ok {
		w.order = append(w.order, path)
	}
	w.data[path], w.perm[path], w.what[path] = data, perm, what
	start := !w.running
	w.running = true
	w.mu.Unlock()
	if start {
		a.goSafe("writeLater", a.drainWrites)
	}
}

// drainWrites grava a fila até ela esvaziar. running só volta a false no
// mesmo trecho travado em que a fila foi vista vazia: assim um writeLater
// que chega no fim ou acha a tarefa rodando (e ela pega o arquivo) ou
// começa outra.
func (a *App) drainWrites() {
	w := &a.later
	done := false
	defer func() { // se der panic: não deixa o flushWrites esperando
		if !done {
			w.mu.Lock()
			w.running = false
			w.idle.Broadcast()
			w.mu.Unlock()
		}
	}()
	for {
		w.mu.Lock()
		if len(w.order) == 0 {
			w.running = false
			w.idle.Broadcast()
			w.mu.Unlock()
			done = true
			return
		}
		path := w.order[0]
		w.order = w.order[1:]
		data, perm, what := w.data[path], w.perm[path], w.what[path]
		delete(w.data, path)
		delete(w.perm, path)
		delete(w.what, path)
		w.mu.Unlock()
		if err := writeFileAtomic(path, data, perm); err != nil {
			a.logf("%s: não consegui gravar o %s: %v", what, filepath.Base(path), err)
		}
	}
}

// flushWrites espera as gravações pendentes do writeLater. Se sobrou algo na
// fila sem tarefa rodando (uma tarefa que morreu em panic), grava aqui mesmo.
func (a *App) flushWrites() {
	w := &a.later
	w.mu.Lock()
	for w.running {
		w.idle.Wait()
	}
	left := len(w.order) > 0
	if left {
		w.running = true
	}
	w.mu.Unlock()
	if left {
		a.drainWrites()
	}
}
