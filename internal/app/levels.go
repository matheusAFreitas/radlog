package app

import (
	"time"

	"radlog/internal/core/levels"
)

// ---------- níveis definidos pelo app ----------
//
// Atenção, perigo e Geiger saturado têm valores padrão do app, mas dá para
// ajustar na aba "Níveis do app" das Configurações. Zero = padrão.

// Levels são os níveis do app (o tipo mora em internal/core/levels).
type Levels = levels.Levels

// DefaultLevels são os valores de fábrica do app.
func DefaultLevels() Levels { return levels.Default() }

// ceilTo arredonda t para cima no múltiplo de step (se já for múltiplo, fica).
func ceilTo(t time.Time, step time.Duration) time.Time {
	f := truncLocal(t, step)
	if f.Equal(t) {
		return t
	}
	return f.Add(step)
}

// truncLocal é o t.Truncate(step) pelo relógio local: o Truncate do Go corta
// pelo UTC, então no Brasil cada "dia" dos gráficos de 1 ano ia das 21:00
// às 21:00 (auditoria 2.76.0, B10). Passos menores que 1 h não mudam.
func truncLocal(t time.Time, step time.Duration) time.Time {
	if step < time.Hour {
		return t.Truncate(step)
	}
	_, off := t.Zone()
	shift := time.Duration(off) * time.Second
	return t.Add(shift).Truncate(step).Add(-shift)
}
