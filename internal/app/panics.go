package app

import (
	"fmt"
	"runtime/debug"
	"sync"
	"time"
)

// ---------- erro interno (panic) nas tarefas de fundo ----------
//
// O app fica dias na bandeja com várias goroutines (leitura do GMC, laço do
// RadiaCode, sync, chuva, atualização...). Sem recover, um panic em qualquer
// uma delas (ex.: um pacote do Bluetooth cortado no meio que o decodificador
// não esperava) fecha o processo inteiro e a gravação para até alguém abrir o
// app de novo. Aqui o panic vira uma linha no log técnico (com a pilha), um
// evento na tela, e a tarefa volta sozinha:
//
//   - goSafe: tarefas que rodam uma vez (backfill, reconexão): só registra.
//   - goLoop: laços que deveriam rodar para sempre: registra e reinicia com
//     espera crescente (5 s, 10 s, 20 s... até 5 min). Se cair panicMax vezes
//     na mesma hora, desiste e avisa, para não encher o log.

const (
	panicWait0 = 5 * time.Second
	panicWaitN = 5 * time.Minute
	panicMax   = 10
)

// panicTrack conta os panics recentes de cada laço (para desistir).
type panicTrack struct {
	mu   sync.Mutex
	hits map[string][]time.Time
}

var panics = panicTrack{hits: map[string][]time.Time{}}

// note registra um panic de name e devolve quantos houve na última hora.
func (p *panicTrack) note(name string, now time.Time) int {
	p.mu.Lock()
	defer p.mu.Unlock()
	keep := p.hits[name][:0]
	for _, t := range p.hits[name] {
		if now.Sub(t) < time.Hour {
			keep = append(keep, t)
		}
	}
	keep = append(keep, now)
	p.hits[name] = keep
	return len(keep)
}

// panicBackoff é a espera antes de reiniciar depois do n-ésimo panic seguido.
func panicBackoff(n int) time.Duration {
	d := panicWait0
	for i := 1; i < n && d < panicWaitN; i++ {
		d *= 2
	}
	if d > panicWaitN {
		d = panicWaitN
	}
	return d
}

// recovered registra um panic já capturado: pilha no log, evento na tela.
// name é o nome técnico da tarefa (ex.: "rcLoop"); vai igual nos dois idiomas.
func (a *App) recovered(name string, r any) {
	a.logf("ERRO interno em %s: %v\n%s", name, r, debug.Stack())
	a.logEvent(fmt.Sprintf("Erro interno no app (%s); detalhes no log técnico", name))
}

// goSafe roda fn numa goroutine; um panic é registrado e não fecha o app.
func (a *App) goSafe(name string, fn func()) {
	go func() {
		defer func() {
			if r := recover(); r != nil {
				a.recovered(name, r)
			}
		}()
		fn()
	}()
}

// goLoop roda fn (um laço que não deveria terminar) numa goroutine e a
// reinicia depois de um panic. Se fn voltar normalmente, não reinicia.
func (a *App) goLoop(name string, fn func()) {
	go a.runLoop(name, fn, time.Sleep)
}

func (a *App) runLoop(name string, fn func(), sleep func(time.Duration)) {
	for {
		if !a.runOnce(name, fn) {
			return // terminou normalmente
		}
		if a.closing.Load() || a.quitting.Load() {
			return
		}
		n := panics.note(name, time.Now())
		if n >= panicMax {
			a.logf("ERRO interno em %s: %d vezes na última hora, não vou reiniciar", name, n)
			a.logEvent(fmt.Sprintf("Erro interno repetido no app (%s); a tarefa parou. Feche e abra o RadLog", name))
			return
		}
		d := panicBackoff(n)
		a.logf("%s: reiniciando em %s (erro interno nº %d na última hora)", name, d, n)
		sleep(d)
	}
}

// runOnce roda fn e diz se ela terminou por panic.
func (a *App) runOnce(name string, fn func()) (panicked bool) {
	defer func() {
		if r := recover(); r != nil {
			a.recovered(name, r)
			panicked = true
		}
	}()
	fn()
	return false
}
