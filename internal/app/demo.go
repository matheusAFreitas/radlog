package app

import (
	"errors"
	"math"
	"time"

	"radlog/internal/store"
)

// ---------- buffer das leituras por segundo (gráficos de 15 e 30 min) ----------

type sample struct {
	t   time.Time
	cpm float64
}

// ring guarda a última hora de leituras por segundo.
type ring struct {
	buf  []sample
	next int
	full bool
}

const ringSize = 3600

func newRing() *ring { return &ring{buf: make([]sample, ringSize)} }

func (r *ring) add(t time.Time, cpm float64) {
	r.buf[r.next] = sample{t, cpm}
	r.next = (r.next + 1) % ringSize
	if r.next == 0 {
		r.full = true
	}
}

// between devolve as amostras em [from, to), em ordem.
func (r *ring) between(from, to time.Time) []sample {
	var out []sample
	n := r.next
	start := 0
	if r.full {
		n = ringSize
		start = r.next
	}
	for k := 0; k < n; k++ {
		s := r.buf[(start+k)%ringSize]
		if !s.t.Before(from) && s.t.Before(to) {
			out = append(out, s)
		}
	}
	return out
}

func (r *ring) reset() { *r = *newRing() }

// ---------- modo demonstração (2.82) ----------
//
// Com a demonstração ligada, a tela inteira mostra um mundo simulado: o
// mesmo do demo.js (o modo navegador), com os cenários tocados lá. O Go só
// guarda o estado (para a tela voltar igual depois de recarregar), mostra o
// valor simulado no ícone da bandeja e recusa o que tiraria dado de mentira
// do app (CSV, PDF, backup, nuvem, sincronização). Os aparelhos de verdade
// continuam sendo lidos e gravados por baixo, e um alarme de verdade tira da
// demonstração.

// DemoState é o estado da demonstração.
type DemoState struct {
	Active   bool    `json:"active"`
	Scenario string  `json:"scenario"` // casa, atencao, alarme, perigo, fonte, picorc, saturado, saida, cs137, desconecta, valor
	USv      float64 `json:"usv"`      // "Valor agora" (cenário "valor")
	Speed    int     `json:"speed"`    // 1 ou 10
	Sound    bool    `json:"sound"`    // som do alarme (mudo por padrão)
	Since    int64   `json:"since"`    // quando ligou (ms)
}

var demoScenarios = map[string]bool{"casa": true, "atencao": true, "alarme": true, "perigo": true, "fonte": true,
	"picorc": true, "saturado": true, "saida": true, "cs137": true, "desconecta": true, "valor": true}

// errDemo é a recusa do que não vale durante a demonstração.
var errDemo = errors.New("saia da demonstração para usar")

// GetDemo devolve o estado da demonstração.
func (a *App) GetDemo() DemoState {
	a.mu.Lock()
	defer a.mu.Unlock()
	return a.demo
}

func clampUSv(v float64) float64 {
	if math.IsNaN(v) || v < 0 {
		return 0
	}
	if v > 10000 {
		return 10000
	}
	return v
}

// SetDemo liga a demonstração ou muda o cenário, o valor, a velocidade e o
// som. Nada disso vai para o disco.
func (a *App) SetDemo(st DemoState) DemoState {
	if !demoScenarios[st.Scenario] {
		st.Scenario = "casa"
	}
	if st.Speed != 10 {
		st.Speed = 1
	}
	st.USv = clampUSv(st.USv)
	st.Active = true
	a.mu.Lock()
	was := a.demo.Active
	st.Since = a.demo.Since
	if !was {
		st.Since = time.Now().UnixMilli()
	}
	a.demo = st
	a.mu.Unlock()
	if !was {
		a.logf("demonstração ligada")
	}
	a.emit("demo", st)
	return st
}

// StopDemo desliga a demonstração (a tela recarrega com os dados reais).
func (a *App) StopDemo() DemoState {
	a.mu.Lock()
	was := a.demo.Active
	a.demo = DemoState{}
	a.mu.Unlock()
	if !was {
		return DemoState{}
	}
	a.logf("demonstração desligada")
	a.trayMu.Lock()
	a.trayRcDemo = nil
	a.trayMu.Unlock()
	a.trayGmcConn("") // a próxima leitura real atualiza o ícone
	a.emit("demo", DemoState{})
	return DemoState{}
}

// DemoTray põe no ícone da bandeja o que a demonstração mostra agora
// (rcState vazio: RC102 desligado na demonstração).
func (a *App) DemoTray(gUSv float64, gLevel string, rUSv float64, rLevel, rcState string) {
	if !a.isDemo() {
		return
	}
	a.trayMu.Lock()
	a.trayGmc = trayDev{Name: "GMC-800", Key: "gmc", State: "ok", USv: clampUSv(gUSv), Level: gLevel}
	a.trayDemo = true
	r := &trayDev{Name: "RC102", Key: "rc", State: "off"}
	if rcState == "conectado" {
		r.State, r.USv, r.Level = "ok", clampUSv(rUSv), rLevel
	}
	a.trayRcDemo = r
	a.trayMu.Unlock()
	a.trayRefresh()
}

// realAlarmStopsDemo: um alarme (ou perigo) de verdade tira da demonstração,
// para ninguém achar que é de mentira.
func (a *App) realAlarmStopsDemo() {
	if a.isDemo() {
		a.StopDemo()
		a.logEvent("Demonstração desligada: alarme de verdade")
	}
}

// points devolve as leituras por minuto em [from, to).
func (a *App) points(from, to time.Time) ([]store.Point, error) {
	return a.storeRange(from, to)
}

// liveSamples devolve as leituras por segundo da última hora.
func (a *App) liveSamples(from, to time.Time) []sample {
	a.mu.Lock()
	defer a.mu.Unlock()
	return a.live.between(from, to)
}
