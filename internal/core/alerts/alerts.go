// Package alerts acha os episódios de alerta numa série de minutos (usado
// nos minutos que chegam do histórico do contador, quando o app não estava
// lendo ao vivo) e diz o nível de cada um. Não depende da interface nem do
// Wails: o app Android vai usar o mesmo pacote.
package alerts

import (
	"time"

	"radlog/internal/core/levels"
)

// Gap é a falta de dado que encerra um episódio.
const Gap = 70 * time.Minute

// Sample é um minuto da série (CPM).
type Sample struct {
	T   time.Time
	CPM float64
}

// Episode é um trecho seguido acima do nível de entrada.
type Episode struct {
	Start, Last, PeakAt time.Time
	First, Peak         float64
	Step                time.Duration // espaço entre as leituras no fim do episódio
}

// End é o fim do episódio: o último minuto mais o passo das leituras.
func (e Episode) End() time.Time { return e.Last.Add(e.Step) }

// Thresholds devolve a entrada e a saída: o nível de atenção, ou o limite do
// alarme do usuário se for menor; a saída fica em 90% da entrada.
func Thresholds(alarmCPM int, lv levels.Levels) (enter, exit float64) {
	enter, exit = lv.AttnEnter(), lv.AttnExit()
	if alarmCPM > 0 && float64(alarmCPM) < enter {
		enter = float64(alarmCPM)
		exit = enter * 0.9
	}
	return enter, exit
}

// Find acha os episódios em pts (em ordem de tempo). Um episódio começa no
// primeiro minuto na entrada ou acima e termina quando o valor cai abaixo da
// saída ou quando falta dado por mais de Gap.
func Find(pts []Sample, alarmCPM int, lv levels.Levels) []Episode {
	enter, exit := Thresholds(alarmCPM, lv)
	var out []Episode
	var cur *Episode
	var prev time.Time
	for _, p := range pts {
		if cur != nil && p.T.Sub(prev) > Gap {
			out = append(out, *cur)
			cur = nil
		}
		if cur == nil && p.CPM >= enter {
			cur = &Episode{Start: p.T, Last: p.T, PeakAt: p.T, First: p.CPM, Peak: p.CPM, Step: time.Minute}
		} else if cur != nil {
			if p.CPM < exit {
				out = append(out, *cur)
				cur = nil
			} else {
				if d := p.T.Sub(cur.Last); d > 0 && d <= Gap {
					cur.Step = d
				}
				cur.Last = p.T
				if p.CPM > cur.Peak {
					cur.Peak, cur.PeakAt = p.CPM, p.T
				}
			}
		}
		prev = p.T
	}
	if cur != nil {
		out = append(out, *cur)
	}
	return out
}

// Level é o nível de um episódio pelo pico: "perigo" (nível do app),
// "alarme" (limite do usuário) ou "atencao".
func Level(peak float64, alarmCPM int, lv levels.Levels) string {
	switch {
	case peak >= lv.DangerEnter():
		return "perigo"
	case peak >= float64(alarmCPM):
		return "alarme"
	}
	return "atencao"
}
