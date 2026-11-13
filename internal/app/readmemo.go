package app

import (
	"sort"
	"sync"
	"time"

	"radlog/internal/store"
)

// ---------- leitura uma vez só por consulta (auditoria 2.76.0, P1) ----------
//
// A Minha dose monta 4 períodos para 2 papéis e cada um relia os 12 meses
// dos dois aparelhos: ~8 leituras do ano, 3,5 s com um ano de dados. Durante
// a consulta, o ano de cada aparelho é lido uma vez e os períodos usam
// pedaços dele. A memória dura só a consulta: nada fica guardado depois.

type readMemo struct {
	mu       sync.Mutex
	depth    int
	from, to time.Time
	gmc      []store.Point
	gmcOK    bool
	rc       []RcPoint
	rcOK     bool
}

// beginReadMemo liga a memória para [from, to) até a função devolvida ser
// chamada. Consultas ao mesmo tempo dividem a mesma.
func (a *App) beginReadMemo(from, to time.Time) func() {
	m := &a.memo
	m.mu.Lock()
	if m.depth == 0 {
		m.from, m.to = from, to
		m.gmc, m.gmcOK, m.rc, m.rcOK = nil, false, nil, false
	}
	m.depth++
	m.mu.Unlock()
	return func() {
		m.mu.Lock()
		m.depth--
		if m.depth == 0 {
			m.gmc, m.rc = nil, nil
			m.gmcOK, m.rcOK = false, false
		}
		m.mu.Unlock()
	}
}

// covers diz se [from, to) está dentro da memória ligada (com m.mu).
func (m *readMemo) covers(from, to time.Time) bool {
	return m.depth > 0 && !from.Before(m.from) && !to.After(m.to)
}

// storeRange é o a.store.Range, pela memória quando ela cobre o trecho.
func (a *App) storeRange(from, to time.Time) ([]store.Point, error) {
	m := &a.memo
	m.mu.Lock()
	if !m.covers(from, to) {
		m.mu.Unlock()
		return a.store.Range(from, to)
	}
	if !m.gmcOK {
		pts, err := a.store.Range(m.from, m.to)
		if err != nil {
			m.mu.Unlock()
			return nil, err
		}
		m.gmc, m.gmcOK = pts, true
	}
	all := m.gmc
	m.mu.Unlock()
	i := sort.Search(len(all), func(i int) bool { return !all[i].T.Before(from) })
	j := sort.Search(len(all), func(i int) bool { return !all[i].T.Before(to) })
	// sem copiar (copiar o ano custava quase o mesmo que ler): a capacidade
	// limitada faz um append de quem chama ir para outro lugar; ninguém muda
	// os pontos no lugar (homeOnly e rcHomeOnly montam lista nova)
	return all[i:j:j], nil
}

// rcRange lê os minutos do Radiacode entre from e to (pela memória quando
// ela cobre o trecho).
func (a *App) rcRange(from, to time.Time) []RcPoint {
	m := &a.memo
	m.mu.Lock()
	if !m.covers(from, to) {
		m.mu.Unlock()
		return a.rcRangeDisk(from, to)
	}
	if !m.rcOK {
		m.rc, m.rcOK = a.rcRangeDisk(m.from, m.to), true
	}
	all := m.rc
	m.mu.Unlock()
	i := sort.Search(len(all), func(i int) bool { return !all[i].T.Before(from) })
	j := sort.Search(len(all), func(i int) bool { return !all[i].T.Before(to) })
	return all[i:j:j]
}
