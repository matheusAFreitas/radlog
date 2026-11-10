package app

import (
	"encoding/json"
	"os"
	"path/filepath"
	"time"
)

// ---------- chuva por dia, para sempre ----------
//
// A chuva por hora (chuva.json) fica só 60 dias. Para o calendário do ano
// basta saber quanto choveu em cada dia, e isso precisa ficar para sempre:
// chuva-dias.json guarda "dia local -> mm" sem o lugar (lat/lon nunca vão
// para este arquivo). Cada busca de chuva atualiza os dias que ela cobre; um
// dia só é trocado por uma conta com tantas horas quanto a guardada ou mais,
// para o dia da borda (com horas já apagadas) não encolher.

const rainDaysFile = "chuva-dias.json"
const rainDaysVersion = 1

// RainDay é a chuva de um dia (data local).
type RainDay struct {
	MM    float64 `json:"mm"`
	Hours int     `json:"h"` // horas com dado nesse dia
}

type rainDaysStore struct {
	V    int                `json:"v"`
	Days map[string]RainDay `json:"days"` // "2006-01-02" (hora local) -> chuva
}

func (a *App) rainDaysPath() string { return filepath.Join(a.dataDir, rainDaysFile) }

func (a *App) loadRainDays() {
	a.rainDays = &rainDaysStore{V: rainDaysVersion, Days: map[string]RainDay{}}
	b, err := os.ReadFile(a.rainDaysPath())
	if err != nil {
		return
	}
	var s rainDaysStore
	if json.Unmarshal(b, &s) == nil && s.V == rainDaysVersion && s.Days != nil {
		a.rainDays = &s
	}
}

// rainDaysFrom soma a chuva por hora em dias locais.
func rainDaysFrom(hours map[int64]float64, loc *time.Location) map[string]RainDay {
	out := map[string]RainDay{}
	for k, mm := range hours {
		d := time.Unix(k, 0).In(loc).Format("2006-01-02")
		r := out[d]
		r.MM += mm
		r.Hours++
		out[d] = r
	}
	for d, r := range out {
		r.MM = float64(int(r.MM*10+0.5)) / 10
		out[d] = r
	}
	return out
}

// mergeRainDaysLocked junta a chuva por hora de agora no arquivo por dia.
// Devolve true se mudou algo. Chamar com a.mu travado.
func (a *App) mergeRainDaysLocked(hours map[int64]float64) bool {
	if a.rainDays == nil {
		a.rainDays = &rainDaysStore{V: rainDaysVersion, Days: map[string]RainDay{}}
	}
	changed := false
	for d, r := range rainDaysFrom(hours, time.Local) {
		old, ok := a.rainDays.Days[d]
		if ok && (r.Hours < old.Hours || r == old) {
			continue
		}
		a.rainDays.Days[d] = r
		changed = true
	}
	return changed
}

func (a *App) saveRainDaysLocked() {
	if a.dataDir == "" || a.rainDays == nil {
		return
	}
	if b, err := json.Marshal(a.rainDays); err == nil {
		_ = os.WriteFile(a.rainDaysPath(), b, 0o644)
	}
}

// mergeRainDaysBackup junta os dias de um backup: só entra o dia que não
// existe aqui ou que lá tem mais horas.
func (a *App) mergeRainDaysBackup(b []byte) {
	var s rainDaysStore
	if json.Unmarshal(b, &s) != nil || s.V != rainDaysVersion {
		return
	}
	a.mu.Lock()
	defer a.mu.Unlock()
	if a.rainDays == nil {
		a.rainDays = &rainDaysStore{V: rainDaysVersion, Days: map[string]RainDay{}}
	}
	changed := false
	for d, r := range s.Days {
		if old, ok := a.rainDays.Days[d]; !ok || r.Hours > old.Hours {
			a.rainDays.Days[d] = r
			changed = true
		}
	}
	if changed {
		a.saveRainDaysLocked()
	}
}
