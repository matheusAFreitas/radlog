package app

import (
	"fmt"
	"time"

	wruntime "github.com/wailsapp/wails/v2/pkg/runtime"
)

const appTitle = "RadLog"

// alarmStarted chama atenção para a janela: título com o alarme e, se a
// opção estiver ligada, traz a janela para a frente (mesmo minimizada).
func (a *App) alarmStarted(cpm float64, demo bool) {
	a.mu.Lock()
	raise := a.settings.AlarmRaise
	a.mu.Unlock()
	a.flagWindow("ALARME", cpm, demo, raise)
}

// dangerStarted é o mesmo para o nível de perigo, que sempre traz a janela
// para a frente (não depende da opção).
func (a *App) dangerStarted(cpm float64, demo bool) {
	a.flagWindow("PERIGO", cpm, demo, true)
}

func (a *App) flagWindow(tag string, cpm float64, demo, raise bool) {
	a.mu.Lock()
	usv := a.convCalLocked().USv(cpm)
	a.mu.Unlock()
	a.flagWindowText(tag, fmtUSv(usv)+" µSv/h", demo, raise)
}

// flagWindowText põe o alerta no título (com o valor já escrito) e, se
// raise, traz a janela para a frente.
func (a *App) flagWindowText(tag, value string, demo, raise bool) {
	if a.ctx == nil {
		return
	}
	if demo {
		tag += " (demo)"
	}
	// 2.101.6: na unidade da tela, como a bandeja
	v := a.T(value)
	a.mu.Lock()
	mrem := a.settings.RateUnit == rateMrem
	a.mu.Unlock()
	if mrem {
		sep := ","
		if a.uiLang() == "en" {
			sep = "."
		}
		v = remText(v, sep)
	}
	wruntime.WindowSetTitle(a.ctx, fmt.Sprintf("%s: %s — %s", a.T(tag), v, appTitle))
	// escondida na bandeja: sempre volta (escondida, o som pode não tocar)
	if a.inTray.Load() {
		raise = true
		a.inTray.Store(false)
	}
	if raise {
		// Só restaura se estiver minimizada: em janela maximizada o "restaurar"
		// do Windows tira do maximizado e a janela muda de tamanho.
		if wruntime.WindowIsMinimised(a.ctx) {
			wruntime.WindowUnminimise(a.ctx)
		}
		wruntime.WindowShow(a.ctx)
		wruntime.WindowSetAlwaysOnTop(a.ctx, true)
		ctx := a.ctx
		time.AfterFunc(2*time.Second, func() { wruntime.WindowSetAlwaysOnTop(ctx, false) })
	}
}

// alarmEnded volta o título ao normal quando nenhum alarme está ativo.
func (a *App) alarmEnded() {
	if a.ctx == nil {
		return
	}
	a.mu.Lock()
	any := a.alarmOn || a.dangerOn || a.rcAlert.alarmOn || a.rcAlert.dangOn
	a.mu.Unlock()
	if !any {
		wruntime.WindowSetTitle(a.ctx, appTitle)
	}
}
