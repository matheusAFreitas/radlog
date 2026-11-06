package app

import (
	"os"
	"path/filepath"
	"strings"
)

// ---------- abrir uma versão nova com a antiga na bandeja ----------
//
// Só pode haver uma cópia aberta (a porta serial é uma só). Quando você abre
// o .exe de uma versão nova com a antiga escondida na bandeja, o Windows
// manda o pedido para a antiga, que antes só mostrava a janela dela.
//
// Agora cada cópia se identifica (versão e caminho do .exe) nos argumentos
// que a Wails repassa. Se quem pediu é outro .exe ou outra versão, a antiga
// abre a nova (que espera ela terminar de fechar) e sai.

const (
	instArg     = "--instancia="  // versão|caminho do .exe de quem abriu
	updatingArg = "--atualizando" // aberta pela cópia antiga: espera ela sair
)

func selfExe() string {
	exe, err := os.Executable()
	if err != nil {
		return ""
	}
	if r, err := filepath.EvalSymlinks(exe); err == nil {
		exe = r
	}
	return exe
}

// tagArgs acrescenta a identificação desta cópia aos argumentos.
func tagArgs(args []string) []string {
	return append(append([]string(nil), args...), instArg+AppVersion+"|"+selfExe())
}

// splitInstance separa a identificação (e o --atualizando) do resto.
func splitInstance(args []string) (version, exe string, rest []string) {
	for _, a := range args {
		switch {
		case strings.HasPrefix(a, instArg):
			v := strings.TrimPrefix(a, instArg)
			if i := strings.Index(v, "|"); i >= 0 {
				version, exe = v[:i], v[i+1:]
			}
		case a == updatingArg, strings.HasPrefix(a, prevArg):
		default:
			rest = append(rest, a)
		}
	}
	return
}

func sameExe(a, b string) bool {
	return strings.EqualFold(filepath.Clean(a), filepath.Clean(b))
}

// shouldHandoff diz se o pedido veio de outra versão ou de outro .exe.
func shouldHandoff(args []string) (exe string, rest []string, ok bool) {
	v, exe, rest := splitInstance(args)
	if exe == "" {
		return "", nil, false // versão antiga, sem identificação: só mostra
	}
	if v == AppVersion && sameExe(exe, selfExe()) {
		return "", nil, false // a mesma cópia aberta de novo
	}
	if _, err := os.Stat(exe); err != nil {
		return "", nil, false
	}
	return exe, rest, true
}

func startedUpdating() bool {
	for _, a := range os.Args[1:] {
		if a == updatingArg {
			return true
		}
	}
	return false
}

// secondInstance trata o pedido de outra cópia.
func (a *App) secondInstance(args []string) {
	if a.closing.Load() {
		// esta cópia está fechando (ou travou fechando): sai logo, para a
		// próxima tentativa de abrir funcionar
		os.Exit(0)
	}
	if exe, rest, ok := shouldHandoff(args); ok {
		v, _, _ := splitInstance(args)
		a.logf("outra versão aberta (%s em %s): abrindo ela e fechando esta (%s)", v, exe, AppVersion)
		if err := startDetached(exe, append(rest, updatingArg)); err != nil {
			a.logf("não consegui abrir a versão nova: %v", err)
			a.showFromTray()
			return
		}
		a.logEvent("Abrindo a versão " + v)
		a.quitApp()
		return
	}
	a.showFromTray()
}
