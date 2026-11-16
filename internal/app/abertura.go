package app

import (
	"math"
	"sync"
	"time"
)

// Tempo de abertura (2.83.10, auditoria 2.79.0 P3/P4/A6): a tela avisa quando
// terminou de montar, e o log guarda quanto levou. Serve para comparar antes
// e depois de mexer no carregamento da tela, no PC e no celular.

// procStart é a hora em que o programa começou (antes da janela abrir).
var procStart = time.Now()

var uiReadyOnce sync.Once

// UIReady recebe da tela, em milissegundos desde que a página começou a
// carregar, quando os scripts terminaram de ler (scripts) e quando a primeira
// tela ficou pronta (ready). Só a primeira chamada de cada abertura conta:
// recarregar a tela (trocar o idioma, entrar na demonstração) não repete.
func (a *App) UIReady(scripts, ready float64) {
	uiReadyOnce.Do(func() {
		ms := func(v float64) int64 {
			if math.IsNaN(v) || math.IsInf(v, 0) || v < 0 || v > 3600e3 {
				return -1
			}
			return int64(math.Round(v))
		}
		a.logf("abertura: tela pronta em %d ms (scripts lidos em %d ms); %d ms desde que o programa começou",
			ms(ready), ms(scripts), time.Since(procStart).Milliseconds())
	})
}
