package app

import (
	"fmt"
	"math"
	"math/rand"
	"sort"
	"strings"
	"time"

	"radlog/internal/core/levels"
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

// ---------- modo demo ----------
//
// No modo demo a tela, o alarme e os gráficos usam um valor simulado. O
// contador continua sendo lido e as leituras REAIS continuam sendo gravadas
// nos arquivos; nada simulado vai para o disco. Ao sair do
// demo, tudo que foi simulado é descartado.

// DemoState é o estado do modo demo.
type DemoState struct {
	Active    bool    `json:"active"`
	CPM       float64 `json:"cpm"`
	Realistic bool    `json:"realistic"` // varia como contagem de verdade
	// RadiaCode simulado (2.14)
	Rc    bool    `json:"rc"`
	RcUSv float64 `json:"rcUSv"`
}

type demoData struct {
	DemoState
	stop     chan struct{}
	ring     *ring
	minutes  map[int64]float64 // minuto (unix) -> CPM simulado
	alarmOn  bool
	attnOn   bool
	dangerOn bool
	sat      satState
	alerts   alertIDs
	rc       *demoRc
}

func (a *App) demoActive() bool {
	a.mu.Lock()
	defer a.mu.Unlock()
	return a.demo.Active
}

// GetDemo devolve o estado do modo demo.
func (a *App) GetDemo() DemoState {
	a.mu.Lock()
	defer a.mu.Unlock()
	return a.demo.DemoState
}

func clampCPM(v float64) float64 {
	if math.IsNaN(v) || v < 0 {
		return 0
	}
	if v > 999999 {
		return 999999
	}
	return v
}

// StartDemo liga o modo demo (ou só muda o valor, se já estiver ligado).
func (a *App) StartDemo(cpm float64, realistic bool) DemoState {
	cpm = clampCPM(cpm)
	a.mu.Lock()
	if a.demo.Active {
		a.demo.CPM, a.demo.Realistic = cpm, realistic
		st := a.demo.DemoState
		a.mu.Unlock()
		a.emit("demo", st)
		return st
	}
	stop := make(chan struct{})
	a.measure = MeasureState{}
	a.demo = demoData{
		DemoState: DemoState{Active: true, CPM: cpm, Realistic: realistic},
		stop:      stop, ring: newRing(), minutes: map[int64]float64{},
	}
	st := a.demo.DemoState
	a.mu.Unlock()

	a.logDemoEvent(fmt.Sprintf("Modo demo ligado (%.0f CPM)", cpm), kindEvent)
	a.emit("measure", MeasureState{})
	a.emit("demo", st)
	go a.demoLoop(stop)
	return st
}

// StopDemo desliga o modo demo e descarta tudo que foi simulado.
func (a *App) StopDemo() DemoState {
	a.mu.Lock()
	if !a.demo.Active {
		a.mu.Unlock()
		return DemoState{}
	}
	close(a.demo.stop)
	rcd := a.demo.rc
	a.demo = demoData{}
	a.measure = MeasureState{}
	// tira os eventos do demo da lista
	kept := a.events[:0]
	for _, e := range a.events {
		if !strings.HasPrefix(e.Text, demoPrefix) {
			kept = append(kept, e)
		}
	}
	a.events = kept
	a.mu.Unlock()
	if rcd != nil {
		a.finishDemoRcAlerts(rcd)
		a.mu.Lock()
		kept := a.events[:0]
		for _, e := range a.events {
			if !strings.HasPrefix(e.Text, demoPrefix) {
				kept = append(kept, e)
			}
		}
		a.events = kept
		a.mu.Unlock()
	}

	a.alarmEnded()
	a.emit("measure", MeasureState{})
	a.emit("demo", DemoState{})
	a.trayShow(appTitle, "")                 // a próxima leitura real atualiza o ícone
	a.emit("stored", time.Now().UnixMilli()) // a tela recarrega com os dados reais
	a.emit("event", Event{Kind: kindAlert})
	a.emit("event", Event{Kind: kindEvent})
	a.emitActive()
	a.emitRc()
	return DemoState{}
}

const demoPrefix = "[demo] "

// logDemoEvent registra um evento ou alerta só na memória (não vai para
// eventos.csv).
func (a *App) logDemoEvent(text, kind string) {
	a.logEntry(demoPrefix+text, kind, false)
}

// poisson sorteia uma contagem com média lambda.
func poisson(rng *rand.Rand, lambda float64) int {
	if lambda <= 0 {
		return 0
	}
	if lambda > 50 { // aproximação normal, rápida para taxas altas
		v := int(math.Round(rng.NormFloat64()*math.Sqrt(lambda) + lambda))
		if v < 0 {
			return 0
		}
		return v
	}
	l, k, p := math.Exp(-lambda), 0, 1.0
	for {
		p *= rng.Float64()
		if p <= l {
			return k
		}
		k++
	}
}

func (a *App) demoLoop(stop chan struct{}) {
	rng := rand.New(rand.NewSource(time.Now().UnixNano()))
	ticker := time.NewTicker(time.Second)
	defer ticker.Stop()

	// janela de 60 s de contagens, como o CPM do contador
	var window [60]int
	wi := 0
	filled := false
	var minute time.Time
	var lastCPM float64
	var lastAt time.Time

	for {
		select {
		case <-stop:
			return
		case <-ticker.C:
		}
		a.demoRcTick(rng, time.Now())
		a.mu.Lock()
		target, realistic := a.demo.CPM, a.demo.Realistic
		cal := a.convCalLocked()
		rawCal := cal
		set := a.effLocked()
		attnU, alarmU, dangerU := thrUSvLocked(a, set)
		a.mu.Unlock()

		var cpm float64
		var cps uint32
		if realistic {
			if !filled { // começa já "aquecido" no valor escolhido
				for i := range window {
					window[i] = poisson(rng, target/60)
				}
				filled = true
			}
			c := poisson(rng, target/60)
			window[wi] = c
			wi = (wi + 1) % len(window)
			sum := 0
			for _, v := range window {
				sum += v
			}
			cpm, cps = float64(sum), uint32(c)
		} else {
			filled = false
			cpm, cps = target, uint32(math.Round(target/60))
		}

		now := time.Now()
		m := now.Truncate(time.Minute)
		a.mu.Lock()
		if !a.demo.Active {
			a.mu.Unlock()
			return
		}
		if !minute.IsZero() && m.After(minute) && lastAt.Sub(minute) >= 50*time.Second {
			a.demo.minutes[minute.Unix()] = lastCPM
		}
		a.demo.ring.add(now, cpm)
		alarm := set.AlarmEnabled && cpm >= float64(set.AlarmCPM)
		was := a.demo.alarmOn
		started := alarm && !a.demo.alarmOn
		if alarm {
			a.demo.alarmOn = true
		} else if cpm < float64(set.AlarmCPM)*0.9 || !set.AlarmEnabled {
			a.demo.alarmOn = false
		}
		ended := was && !a.demo.alarmOn
		attnWas := a.demo.attnOn
		lv := set.Levels.Norm()
		attnStarted := levels.AttentionStep(&a.demo.attnOn, set.AttentionEnabled, cpm, lv)
		attnOn := a.demo.attnOn
		satOn, satStarted := a.demo.sat.step(now, rawCal.USv(cpm), rawCal.USv(float64(cps)*60), lv)
		dCPM := cpm
		if satOn {
			dCPM = math.Max(dCPM, lv.DangerEnter())
		}
		dangerStarted, dangerEnded := levels.DangerStep(&a.demo.dangerOn, dCPM, lv)
		dangerOn := a.demo.dangerOn
		if dangerStarted {
			a.demo.sat.rise = a.demo.sat.riseFrom(now, cal.USv(lv.AttnEnter()))
		}
		if !dangerOn {
			a.demo.sat.rise = nil
		}
		satReason, rise := a.demo.sat.reason, a.demo.sat.rise
		alarmOn := a.demo.alarmOn && set.AlarmEnabled
		a.mu.Unlock()
		if satStarted {
			a.logDemoEvent(satText(satReason, lv), kindEvent)
		}
		newMinute := !minute.IsZero() && m.After(minute)
		minute, lastCPM, lastAt = m, cpm, now

		a.stepAlerts(&a.demo.alerts, alertStep{
			Now: now, CPM: cpm, AlarmCPM: set.AlarmCPM, Demo: true, Lv: lv,
			USv: cal.USv(cpm), AttnUSv: attnU, AlarmUSv: alarmU, DngrUSv: dangerU,
			AttnStarted: attnStarted, AttnEnded: attnWas && !attnOn, AttnOn: attnOn,
			AlarmStarted: started, AlarmEnded: ended, AlarmOn: alarmOn,
			DangerStarted: dangerStarted, DangerEnded: dangerEnded,
		})
		if dangerStarted {
			a.dangerStarted(cpm, true)
		} else if started {
			a.alarmStarted(cpm, true)
		}
		if dangerEnded && alarmOn {
			// saiu do perigo mas continua no alarme do usuário: só o título
			a.flagWindow("ALARME", cpm, true, false)
		}
		if ended || dangerEnded {
			a.alarmEnded()
		}
		if newMinute {
			a.emit("stored", now.UnixMilli())
		}
		usv := cal.USv(cpm)
		r := Reading{
			Time: now.UnixMilli(), CPM: uint32(math.Round(cpm)), CPS: cps,
			USv: usv, MRh: usv / 10, Level: levels.LevelWith(cpm, float64(set.AlarmCPM), lv), Alarm: alarmOn, Demo: true,
			Attention: attnOn, Danger: dangerOn,
			Saturated: satOn, SatReason: satReason, Rise: rise,
		}
		a.fillSigma(&r)
		a.feedMeasure(cpm, cps, true)
		trayLvl := ""
		switch {
		case dangerOn:
			trayLvl = "perigo"
		case alarmOn:
			trayLvl = "alarme"
		case attnOn:
			trayLvl = "atencao"
		}
		a.trayTick(r.CPM, usv, trayLvl, false, true)
		a.emit("reading", r)
	}
}

// points devolve as leituras por minuto em [from, to). Com o demo ligado, os
// minutos simulados aparecem por cima dos reais (só na tela).
func (a *App) points(from, to time.Time) ([]store.Point, error) {
	pts, err := a.store.Range(from, to)
	if err != nil {
		return nil, err
	}
	a.mu.Lock()
	var demo map[int64]float64
	if a.demo.Active && len(a.demo.minutes) > 0 {
		demo = make(map[int64]float64, len(a.demo.minutes))
		for k, v := range a.demo.minutes {
			demo[k] = v
		}
	}
	a.mu.Unlock()
	if demo == nil {
		return pts, nil
	}
	out := make([]store.Point, 0, len(pts)+len(demo))
	for _, p := range pts {
		if _, sim := demo[p.T.Unix()]; !sim {
			out = append(out, p)
		}
	}
	for k, v := range demo {
		t := time.Unix(k, 0)
		if !t.Before(from) && t.Before(to) {
			out = append(out, store.Point{T: t, CPM: v, Src: "demo"})
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].T.Before(out[j].T) })
	return out, nil
}

// liveSamples devolve as leituras por segundo (reais, ou simuladas no demo).
func (a *App) liveSamples(from, to time.Time) []sample {
	a.mu.Lock()
	defer a.mu.Unlock()
	if a.demo.Active {
		return a.demo.ring.between(from, to)
	}
	return a.live.between(from, to)
}
