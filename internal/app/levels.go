package app

import "time"

// ---------- níveis definidos pelo app ----------
//
// Atenção, perigo e Geiger saturado têm valores padrão do app, mas dá para
// ajustar na aba "Níveis do app" das Configurações. Zero = padrão.

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

// DefaultLevels são os valores de fábrica do app.
func DefaultLevels() Levels {
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

// norm troca zeros pelos padrões e segura valores absurdos.
func (l Levels) norm() Levels {
	d := DefaultLevels()
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

// dtCPM corrige um CPM (soma de 60 s: usa a taxa média do minuto).
func (l Levels) dtCPM(cpm float64) float64 {
	if !l.DeadTimeOn || cpm <= 0 {
		return cpm
	}
	return cpm * dtFactor(cpm/60, l.DeadTimeUs*1e-6)
}

// dtCPS corrige a contagem de 1 segundo.
func (l Levels) dtCPS(cps float64) float64 {
	if !l.DeadTimeOn || cps <= 0 {
		return cps
	}
	return cps * dtFactor(cps, l.DeadTimeUs*1e-6)
}

// saída um pouco abaixo da entrada, para não ficar entrando e saindo
func (l Levels) attnEnter() float64   { return float64(l.AttentionCPM) }
func (l Levels) attnExit() float64    { return float64(l.AttentionCPM) * 0.9 }
func (l Levels) dangerEnter() float64 { return float64(l.DangerCPM) }
func (l Levels) dangerExit() float64  { return float64(l.DangerCPM) * 0.9 }
func (l Levels) satHold() time.Duration {
	return time.Duration(l.SatHoldSec) * time.Second
}

// GetDefaultLevels devolve os padrões (para o botão "Restaurar padrões").
func (a *App) GetDefaultLevels() Levels { return DefaultLevels() }

// ceilTo arredonda t para cima no múltiplo de step (se já for múltiplo, fica).
func ceilTo(t time.Time, step time.Duration) time.Time {
	f := t.Truncate(step)
	if f.Equal(t) {
		return t
	}
	return f.Add(step)
}
