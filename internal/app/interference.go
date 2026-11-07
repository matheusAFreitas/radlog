package app

import (
	"fmt"
	"math"
	"strings"
	"time"

	"radlog/internal/store"
)

// ---------- pico suspeito de interferência ----------
//
// Radiação de verdade sobe a contagem por vários segundos seguidos (um objeto
// perto, chuva, radônio). Ruído elétrico (celular encostado, fonte, motor,
// descarga) costuma aparecer como um segundo isolado com contagens demais e
// os vizinhos normais. O detector olha cada segundo com 10 s de atraso, para
// ver os 10 s antes e os 10 s depois:
//   - o segundo do meio é quase impossível pelo acaso (Poisson, cauda < 1e-6)
//     mesmo com a taxa mais alta entre o fundo e a média dos vizinhos;
//   - os 20 vizinhos somados estão normais para o fundo (cauda > 1e-3).

const (
	ifHalf      = 10   // segundos de cada lado
	ifTailMax   = 1e-6 // o segundo do meio precisa ser mais raro que isto
	ifCalmMin   = 1e-3 // os vizinhos não podem estar subindo
	ifMinCounts = 5
	ifQuiet     = 5 * time.Minute // no máximo um aviso a cada 5 min (os outros somam)
)

type ifSample struct {
	t   time.Time
	cps float64
}

// spikeSuspect é um segundo marcado como provável interferência.
type spikeSuspect struct {
	At       time.Time
	Counts   float64
	Expected float64 // contagens esperadas por segundo
}

type interferenceDetector struct {
	win      []ifSample
	lastWarn time.Time
	pending  int // suspeitos calados desde o último aviso
}

// step recebe um segundo (CPS cru do contador) e o fundo em CPM (0 se não
// houver). Devolve o segundo do meio da janela se ele for suspeito.
func (d *interferenceDetector) step(t time.Time, cps, bgCPM float64) *spikeSuspect {
	// buraco na leitura (porta travou, demo ligado/desligado): recomeça
	if n := len(d.win); n > 0 && t.Sub(d.win[n-1].t) > 3*time.Second {
		d.win = d.win[:0]
	}
	d.win = append(d.win, ifSample{t, cps})
	if len(d.win) < 2*ifHalf+1 {
		return nil
	}
	if len(d.win) > 2*ifHalf+1 {
		d.win = d.win[len(d.win)-(2*ifHalf+1):]
	}
	mid := d.win[ifHalf]
	k := mid.cps
	if k < ifMinCounts {
		return nil
	}
	var sum float64
	for i, s := range d.win {
		if i != ifHalf {
			sum += s.cps
		}
	}
	lambda := bgCPM / 60
	if lambda <= 0 {
		lambda = sum / (2 * ifHalf) // sem fundo ainda: a média dos vizinhos
	}
	if lambda <= 0 {
		lambda = 0.05
	}
	nbMean := sum / (2 * ifHalf)
	if poissonTail(k, math.Max(lambda, nbMean)) >= ifTailMax {
		return nil
	}
	if poissonTail(sum, lambda*2*ifHalf) <= ifCalmMin {
		return nil // os vizinhos também estão altos: parece radiação de verdade
	}
	return &spikeSuspect{At: mid.t, Counts: k, Expected: lambda}
}

// poissonTail é P(X ≥ k) para X ~ Poisson(λ).
func poissonTail(k, lambda float64) float64 {
	k = math.Ceil(k)
	if k <= 0 {
		return 1
	}
	if lambda <= 0 {
		return 0
	}
	logTerm := func(i float64) float64 {
		lg, _ := math.Lgamma(i + 1)
		return -lambda + i*math.Log(lambda) - lg
	}
	if k > lambda {
		// soma a cauda a partir de k (os termos só diminuem)
		total := 0.0
		for i := k; i < k+400; i++ {
			t := math.Exp(logTerm(i))
			total += t
			if t < total*1e-12 {
				break
			}
		}
		return math.Min(1, total)
	}
	below := 0.0
	for i := 0.0; i < k; i++ {
		below += math.Exp(logTerm(i))
	}
	return math.Max(0, 1-below)
}

// checkInterference roda a cada segundo da leitura ao vivo.
func (a *App) checkInterference(now time.Time, rawCPS float64) {
	bg, _ := a.backgroundCPM()
	a.mu.Lock()
	s := a.interf.step(now, rawCPS, bg)
	if s == nil {
		a.mu.Unlock()
		return
	}
	a.markMinuteLocked(s.At, store.ObsInterference)
	if !a.interf.lastWarn.IsZero() && s.At.Sub(a.interf.lastWarn) < ifQuiet {
		a.interf.pending++
		a.mu.Unlock()
		a.logf("interferência: outro segundo suspeito às %s (%.0f contagens; esperado ~%s), aviso já dado há menos de 5 min",
			s.At.Format("15:04:05"), s.Counts, fmtBR(s.Expected, 2))
		return
	}
	more := a.interf.pending
	a.interf.lastWarn, a.interf.pending = s.At, 0
	// alerta que começou por causa deste pico: anota nele
	var flagged []int64
	for i := range a.events {
		e := &a.events[i]
		if e.Kind != kindAlert || e.Retro || e.Demo || e.Device == devRC {
			continue
		}
		st := time.UnixMilli(e.Time)
		if !st.Before(s.At.Add(-2*time.Second)) && st.Before(s.At.Add(61*time.Second)) && !strings.Contains(e.Text, "interferência") {
			e.Text += fmt.Sprintf(" · possível interferência (%.0f contagens em 1 s às %s)", s.Counts, s.At.Format("15:04:05"))
			flagged = append(flagged, e.ID)
		}
	}
	a.mu.Unlock()
	if len(flagged) > 0 {
		a.saveEvents()
	}
	a.logEvent(interferenceText(*s, more))
}

func interferenceText(s spikeSuspect, more int) string {
	t := fmt.Sprintf("Pico suspeito de interferência: %.0f contagens em 1 s às %s, com os 20 s em volta normais (esperado ~%s por segundo). "+
		"Parece ruído elétrico (celular encostado, fonte, motor), não radiação", s.Counts, s.At.Format("15:04:05"), fmtBR(s.Expected, 2))
	if more > 0 {
		t += fmt.Sprintf(" (mais %d desde o último aviso)", more)
	}
	return t
}
