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
