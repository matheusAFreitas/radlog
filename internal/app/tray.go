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

// trayTick guarda a leitura do GMC-800 e atualiza o ícone e a dica.
func (a *App) trayTick(usv float64, level string, noCount, sat, demo bool) {
	a.trayMu.Lock()
	a.trayGmc = trayDev{Name: "GMC-800", Key: "gmc", State: "ok", USv: usv, Level: level, NoCount: noCount, Sat: sat}
	a.trayDemo = demo
	a.trayMu.Unlock()
	a.trayRefresh()
}

// trayGmcConn segue a conexão do GMC-800 (conectando, desconectado).
func (a *App) trayGmcConn(state string) {
	a.trayMu.Lock()
	switch state {
	case "conectado":
		a.trayMu.Unlock()
		return // a primeira leitura traz o valor
	case "": // saiu do demo: a próxima leitura real atualiza o valor
		a.trayDemo = false
		a.trayMu.Unlock()
		a.trayRefresh()
		return
	case "conectando":
		a.trayGmc = trayDev{Name: "GMC-800", Key: "gmc", State: "wait"}
	default:
		a.trayGmc = trayDev{Name: "GMC-800", Key: "gmc", State: "off"}
	}
	a.trayDemo = false
	a.trayMu.Unlock()
	a.trayRefresh()
}

// trayRc lê o RC102 para o ícone e a dica.
func (a *App) trayRc() (d trayDev, show bool) {
	a.rc.mu.Lock()
	st := a.rc.st
	a.rc.mu.Unlock()
	a.demoRcOverlay(&st)
	now := time.Now()
	a.mu.Lock()
	rcOff := a.settings.RcOff && !st.Demo
	lvl := a.rcAlertLevelLocked()
	satAt := a.effLocked().Levels.RcSatUSv
	away := a.whereaboutsLocked(now).NotHome
	a.mu.Unlock()
	if st.Demo {
		lvl = st.AlertLevel
	}
	d = trayDev{Name: "RC102", Key: "rc", State: "off", SatAbove: satAt, Away: away}
	switch st.State {
	case "conectado":
		d.State = "ok"
		d.USv, d.Level = st.USvh, lvl
		d.Sat = st.Offscale || (satAt > 0 && st.USvh >= satAt)
		if d.Sat {
			d.Level = "perigo"
		}
	case "conectando":
		d.State = "wait"
	}
	return d, !rcOff
}

// trayRefresh monta o ícone (número do aparelho escolhido) e a dica (os
// dois aparelhos, uma linha cada).
func (a *App) trayRefresh() {
	if !trayAvailable || !a.trayReady.Load() {
		return
	}
	r, showRc := a.trayRc()
	a.mu.Lock()
	plain, choice := a.settings.TrayPlain, a.settings.TrayDevice
	a.mu.Unlock()
	sep := ","
	if a.uiLang() == "en" {
		sep = "."
	}
	a.trayMu.Lock()
	g, demo := a.trayGmc, a.trayDemo
	a.trayMu.Unlock()
	if g.Name == "" {
		g = trayDev{Name: "GMC-800", Key: "gmc", State: "off"}
	}
	if !showRc {
		r.State = "off"
		if choice == "radiacode" {
			choice = ""
		}
	}
	me := trayPick(choice, g, r)
	other := r
	if me.Key == "rc" {
		other = g
	}
	lines := []string{a.T(trayLine(me, sep))}
	if showRc || other.Key == "gmc" {
		lines = append(lines, a.T(trayLine(other, sep)))
	}
	lines = append(lines, "RadLog")
	tip := strings.Join(lines, "\n")
	if demo {
		tip = "DEMO · " + tip
	}
	if n := []rune(tip); len(n) > 127 { // limite do Windows
		tip = string(n[:127])
	}
	txt, lvl := trayText(me, sep)
	if me.State != "ok" {
		lvl = ""
	}
	key := fmt.Sprintf("%s|%s|%s|%s", me.Key, me.State, txt, lvl)
	if plain {
		key = "plain|" + lvl
	}
	a.trayApply(tip, plain, lvl, key, func() []byte { return trayICO(me, sep) })
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
