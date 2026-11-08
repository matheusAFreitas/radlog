package app

import (
	"fmt"
	"math"
	"time"
)

// ---------- Geiger saturado ----------
//
// Em radiação muito forte o tubo Geiger não consegue contar cada partícula:
// a leitura para de subir (fim da escala) ou, pior, cai e mostra menos do que
// o real ("fold-back"), porque o tubo fica travado. O app trata os dois casos
// como "Geiger saturado" e mantém o alarme de perigo ligado.
//
//  limite: a leitura chegou ao fim da escala do GMC-800 (2.000 µSv/h, manual);
//  queda:  a taxa do último segundo estava extrema (≥ 1.000 µSv/h) e caiu
//          para menos de 10% em até 5 s. Pode ser o tubo travando ou a fonte
//          indo embora; na dúvida, o aviso fica 60 s.
//
// Também guarda como a leitura subiu até o perigo: subir em segundos com a
// pessoa parada indica que algo chegou perto; subir em minutos, que a pessoa
// foi chegando perto de algo.

// Os limites (fim da escala, queda brusca e quanto tempo o aviso fica) vêm de
// Levels; padrão: 2.000 µSv/h, 1.000 µSv/h e 60 s.
const (
	satDropWin = 5 * time.Second
	riseWindow = 3 * time.Minute
)

type satSample struct {
	t   time.Time
	usv float64
}

// RiseInfo diz como a leitura chegou ao perigo.
type RiseInfo struct {
	FromUSv float64 `json:"fromUSv"` // última leitura normal antes de subir
	Seconds float64 `json:"seconds"` // quanto tempo levou
	Longer  bool    `json:"longer"`  // não achou leitura normal nos últimos 3 min
}

// satState acompanha uma fonte (contador real ou demo). Protegido por a.mu.
type satState struct {
	hist      []satSample
	highAt    time.Time
	highUSv   float64
	holdUntil time.Time
	on        bool
	reason    string
	rise      *RiseInfo
}

// step recebe a leitura (µSv/h pelo CPM) e a taxa do último segundo (µSv/h
// pelo CPS). Diz se está saturado e se a saturação começou agora.
func (st *satState) step(now time.Time, usv, instUSv float64, lv Levels) (on, started bool) {
	st.hist = append(st.hist, satSample{now, usv})
	cut := 0
	for cut < len(st.hist) && now.Sub(st.hist[cut].t) > riseWindow {
		cut++
	}
	st.hist = st.hist[cut:]

	reason := ""
	if usv >= lv.SatMaxUSv || instUSv >= lv.SatMaxUSv {
		reason = "limite"
	}
	if instUSv >= lv.SatDropUSv {
		if now.Sub(st.highAt) > satDropWin {
			st.highUSv = 0
		}
		st.highAt, st.highUSv = now, math.Max(st.highUSv, instUSv)
	} else if !st.highAt.IsZero() && now.Sub(st.highAt) <= satDropWin && instUSv < st.highUSv*0.1 {
		st.holdUntil = now.Add(lv.SatHold())
		st.highAt, st.highUSv = time.Time{}, 0
	}
	if reason == "" && now.Before(st.holdUntil) {
		reason = "queda"
	}
	started = reason != "" && !st.on
	st.on, st.reason = reason != "", reason
	return st.on, started
}

// riseFrom procura a última leitura abaixo de normalUSv antes de agora.
func (st *satState) riseFrom(now time.Time, normalUSv float64) *RiseInfo {
	if len(st.hist) == 0 {
		return nil
	}
	for i := len(st.hist) - 1; i >= 0; i-- {
		if st.hist[i].usv < normalUSv {
			return &RiseInfo{FromUSv: st.hist[i].usv, Seconds: now.Sub(st.hist[i].t).Seconds()}
		}
	}
	first := st.hist[0]
	return &RiseInfo{FromUSv: first.usv, Seconds: now.Sub(first.t).Seconds(), Longer: true}
}

func satText(reason string, lv Levels) string {
	if reason == "limite" {
		return fmt.Sprintf("Geiger saturado: a leitura chegou a %.0f µSv/h (fim da escala); o valor real pode ser maior", lv.SatMaxUSv)
	}
	return "Geiger saturado: a leitura caiu de repente depois de um valor extremo; o tubo pode ter travado e estar mostrando menos que o real"
}
