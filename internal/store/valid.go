package store

import (
	"math"
	"time"
)

// ---------- o que pode ser gravado (auditoria 2.76.0, B1 e B4) ----------

// Limites de uma leitura possível. Acima disso é defeito (aparelho, arquivo
// estragado ou conta de tempo morto que estourou), não radiação: o GMC-800
// satura bem antes de 10 milhões de CPM e o RC102 mede até 1 Sv/h.
const (
	MaxCPM  = 1e7
	MaxUSvh = 1e6
)

// SaneCPM diz se um CPM pode ser gravado: finito, sem sinal e abaixo do limite.
// NaN e infinito também quebram o envio do estado para a tela (JSON).
func SaneCPM(v float64) bool { return sane(v, MaxCPM) }

// SaneUSvh é o SaneCPM para µSv/h (e CPS) do RC102.
func SaneUSvh(v float64) bool { return sane(v, MaxUSvh) }

func sane(v, max float64) bool {
	return !math.IsNaN(v) && !math.IsInf(v, 0) && v >= 0 && v < max
}

// FirstYear é o ano mais antigo aceito: antes disso é relógio zerado.
const FirstYear = 2020

// FutureSlack é quanto um registro pode estar à frente do relógio do PC.
const FutureSlack = 2 * time.Minute

// SaneTimes separa os pontos com hora possível dos que vêm de um relógio
// errado: no futuro (mais de 2 min depois de now) ou antes de 2020. Um
// minuto do futuro gravado travava a gravação ao vivo até a hora real passar
// dele (B1).
func SaneTimes(pts []Point, now time.Time) (ok []Point, future, old int) {
	lim := now.Add(FutureSlack)
	ok = pts[:0:0]
	for _, p := range pts {
		switch {
		case p.T.After(lim):
			future++
		case p.T.Year() < FirstYear:
			old++
		default:
			ok = append(ok, p)
		}
	}
	return ok, future, old
}
