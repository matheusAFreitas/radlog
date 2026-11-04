package main

import (
	"fmt"
	"time"

	wruntime "github.com/wailsapp/wails/v2/pkg/runtime"
)

const appTitle = "GMC Monitor"

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
	if a.ctx == nil {
		return
	}
	if demo {
		tag += " (demo)"
	}
	wruntime.WindowSetTitle(a.ctx, fmt.Sprintf("%s: %.0f CPM — %s", tag, cpm, appTitle))
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
	any := a.alarmOn || a.dangerOn || (a.demo.Active && (a.demo.alarmOn || a.demo.dangerOn))
	a.mu.Unlock()
	if !any {
		wruntime.WindowSetTitle(a.ctx, appTitle)
	}
}
