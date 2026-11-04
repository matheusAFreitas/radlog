package app

import (
	"fmt"
	"os"
	"strings"
	"time"

	wruntime "github.com/wailsapp/wails/v2/pkg/runtime"
)

// ---------- bandeja: comportamento comum ----------
//
// Com "continuar na bandeja" ligado, fechar a janela só a esconde: o app
// segue lendo o contador, gravando e alarmando. "Sair" (no menu do ícone)
// fecha de verdade. Alarme e perigo trazem a janela de volta sempre que ela
// estiver escondida (escondida, o som pode não tocar).

// trayArg é o argumento do início com o Windows: abre direto na bandeja.
const trayArg = "--bandeja"

func startedInTray() bool {
	for _, a := range os.Args[1:] {
		if a == trayArg {
			return true
		}
	}
	return false
}

// hideToTray esconde a janela (fechar com a bandeja ligada).
func (a *App) hideToTray() {
	a.inTray.Store(true)
	if a.ctx != nil {
		wruntime.WindowHide(a.ctx)
	}
}

func (a *App) showFromTray() {
	a.inTray.Store(false)
	if a.ctx == nil {
		return
	}
	wruntime.WindowShow(a.ctx)
	if wruntime.WindowIsMinimised(a.ctx) {
		wruntime.WindowUnminimise(a.ctx)
	}
	a.emit("shown", time.Now().UnixMilli())
}

// quitApp fecha de verdade (menu "Sair" da bandeja).
func (a *App) quitApp() {
	a.quitting.Store(true)
	if a.ctx != nil {
		wruntime.Quit(a.ctx)
	}
}

// onBeforeClose decide o que o X da janela faz.
func (a *App) onBeforeClose() (prevent bool) {
	a.mu.Lock()
	keep := a.settings.TrayOnClose
	a.mu.Unlock()
	if keep && trayAvailable && a.trayReady.Load() && !a.quitting.Load() {
		a.hideToTray()
		return true
	}
	a.beforeClose()
	return false
}

// trayTick atualiza a dica do ícone a cada leitura.
func (a *App) trayTick(cpm uint32, usv float64, level string, noCount, demo bool) {
	tip := fmt.Sprintf("GMC Monitor · %d CPM · %.3f µSv/h", cpm, usv)
	switch {
	case level == "perigo":
		tip = fmt.Sprintf("PERIGO · %d CPM · GMC Monitor", cpm)
	case level == "alarme":
		tip = fmt.Sprintf("ALARME · %d CPM · GMC Monitor", cpm)
	case level == "atencao":
		tip = fmt.Sprintf("Atenção · %d CPM · GMC Monitor", cpm)
	case noCount:
		tip = "Sem contagens · confira o contador · GMC Monitor"
		level = "alarme"
	}
	if demo {
		tip = "DEMO · " + tip
	}
	a.trayShow(tip, level)
}

func (a *App) isDemo() bool {
	a.mu.Lock()
	defer a.mu.Unlock()
	return a.demo.Active
}

// syncAutostart aplica a configuração "iniciar com o Windows".
func (a *App) syncAutostart(on bool) {
	if err := setAutostart(on); err != nil {
		a.logEvent("Não consegui mudar o início com o Windows: " + err.Error())
	}
}

// AutostartStatus é o que está registrado para abrir com o Windows.
type AutostartStatus struct {
	Available bool   `json:"available"` // só no Windows
	On        bool   `json:"on"`        // opção ligada no app
	Path      string `json:"path"`      // .exe registrado ("" = nada)
	Exists    bool   `json:"exists"`    // o .exe registrado existe
	Current   bool   `json:"current"`   // é este .exe
	Disabled  bool   `json:"disabled"`  // desativado no Gerenciador de Tarefas
	Started   bool   `json:"started"`   // esta cópia abriu com o Windows
}

func runExe(cmdline string) string {
	s := strings.TrimSpace(cmdline)
	if strings.HasPrefix(s, `"`) {
		if i := strings.Index(s[1:], `"`); i >= 0 {
			return s[1 : i+1]
		}
	}
	if i := strings.Index(s, " --"); i >= 0 {
		return s[:i]
	}
	return s
}

// GetAutostartStatus devolve o estado do início com o Windows.
func (a *App) GetAutostartStatus() AutostartStatus {
	a.mu.Lock()
	on := a.settings.StartWithWindows
	a.mu.Unlock()
	st := AutostartStatus{Available: trayAvailable, On: on, Started: startedInTray()}
	if !trayAvailable {
		return st
	}
	st.Path = runExe(autostartPath())
	if st.Path != "" {
		_, err := os.Stat(st.Path)
		st.Exists = err == nil
		st.Current = sameExe(st.Path, selfExe())
	}
	st.Disabled = startupDisabled()
	return st
}

// FixAutostart registra este .exe e tira a trava do Gerenciador de Tarefas
// (botão nas Configurações).
func (a *App) FixAutostart() (AutostartStatus, error) {
	a.mu.Lock()
	a.settings.StartWithWindows = true
	a.saveSettingsLocked()
	a.mu.Unlock()
	if err := setAutostart(true); err != nil {
		return a.GetAutostartStatus(), err
	}
	if startupDisabled() {
		if err := enableStartup(); err != nil {
			return a.GetAutostartStatus(), err
		}
	}
	a.logAutostart()
	return a.GetAutostartStatus(), nil
}

// logAutostart anota no log técnico como está o início com o Windows.
func (a *App) logAutostart() {
	st := a.GetAutostartStatus()
	if !st.Available {
		return
	}
	a.logf("início com o Windows: opção %v, registrado %q (existe %v, este .exe %v), desativado no Windows %v, abriu com o Windows %v",
		st.On, st.Path, st.Exists, st.Current, st.Disabled, st.Started)
}
