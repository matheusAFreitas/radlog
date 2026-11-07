package app

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"time"

	"radlog/internal/gmc"
)

// ---------- log técnico (log.txt) ----------
//
// Tudo o que ajuda a entender um travamento: conexões, cada erro da porta
// com a mensagem exata, quanto cada comando demorou quando passou de 1 s,
// reconexões, PC parado (suspensão), downloads. Fica em log.txt na pasta de
// dados; passando de 2 MB vira log-anterior.txt.

const logMax = 2 << 20

var logMu sync.Mutex

func (a *App) logPath() string { return filepath.Join(a.dataDir, "log.txt") }

// logf escreve uma linha no log.txt. Não usa a.mu (pode ser chamado de
// qualquer lugar sem risco de travar).
func (a *App) logf(format string, args ...any) {
	if a.dataDir == "" {
		return
	}
	msg := fmt.Sprintf(format, args...)
	line := time.Now().Format("2006-01-02 15:04:05.000") + "  " + fmt.Sprintf("%-5s", classifyLog(msg)) + "  " + msg + "\n"
	logMu.Lock()
	defer logMu.Unlock()
	p := a.logPath()
	if st, err := os.Stat(p); err == nil && st.Size() > logMax {
		_ = os.Rename(p, filepath.Join(a.dataDir, "log-anterior.txt"))
	}
	f, err := os.OpenFile(p, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o644)
	if err != nil {
		return
	}
	_, _ = f.WriteString(line)
	f.Close()
}

func (a *App) logStart() {
	host, _ := os.Hostname()
	a.logf("==== RadLog %s iniciado (%s/%s, %s, PC %s)", AppVersion, runtime.GOOS, runtime.GOARCH, runtime.Version(), host)
}

// logPorts registra as portas seriais que o Windows está vendo agora.
func (a *App) logPorts(why string) {
	ports, err := gmc.ListPorts()
	if err != nil {
		a.logf("portas (%s): erro ao listar: %v", why, err)
		return
	}
	s := ""
	for i, p := range ports {
		if i > 0 {
			s += ", "
		}
		s += p.Name
		if p.Description != "" {
			s += " (" + p.Description + ")"
		}
	}
	if s == "" {
		s = "nenhuma"
	}
	a.logf("portas (%s): %s", why, s)
}

// withTimeout roda f com prazo. Se f travar, devolve errStuck (f continua
// presa até a porta ser fechada).
func withTimeout[T any](d time.Duration, f func() (T, error)) (T, error) {
	type res struct {
		v   T
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
		var zero T
		return zero, errStuck
	}
}

// OpenLogFile abre o log.txt no programa padrão do Windows.
func (a *App) OpenLogFile() error {
	p := a.logPath()
	if _, err := os.Stat(p); err != nil {
		a.logf("log aberto pela tela")
	}
	return openFolder(p)
}

// serialErrHint explica, na primeira falha, o que o erro do Windows quer
// dizer na prática (as mensagens do sistema enganam).
func serialErrHint(err error) string {
	m := strings.ToLower(err.Error())
	switch {
	case strings.Contains(m, "access is denied"):
		return " (o Windows nega acesso à porta quando o adaptador USB some: cabo tirado, troca de porta ou USB caiu)"
	case strings.Contains(m, "not functioning"):
		return " (o driver USB do contador parou de responder; o Windows ainda vê a porta)"
	case strings.Contains(m, "semaphore"), strings.Contains(m, "timeout"):
		return " (o contador não respondeu a tempo)"
	}
	return ""
}

// portListed diz se a porta ainda aparece na lista do sistema.
func portListed(port string) bool {
	ports, err := gmc.ListPorts()
	if err != nil {
		return true // não dá para saber: não afirma que sumiu
	}
	for _, p := range ports {
		if strings.EqualFold(p.Name, port) {
			return true
		}
	}
	return false
}
