package app

import (
	"fmt"
	"time"
)

// ---------- contador sem contagens ----------
//
// Com o fundo normal (~20–30 CPM), passar um minuto inteiro sem nenhuma
// contagem é praticamente impossível. CPM em 0 por 2 min seguidos (ou seja,
// uns 3 min sem um único pulso) indica tubo ou alta tensão com problema.
// Se o Geiger estiver saturado, o aviso de saturação já cobre (saturation.go).

const noCountAfter = 2 * time.Minute

type noCountState struct {
	since time.Time // primeira leitura em 0 da sequência
	on    bool
}

// step diz se o aviso começou ou acabou agora.
func (n *noCountState) step(now time.Time, cpm uint32, saturated bool) (started, ended bool) {
	if cpm > 0 || saturated {
		ended = n.on && !saturated
		*n = noCountState{}
		return false, ended
	}
	if n.since.IsZero() {
		n.since = now
	}
	if !n.on && now.Sub(n.since) >= noCountAfter {
		n.on = true
		return true, false
	}
	return false, false
}

func noCountText(since, now time.Time) string {
	m := int(now.Sub(since).Round(time.Minute) / time.Minute)
	if m < 1 {
		m = 1
	}
	return fmt.Sprintf("Contador voltou a contar depois de %d min em 0 CPM", m)
}
