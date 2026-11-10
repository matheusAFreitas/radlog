package app

import (
	"fmt"
	"math"
	"math/rand"
	"time"
)

// ---------- Radiacode no modo demo (2.14) ----------
//
// Com o demo ligado dá para simular também o Radiacode: a tela, os alertas
// (com os limites dele) e os gráficos usam um µSv/h escolhido. Como no GMC,
// nada simulado vai para o disco; o aparelho de verdade, se conectado,
// continua sendo lido e gravado.

// rcDemoCPSPerUSv é a sensibilidade usada para simular as contagens (RC-102:
// ~13 CPS num fundo de ~0,15 µSv/h).
const rcDemoCPSPerUSv = 85.0

type demoRc struct {
	ring    *ring             // por segundo (µSv/h no campo cpm)
	minutes map[int64]float64 // minuto (unix) -> µSv/h simulado
	alert   rcAlertState
	usv     float64 // valor mostrado agora
	cps     float64
	minAcc  float64
	minN    int
	minT    time.Time
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

// SetDemoRc liga ou desliga o Radiacode simulado e escolhe o valor (µSv/h).
// Só vale com o modo demo ligado.
func (a *App) SetDemoRc(on bool, usv float64) DemoState {
	usv = clampUSv(usv)
	a.mu.Lock()
	if !a.demo.Active {
		a.mu.Unlock()
		return DemoState{}
	}
	was := a.demo.Rc
	a.demo.Rc, a.demo.RcUSv = on, usv
	if on && a.demo.rc == nil {
		a.demo.rc = &demoRc{ring: newRing(), minutes: map[int64]float64{}}
	}
	var ending *demoRc
	if !on && a.demo.rc != nil {
		ending = a.demo.rc
		a.demo.rc = nil
	}
	st := a.demo.DemoState
	a.mu.Unlock()
	if ending != nil {
		a.finishDemoRcAlerts(ending)
	}
	if on && !was {
		a.logDemoEvent(fmt.Sprintf("RC102 simulado ligado (%s µSv/h)", usvTxt(usv)), kindEvent)
	}
	a.emit("demo", st)
	a.emitRc()
	a.emit("stored", time.Now().UnixMilli())
	return st
}

// finishDemoRcAlerts fecha os alertas simulados do Radiacode (saiu do demo
// ou desligou a simulação).
func (a *App) finishDemoRcAlerts(d *demoRc) {
	s := &d.alert
	if !s.attnOn && !s.alarmOn && !s.dangOn {
		return
	}
	a.stepAlerts(&s.ids, alertStep{
		Now: time.Now(), CPM: d.usv, Device: devRC, RcLim: a.rcLimits(), Demo: true,
		AttnEnded: s.attnOn, AlarmEnded: s.alarmOn, DangerEnded: s.dangOn,
	})
	s.attnOn, s.alarmOn, s.dangOn = false, false, false
	a.alarmEnded()
	a.emitActive()
}

// demoRcTick gera um segundo do Radiacode simulado (chamado pelo demoLoop).
func (a *App) demoRcTick(rng *rand.Rand, now time.Time) {
	a.mu.Lock()
	d := a.demo.rc
	if !a.demo.Active || !a.demo.Rc || d == nil {
		a.mu.Unlock()
		return
	}
	target, realistic := a.demo.RcUSv, a.demo.Realistic
	usv, cps := target, target*rcDemoCPSPerUSv
	if realistic {
		// contagem de Poisson do segundo, suavizada como o aparelho faz
		c := float64(poisson(rng, target*rcDemoCPSPerUSv))
		if d.cps == 0 {
			d.cps = target * rcDemoCPSPerUSv
		}
		d.cps += (c - d.cps) * 0.08
		cps = d.cps
		usv = cps / rcDemoCPSPerUSv
	}
	d.usv, d.cps = usv, cps
	d.ring.add(now, usv)
	m := now.Truncate(time.Minute)
	if !d.minT.IsZero() && m.After(d.minT) && d.minN >= 50 {
		d.minutes[d.minT.Unix()] = d.minAcc / float64(d.minN)
	}
	if !m.Equal(d.minT) {
		d.minT, d.minAcc, d.minN = m, 0, 0
	}
	d.minAcc += usv
	d.minN++
	set := a.settings
	a.mu.Unlock()

	// alertas com os limites do Radiacode, só na memória
	l := a.rcLimits()
	a.mu.Lock()
	s := &d.alert
	step := func(on *bool, enabled bool, enter float64) (started, ended bool) {
		was := *on
		switch {
		case !enabled:
			*on = false
		case usv >= enter:
			*on = true
		case usv < enter*0.9:
			*on = false
		}
		return *on && !was, was && !*on
	}
	attnStarted, attnEnded := step(&s.attnOn, set.AttentionEnabled, l.Attn)
	alarmStarted, alarmEnded := step(&s.alarmOn, set.AlarmEnabled, l.Alarm)
	dangStarted, dangEnded := step(&s.dangOn, true, l.Danger)
	attnOn, alarmOn := s.attnOn, s.alarmOn
	still := a.demo.Active && a.demo.rc == d // saiu do demo no meio: não abre nada
	a.mu.Unlock()
	if !still {
		return
	}
	a.stepAlerts(&s.ids, alertStep{
		Now: now, CPM: usv, Device: devRC, RcLim: l, Demo: true,
		AttnStarted: attnStarted, AttnEnded: attnEnded, AttnOn: attnOn,
		AlarmStarted: alarmStarted, AlarmEnded: alarmEnded, AlarmOn: alarmOn,
		DangerStarted: dangStarted, DangerEnded: dangEnded,
	})
	val := usvTxt(usv) + " µSv/h no RC102"
	switch {
	case dangStarted:
		a.flagWindowText("PERIGO", val, true, true)
	case alarmStarted:
		a.flagWindowText("ALARME", val, true, set.AlarmRaise)
	}
	if dangEnded && alarmOn {
		a.flagWindowText("ALARME", val, true, false)
	}
	if alarmEnded || dangEnded {
		a.alarmEnded()
	}
	a.emitRc()
}

// demoRcOverlay põe o Radiacode simulado no estado mostrado na tela.
func (a *App) demoRcOverlay(st *RcState) {
	a.mu.Lock()
	defer a.mu.Unlock()
	d := a.demo.rc
	if !a.demo.Active || !a.demo.Rc || d == nil {
		return
	}
	st.Demo = true
	if st.State != "conectado" {
		st.Via, st.Model = "demo", "RC102 (simulado)"
	}
	st.State = "conectado"
	st.Time = time.Now().UnixMilli()
	st.USvh, st.CPS = d.usv, d.cps
	st.USvhErr, st.CPSErr = 0, 0
	if d.cps > 0 {
		st.CPSErr = 100 / math.Sqrt(d.cps*10) // ~10 s de contagem
		st.USvhErr = st.CPSErr
	}
	st.AlertLevel = ""
	switch {
	case d.alert.dangOn:
		st.AlertLevel = "perigo"
	case d.alert.alarmOn:
		st.AlertLevel = "alarme"
	case d.alert.attnOn:
		st.AlertLevel = "atencao"
	}
}

// demoRcSeries devolve o Radiacode simulado para o gráfico: por segundo
// (passo < 1 min) ou por minuto. ok = falso sem simulação.
func (a *App) demoRcSeries(from, to time.Time, fine bool) (pts []sample, ok bool) {
	a.mu.Lock()
	defer a.mu.Unlock()
	d := a.demo.rc
	if !a.demo.Active || !a.demo.Rc || d == nil {
		return nil, false
	}
	if fine {
		return d.ring.between(from, to), true
	}
	for k, v := range d.minutes {
		t := time.Unix(k, 0)
		if !t.Before(from) && t.Before(to) {
			pts = append(pts, sample{t, v})
		}
	}
	// o minuto em andamento também
	if d.minN > 0 && !d.minT.Before(from) && d.minT.Before(to) {
		pts = append(pts, sample{d.minT, d.minAcc / float64(d.minN)})
	}
	return pts, true
}
