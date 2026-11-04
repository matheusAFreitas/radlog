package main

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"time"
)

// ---------- leitura travada e fechamento do app ----------
//
// No Windows, a leitura ou a escrita na porta serial pode travar de vez no
// driver (USB suspenso, PC voltando da suspensão, cabo mexido): a chamada
// nunca volta, o app para de atualizar e, ao fechar, ficava esperando por
// ela. O processo continuava aberto e o bloqueio de "uma cópia só" impedia
// de abrir de novo. Agora cada comando tem prazo, o fechamento tem prazo, e
// quando algo trava o app grava um diagnóstico na pasta de dados.

const pollTimeout = 6 * time.Second

var errStuck = errors.New("o contador parou de responder (a porta travou)")

// callTimeout roda f com prazo. Se f travar, devolve errStuck (f continua
// presa até a porta ser fechada).
func callTimeout(f func() (uint32, error), d time.Duration) (uint32, error) {
	type res struct {
		v   uint32
		err error
	}
	ch := make(chan res, 1)
	go func() {
		v, err := f()
		ch <- res{v, err}
	}()
	select {
	case r := <-ch:
		return r.v, r.err
	case <-time.After(d):
		return 0, errStuck
	}
}

// dumpDiag grava o estado de todas as goroutines em diagnostico.txt (o mais
// recente substitui o anterior), para dar para ver onde travou.
func (a *App) dumpDiag(why string) {
	if a.dataDir == "" {
		return
	}
	buf := make([]byte, 1<<20)
	n := runtime.Stack(buf, true)
	txt := fmt.Sprintf("GMC Monitor %s\n%s\nMotivo: %s\n\n%s", AppVersion, time.Now().Format(time.RFC3339), why, buf[:n])
	_ = os.WriteFile(filepath.Join(a.dataDir, "diagnostico.txt"), []byte(txt), 0o644)
}

// beforeClose garante que o processo termina: se o fechamento normal não
// acabar em 5 s, grava o diagnóstico e sai assim mesmo.
func (a *App) beforeClose() {
	if !a.closing.CompareAndSwap(false, true) {
		return
	}
	time.AfterFunc(5*time.Second, func() {
		a.dumpDiag("o app demorou mais de 5 s para fechar")
		os.Exit(0)
	})
}
