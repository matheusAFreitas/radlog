package app

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"radlog/internal/store"
)

// ---------- períodos fora de casa ----------
//
// O usuário marca trechos em que o contador estava fora de casa (passeio,
// viagem...). As leituras continuam guardadas e entram na dose, mas ficam fora
// de tudo que descreve o fundo da casa: a mediana do fundo (e o "normal" da
// leitura), o fundo e o teto do dosímetro, o fundo diário e o padrão por hora.

// Period é um trecho marcado.
type Period struct {
	ID   int64  `json:"id"`
	From int64  `json:"from"` // ms
	To   int64  `json:"to"`   // ms
	Name string `json:"name"`
}

func (a *App) periodsPath() string { return filepath.Join(a.dataDir, "fora-de-casa.json") }

func (a *App) loadPeriods() {
	b, err := os.ReadFile(a.periodsPath())
	if err != nil {
		return
	}
	var list []Period
	if json.Unmarshal(b, &list) == nil {
		sort.Slice(list, func(i, j int) bool { return list[i].From < list[j].From })
		a.periods = list
	}
}

func (a *App) savePeriodsLocked() error {
	if a.dataDir == "" {
		return nil
	}
	b, _ := json.MarshalIndent(a.periods, "", "  ")
	tmp := a.periodsPath() + ".tmp"
	if err := os.WriteFile(tmp, b, 0o644); err != nil {
		return err
	}
	return os.Rename(tmp, a.periodsPath())
}

// GetPeriods devolve os períodos marcados, do mais antigo ao mais novo.
func (a *App) GetPeriods() []Period {
	a.mu.Lock()
	defer a.mu.Unlock()
	return append([]Period{}, a.periods...)
}

// SavePeriod cria (ID 0) ou altera um período.
func (a *App) SavePeriod(p Period) (Period, error) {
	if p.To < p.From {
		p.From, p.To = p.To, p.From
	}
	if p.To-p.From < 60_000 {
		return p, errors.New("o período precisa ter pelo menos 1 minuto")
	}
	p.Name = strings.TrimSpace(p.Name)
	if p.Name == "" {
		p.Name = "Fora de casa"
	}
	a.mu.Lock()
	if p.ID == 0 {
		var max int64
		for _, q := range a.periods {
			if q.ID > max {
				max = q.ID
			}
		}
		p.ID = max + 1
		a.periods = append(a.periods, p)
	} else {
		found := false
		for i := range a.periods {
			if a.periods[i].ID == p.ID {
				a.periods[i] = p
				found = true
			}
		}
		if !found {
			a.mu.Unlock()
			return p, errors.New("período não encontrado")
		}
	}
	sort.Slice(a.periods, func(i, j int) bool { return a.periods[i].From < a.periods[j].From })
	a.bgAt = time.Time{} // o fundo muda: recalcula
	err := a.savePeriodsLocked()
	a.mu.Unlock()
	a.emit("periods", a.GetPeriods())
	return p, err
}

// DeletePeriod apaga um período.
func (a *App) DeletePeriod(id int64) error {
	a.mu.Lock()
	kept := a.periods[:0]
	for _, q := range a.periods {
		if q.ID != id {
			kept = append(kept, q)
		}
	}
	a.periods = kept
	a.bgAt = time.Time{}
	err := a.savePeriodsLocked()
	a.mu.Unlock()
	a.emit("periods", a.GetPeriods())
	return err
}

// awayChecker diz se o GMC-800 estava fora de casa num instante: num período
// marcado, a não ser que ele tenha sido marcado em casa (roles.go). Para o
// RadiaCode, use rcAwayChecker.
func (a *App) awayChecker() func(time.Time) bool { return a.awayCheckerFor("gmc") }

// periodChecker devolve uma função que diz se um instante está num período
// fora de casa (cópia dos períodos, para usar sem segurar o lock).
func (a *App) periodChecker() func(time.Time) bool {
	a.mu.Lock()
	list := append([]Period{}, a.periods...)
	a.mu.Unlock()
	if len(list) == 0 {
		return func(time.Time) bool { return false }
	}
	return func(t time.Time) bool {
		ms := t.UnixMilli()
		// poucos períodos: busca simples
		for _, p := range list {
			if ms >= p.From && ms < p.To {
				return true
			}
		}
		return false
	}
}

// homeOnly tira os pontos do GMC fora de casa (roles.go) e
// das medições de objeto. Para o RadiaCode, use rcHomeOnly.
func (a *App) homeOnly(pts []store.Point) []store.Point {
	away := a.awayChecker()
	measuring := a.measuringChecker()
	out := pts[:0:0]
	for _, p := range pts {
		if !away(p.T) && !measuring(p.T) {
			out = append(out, p)
		}
	}
	return out
}

// NewRange é um trecho contínuo de minutos que entrou ao baixar o histórico.
type NewRange struct {
	From    int64   `json:"from"`
	To      int64   `json:"to"`
	Minutes int     `json:"minutes"`
	AvgCPM  float64 `json:"avgCPM"`
}

const (
	newRangeGap    = 5 * time.Minute // buraco maior que isso separa os trechos
	awayAskDefault = 15              // min: trechos menores não valem a pergunta (Configurações)
)

// newRanges junta os minutos novos em trechos contínuos e devolve os que
// valem perguntar se foram fora de casa (30 min ou mais, ainda não marcados).
func (a *App) newRanges(pts []store.Point) []NewRange {
	if len(pts) == 0 {
		return []NewRange{}
	}
	sorted := append([]store.Point(nil), pts...)
	sort.Slice(sorted, func(i, j int) bool { return sorted[i].T.Before(sorted[j].T) })
	away := a.awayChecker()
	a.mu.Lock()
	minLen := time.Duration(normAwayAsk(a.settings.AwayAskMin)) * time.Minute
	a.mu.Unlock()
	out := []NewRange{}
	start, last, n, sum := sorted[0].T, sorted[0].T, 1, sorted[0].CPM
	flush := func() {
		end := last.Add(time.Minute)
		if end.Sub(start) < minLen {
			return
		}
		// já marcado (começo e fim dentro de períodos)? não pergunta de novo
		if away(start) && away(last) {
			return
		}
		out = append(out, NewRange{From: start.UnixMilli(), To: end.UnixMilli(), Minutes: n, AvgCPM: sum / float64(n)})
	}
	for _, p := range sorted[1:] {
		if p.T.Sub(last) > newRangeGap {
			flush()
			start, n, sum = p.T, 0, 0
		}
		last = p.T
		n++
		sum += p.CPM
	}
	flush()
	return out
}

// normAwayAsk deixa o tempo mínimo da pergunta entre 5 min e 4 h.
func normAwayAsk(m int) int {
	switch {
	case m <= 0:
		return awayAskDefault
	case m < 5:
		return 5
	case m > 240:
		return 240
	}
	return m
}
