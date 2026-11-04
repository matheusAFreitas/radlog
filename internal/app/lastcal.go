package app

import (
	"encoding/json"
	"os"
	"path/filepath"

	"radlog/internal/gmc"
)

// ---------- última calibração lida do contador ----------
//
// A calibração vem da configuração do contador ao conectar. Sem guardar,
// com o contador desconectado o app voltava para a de fábrica (0,0065
// µSv/h por CPM) e a dose, o µSv/h e os relatórios saíam com outro fator.
// Agora a última lida fica em calibracao.json e vale até o contador
// conectar de novo.

const calFromLast = "última lida do contador"

func (a *App) lastCalPath() string { return filepath.Join(a.dataDir, "calibracao.json") }

// saveLastCal guarda a calibração (só a que veio do contador).
func (a *App) saveLastCal(c gmc.Calibration) {
	if a.dataDir == "" || len(c.Points) == 0 || c.Source == "padrão" {
		return
	}
	b, err := json.MarshalIndent(c, "", "  ")
	if err != nil {
		return
	}
	tmp := a.lastCalPath() + ".tmp"
	if os.WriteFile(tmp, b, 0o644) == nil {
		_ = os.Rename(tmp, a.lastCalPath())
	}
}

// loadLastCal usa a última calibração guardada enquanto não conecta.
func (a *App) loadLastCal() {
	b, err := os.ReadFile(a.lastCalPath())
	if err != nil {
		return
	}
	var c gmc.Calibration
	if json.Unmarshal(b, &c) != nil || len(c.Points) == 0 {
		return
	}
	c.Source = calFromLast
	a.mu.Lock()
	if a.dev == nil {
		a.cal = c
	}
	a.mu.Unlock()
	a.logf("calibração: usando a última lida do contador (%.4f µSv/h por CPM) até conectar", c.USv(100)/100)
}
