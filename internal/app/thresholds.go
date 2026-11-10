package app

import (
	"math"
	"time"

	"radlog/internal/gmc"
)

// ---------- limites em µSv/h ----------
//
// Desde a 2.29 os limites do GMC-800 (alarme, atenção e perigo) são
// escolhidos em µSv/h, a unidade da dose. O contador mede CPM, então o app
// guarda o µSv/h e acha o CPM equivalente pela calibração em uso: trocar o
// fator muda o CPM em que o alarme toca, não o µSv/h. Os campos em CPM do
// config.json (AlarmCPM, Levels.AttentionCPM/DangerCPM) ficam como o valor
// derivado, para o resto do motor seguir comparando CPM com CPM.

// cpmForUSv é o CPM em que a calibração dá usv µSv/h (o menor inteiro que
// chega lá), por busca binária: a curva do contador é crescente.
func cpmForUSv(cal gmc.Calibration, usv float64) int {
	if usv <= 0 {
		return 0
	}
	// tolerância relativa pequena: o µSv/h migrado de um CPM inteiro volta
	// exatamente para ele, sem erro de arredondamento
	target := usv * (1 - 1e-9)
	lo, hi := 0, 1
	for cal.USv(float64(hi)) < target && hi < 1e9 {
		lo, hi = hi, hi*2
	}
	for hi-lo > 1 { // invariante: USv(lo) < target <= USv(hi)
		mid := (lo + hi) / 2
		if cal.USv(float64(mid)) < target {
			lo = mid
		} else {
			hi = mid
		}
	}
	return hi
}

// migrateThrLocked passa os limites antigos (em CPM) para µSv/h pela
// calibração de agora, uma vez: o alarme continua tocando no mesmo CPM de
// hoje. Diz se mudou algo. (a.mu travado)
func (a *App) migrateThrLocked() bool {
	s := &a.settings
	if s.AlarmUSv > 0 && s.Levels.AttentionUSv > 0 && s.Levels.DangerUSv > 0 {
		return false
	}
	cal := a.convCalLocked()
	if s.AlarmUSv <= 0 {
		s.AlarmUSv = cal.USv(float64(s.AlarmCPM))
	}
	if s.Levels.AttentionUSv <= 0 {
		s.Levels.AttentionUSv = cal.USv(float64(s.Levels.AttentionCPM))
	}
	if s.Levels.DangerUSv <= 0 {
		s.Levels.DangerUSv = cal.USv(float64(s.Levels.DangerCPM))
	}
	return true
}

// effLocked são as configurações com os limites em CPM tirados dos µSv/h
// pela calibração de agora: é o que o motor de alertas compara com a
// leitura. (a.mu travado)
func (a *App) effLocked() Settings {
	s := a.settings
	cal := a.convCalLocked()
	if s.AlarmUSv > 0 {
		s.AlarmCPM = max(cpmForUSv(cal, s.AlarmUSv), 1)
	}
	if s.Levels.AttentionUSv > 0 {
		s.Levels.AttentionCPM = max(cpmForUSv(cal, s.Levels.AttentionUSv), 1)
	}
	if s.Levels.DangerUSv > 0 {
		s.Levels.DangerCPM = max(cpmForUSv(cal, s.Levels.DangerUSv), 1)
	}
	return s
}

// clampThr segura um limite em µSv/h dentro de [lo, hi].
func clampThr(v, lo, hi float64) float64 {
	return math.Min(math.Max(v, lo), hi)
}

// fmtUSv escreve um valor em µSv/h com 3 algarismos significativos e
// vírgula decimal (0,564; 5,64; 56,4; 564).
func fmtUSv(v float64) string {
	if v <= 0 {
		return "0"
	}
	d := 2 - int(math.Floor(math.Log10(v)))
	if d < 0 {
		d = 0
	}
	return fmtBR(v, d)
}

// thrDesc descreve um limite em µSv/h (o de antes da migração, em CPM,
// pela calibração de agora).
func thrDesc(usv float64, cpm int, cal gmc.Calibration) string {
	if usv <= 0 {
		usv = cal.USv(float64(cpm))
	}
	return fmtUSv(usv) + " µSv/h"
}

// evUSvLocked preenche o valor e o pico de um alerta do GMC em µSv/h, pela
// calibração que valia na hora. (a.mu travado)
func (a *App) evUSvLocked(e *Event) {
	if e.Kind != kindAlert || e.Device == devRC {
		return
	}
	tl := a.calTLLocked()
	// fator da hora do alerta, para a tela converter os textos antigos em CPM
	e.USvPerCPM = tl.USvAt(time.UnixMilli(e.Time), 100) / 100
	if e.CPM > 0 {
		e.USv = tl.USvAt(time.UnixMilli(e.Time), e.CPM)
	}
	if e.Peak > 0 {
		at := e.PeakAt
		if at == 0 {
			at = e.Time
		}
		e.PeakUSv = tl.USvAt(time.UnixMilli(at), e.Peak)
	}
}

// thrUSvLocked são os limites de atenção, alarme e perigo em µSv/h (os
// escolhidos; antes da migração, os CPM pela calibração de agora).
// (a.mu travado)
func thrUSvLocked(a *App, eff Settings) (attn, alarm, danger float64) {
	cal := a.convCalLocked()
	lv := eff.Levels.Norm()
	or := func(v float64, cpm int) float64 {
		if v > 0 {
			return v
		}
		return cal.USv(float64(cpm))
	}
	return or(lv.AttentionUSv, lv.AttentionCPM), or(eff.AlarmUSv, eff.AlarmCPM), or(lv.DangerUSv, lv.DangerCPM)
}
