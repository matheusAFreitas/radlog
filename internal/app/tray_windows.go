//go:build windows

package app

import (
	_ "embed"
	"fmt"
	"os"
	"runtime"
	"strings"

	"fyne.io/systray"
	"golang.org/x/sys/windows/registry"
)

// ---------- ícone na bandeja (perto do relógio) ----------

var (
	//go:embed icons/icon.ico
	trayIcon []byte
	//go:embed icons/icon-atencao.ico
	trayIconAttn []byte
	//go:embed icons/icon-alarme.ico
	trayIconAlarm []byte
	//go:embed icons/icon-perigo.ico
	trayIconDanger []byte
)

const trayAvailable = true

// startTray cria o ícone. O systray do Windows precisa da sua própria fila
// de mensagens, numa thread fixa.
func (a *App) startTray() {
	go func() {
		runtime.LockOSThread()
		systray.Run(func() {
			systray.SetIcon(trayIcon)
			systray.SetTooltip(appTitle)
			open := systray.AddMenuItem(a.T("Abrir o GMC Monitor"), a.T("Mostra a janela"))
			systray.AddSeparator()
			quit := systray.AddMenuItem(a.T("Sair"), a.T("Fecha o app (para de monitorar)"))
			a.trayMu.Lock()
			a.trayItems = []trayItem{{open, "Abrir o GMC Monitor", "Mostra a janela"}, {quit, "Sair", "Fecha o app (para de monitorar)"}}
			a.trayMu.Unlock()
			a.trayReady.Store(true)
			for {
				select {
				case <-open.ClickedCh:
					a.showFromTray()
				case <-quit.ClickedCh:
					a.quitApp()
					return
				}
			}
		}, nil)
	}()
}

func (a *App) stopTray() {
	if a.trayReady.Load() {
		systray.Quit()
	}
}

// trayShow atualiza a dica e o ícone (só quando mudam).
func (a *App) trayShow(tip, level string) {
	if !a.trayReady.Load() {
		return
	}
	a.trayMu.Lock()
	defer a.trayMu.Unlock()
	tip = a.T(tip)
	if tip != a.trayTip {
		a.trayTip = tip
		systray.SetTooltip(tip)
	}
	if level != a.trayLevel {
		a.trayLevel = level
		switch level {
		case "perigo":
			systray.SetIcon(trayIconDanger)
		case "alarme":
			systray.SetIcon(trayIconAlarm)
		case "atencao":
			systray.SetIcon(trayIconAttn)
		default:
			systray.SetIcon(trayIcon)
		}
	}
}

// ---------- iniciar com o Windows ----------

const runKey = `Software\Microsoft\Windows\CurrentVersion\Run`
const runName = "GMC Monitor"

// O Gerenciador de Tarefas (Aplicativos de inicialização) desativa um item
// sem apagar a linha do Run: marca aqui, com o 1º byte ímpar (0x03).
const approvedKey = `Software\Microsoft\Windows\CurrentVersion\Explorer\StartupApproved\Run`

// startupDisabled diz se o Windows está segurando o início do app.
func startupDisabled() bool {
	k, err := registry.OpenKey(registry.CURRENT_USER, approvedKey, registry.QUERY_VALUE)
	if err != nil {
		return false
	}
	defer k.Close()
	b, _, err := k.GetBinaryValue(runName)
	return err == nil && len(b) > 0 && b[0]&1 == 1
}

// enableStartup tira a marca de "desativado" (como ativar no Gerenciador).
func enableStartup() error {
	k, _, err := registry.CreateKey(registry.CURRENT_USER, approvedKey, registry.SET_VALUE|registry.QUERY_VALUE)
	if err != nil {
		return err
	}
	defer k.Close()
	return k.SetBinaryValue(runName, []byte{2, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0})
}

// setAutostart liga ou desliga o início com o Windows. Grava o caminho do
// .exe atual: abrindo uma versão nova, ela passa a ser a que inicia.
func setAutostart(on bool) error {
	k, err := registry.OpenKey(registry.CURRENT_USER, runKey, registry.SET_VALUE|registry.QUERY_VALUE)
	if err != nil {
		return err
	}
	defer k.Close()
	if !on {
		if err := k.DeleteValue(runName); err != nil && err != registry.ErrNotExist {
			return err
		}
		return nil
	}
	exe, err := os.Executable()
	if err != nil {
		return err
	}
	return k.SetStringValue(runName, fmt.Sprintf(`"%s" %s`, exe, trayArg))
}

func autostartPath() string {
	k, err := registry.OpenKey(registry.CURRENT_USER, runKey, registry.QUERY_VALUE)
	if err != nil {
		return ""
	}
	defer k.Close()
	v, _, err := k.GetStringValue(runName)
	if err != nil {
		return ""
	}
	return strings.TrimSpace(v)
}

type trayItem struct {
	it         *systray.MenuItem
	title, tip string
}

// trayRelabel troca os textos do menu quando o idioma muda.
func (a *App) trayRelabel() {
	if !a.trayReady.Load() {
		return
	}
	a.trayMu.Lock()
	defer a.trayMu.Unlock()
	for _, t := range a.trayItems {
		t.it.SetTitle(a.T(t.title))
		t.it.SetTooltip(a.T(t.tip))
	}
	a.trayTip = "" // a próxima dica sai no idioma novo
}
