// Package levels tem os níveis do app (atenção, perigo, Geiger saturado), a
// correção de tempo morto e a classificação de um CPM. Não depende da
// interface nem do Wails: o app Android vai usar o mesmo pacote.
package levels

import "time"

// Níveis fixos da faixa do manual do GMC-800 (a escada de cores).
const (
	ManualAttnCPM = 50  // atenção do manual
	WarningCPM    = 100 // alerta do manual (e o alarme padrão do usuário)
)

// Levels são os níveis do app.
type Levels struct {
	AttentionCPM int     `json:"attentionCPM"` // aviso de atenção
	DangerCPM    int     `json:"dangerCPM"`    // alarme de perigo
	SnoozeMin    int     `json:"snoozeMin"`    // "Silenciar" no perigo dura isso
	SatMaxUSv    float64 `json:"satMaxUSv"`    // Geiger saturado: fim da escala
	SatDropUSv   float64 `json:"satDropUSv"`   // Geiger saturado: queda brusca a partir de
	SatHoldSec   int     `json:"satHoldSec"`   // aviso de queda brusca fica por
	// Compensação de tempo morto (modelo não paralisável, como no Rad Pro):
	// taxa real = medida ÷ (1 − medida × τ), com o fator limitado a 10×.
	DeadTimeOn bool    `json:"deadTimeOn"`
	DeadTimeUs float64 `json:"deadTimeUs"` // τ em microssegundos
}

// Default são os valores de fábrica do app.
func Default() Levels {
	return Levels{
		AttentionCPM: 50,   // faixa de atenção do manual do GMC-800
		DangerCPM:    1000, // cerca de 10 µSv/h
		SnoozeMin:    5,
		SatMaxUSv:    2000, // fim da escala do GMC-800 (manual)
		SatDropUSv:   1000,
		SatHoldSec:   60,
		DeadTimeOn:   false,
		DeadTimeUs:   200, // típico do M4011 (Rad Pro: 50–200 µs); ajuste se souber o do seu tubo
	}
}

func clampInt(v, lo, hi, def int) int {
	switch {
	case v <= 0:
		return def
	case v < lo:
		return lo
	case v > hi:
		return hi
	}
	return v
}

func clampF(v, lo, hi, def float64) float64 {
	switch {
	case v <= 0:
		return def
	case v < lo:
		return lo
	case v > hi:
		return hi
	}
	return v
}

// Norm troca zeros pelos padrões e segura valores absurdos.
func (l Levels) Norm() Levels {
	d := Default()
	l.AttentionCPM = clampInt(l.AttentionCPM, 5, 100000, d.AttentionCPM)
	l.DangerCPM = clampInt(l.DangerCPM, 50, 10000000, d.DangerCPM)
	l.SnoozeMin = clampInt(l.SnoozeMin, 1, 60, d.SnoozeMin)
	l.SatMaxUSv = clampF(l.SatMaxUSv, 10, 1e6, d.SatMaxUSv)
	l.SatDropUSv = clampF(l.SatDropUSv, 1, 1e6, d.SatDropUSv)
	l.SatHoldSec = clampInt(l.SatHoldSec, 5, 3600, d.SatHoldSec)
	l.DeadTimeUs = clampF(l.DeadTimeUs, 5, 500, d.DeadTimeUs)
	return l
}

// ---------- tempo morto ----------
//
// Depois de cada pulso o tubo fica "cego" por τ; em radiação alta ele perde
// contagens. A correção (modelo não paralisável) é n = m ÷ (1 − m·τ), com m
// em pulsos por segundo. Como no Rad Pro, o fator para em 10× (denominador
// mínimo 0,1): acima disso o tubo está saturado e a conta não vale mais.

const dtMaxFactor = 10

// dtFactor é o fator de correção para uma taxa de m pulsos por segundo.
func dtFactor(perSec, tauSec float64) float64 {
	den := 1 - perSec*tauSec
	if den < 1.0/dtMaxFactor {
		den = 1.0 / dtMaxFactor
	}
	return 1 / den
}

// DtCPM corrige um CPM (soma de 60 s: usa a taxa média do minuto).
func (l Levels) DtCPM(cpm float64) float64 {
	if !l.DeadTimeOn || cpm <= 0 {
		return cpm
	}
	return cpm * dtFactor(cpm/60, l.DeadTimeUs*1e-6)
}

// DtCPS corrige a contagem de 1 segundo.
func (l Levels) DtCPS(cps float64) float64 {
	if !l.DeadTimeOn || cps <= 0 {
		return cps
	}
	return cps * dtFactor(cps, l.DeadTimeUs*1e-6)
}

// saída um pouco abaixo da entrada, para não ficar entrando e saindo
func (l Levels) AttnEnter() float64   { return float64(l.AttentionCPM) }
func (l Levels) AttnExit() float64    { return float64(l.AttentionCPM) * 0.9 }
func (l Levels) DangerEnter() float64 { return float64(l.DangerCPM) }
func (l Levels) DangerExit() float64  { return float64(l.DangerCPM) * 0.9 }
func (l Levels) SatHold() time.Duration {
	return time.Duration(l.SatHoldSec) * time.Second
}

// Level classifica um CPM: "perigo" (nível do app), "alerta" e "atencao"
// (faixas do manual) ou "seguro".
func Level(cpm float64, lv Levels) string {
	switch {
	case cpm >= lv.DangerEnter():
		return "perigo"
	case cpm >= WarningCPM:
		return "alerta"
	case cpm >= ManualAttnCPM:
		return "atencao"
	}
	return "seguro"
}

// DangerStep atualiza o estado de perigo e diz se começou ou terminou. Não
// depende do alarme do usuário (só dá para silenciar o som, na tela).
func DangerStep(on *bool, cpm float64, lv Levels) (started, ended bool) {
	switch {
	case cpm >= lv.DangerEnter() && !*on:
		*on = true
		return true, false
	case cpm < lv.DangerExit() && *on:
		*on = false
		return false, true
	}
	return false, false
}

// AttentionStep atualiza o estado de atenção e diz se um episódio começou.
// A saída fica um pouco abaixo da entrada, para não ficar entrando e saindo
// quando o valor oscila perto do nível.
func AttentionStep(on *bool, enabled bool, cpm float64, lv Levels) (started bool) {
	switch {
	case !enabled:
		*on = false
	case cpm >= lv.AttnEnter() && !*on:
		*on = true
		return true
	case cpm < lv.AttnExit():
		*on = false
	}
	return false
}
