package app

import (
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"path/filepath"
	"time"

	"radlog/internal/gmc"
)

// ---------- calibrar o contador com o fator do app (2.37.0) ----------
//
// O app converte CPM em µSv/h com o fator da intercalibração, mas o GMC-800
// continua com a calibração de fábrica e mostra outro número na tela. Aqui o
// app sugere os 6 pontos com o mesmo fator (o µSv/h de cada ponto fica, o
// CPM é escalado, então a forma da curva continua) e grava no contador.
//
// Toda gravação que muda a calibração guarda duas cópias, só dos pontos:
//   - "fábrica": a que estava no contador na primeira vez que o app mexeu na
//     calibração. Nunca é trocada.
//   - "anterior": a que estava antes da última mudança (se não for a de
//     fábrica). É trocada a cada gravação.
// Voltar a uma delas grava só os 6 pontos; o resto da configuração fica.
//
// No app nada muda: a versão da calibração mantém o fator em uso (o
// calHistDevice já faz isso quando a curva do contador muda).

const counterCalFile = "calibracao-contador.json"

// SavedCal é uma calibração do contador guardada.
type SavedCal struct {
	Points  []gmc.CalPoint `json:"pontos"`
	At      int64          `json:"em"`      // ms: quando foi guardada
	Factory bool           `json:"fabrica"` // igual à de fábrica do GMC-800
}

type counterCalData struct {
	Factory  *SavedCal `json:"fabrica,omitempty"`
	Previous *SavedCal `json:"anterior,omitempty"`
}

func (a *App) counterCalPath() string { return filepath.Join(a.dataDir, counterCalFile) }

func (a *App) loadCounterCal() counterCalData {
	var d counterCalData
	if a.dataDir == "" {
		return d
	}
	a.readStateJSON(a.counterCalPath(), "calibração do contador", &d)
	return d
}

func (a *App) saveCounterCal(d counterCalData) error {
	if a.dataDir == "" {
		return nil
	}
	b, err := json.MarshalIndent(d, "", "  ")
	if err != nil {
		return err
	}
	return writeFileAtomic(a.counterCalPath(), b, 0o644)
}

func isFactoryCal(pts []gmc.CalPoint) bool {
	return samePoints(pts, gmc.DefaultCalibration().Points)
}

// rotateCounterCal guarda a calibração que estava no contador antes de uma
// gravação que a mudou.
func rotateCounterCal(d counterCalData, before []gmc.CalPoint, now time.Time) counterCalData {
	if len(before) == 0 {
		return d
	}
	s := &SavedCal{Points: append([]gmc.CalPoint(nil), before...), At: now.UnixMilli(), Factory: isFactoryCal(before)}
	switch {
	case d.Factory == nil:
		d.Factory = s
	case samePoints(d.Factory.Points, before):
		// voltou da de fábrica: ela já está guardada
	default:
		d.Previous = s
	}
	return d
}

// counterCalChanged é chamado depois de gravar uma configuração que mudou a
// calibração.
func (a *App) counterCalChanged(before []gmc.CalPoint) {
	d := rotateCounterCal(a.loadCounterCal(), before, time.Now())
	if err := a.saveCounterCal(d); err != nil {
		a.logf("calibração do contador: não consegui guardar a cópia (%v)", err)
	}
}

// suggestCounterCal escala o CPM de cada ponto para o contador dar k µSv/h
// por CPM no fundo, mantendo o µSv/h dos pontos.
func suggestCounterCal(pts []gmc.CalPoint, k float64) ([]gmc.CalPoint, error) {
	if len(pts) == 0 || pts[0].CPM <= 0 || pts[0].USv <= 0 {
		return nil, errors.New("a calibração do contador não tem pontos válidos")
	}
	if !(k > 0) {
		return nil, errors.New("o app não tem um fator em uso")
	}
	r := (pts[0].USv / pts[0].CPM) / k
	out := make([]gmc.CalPoint, len(pts))
	for i, p := range pts {
		out[i] = gmc.CalPoint{CPM: math.Round(p.CPM * r), USv: p.USv}
	}
	return out, nil
}

// sameK diz se dois fatores são iguais para a tela (diferença < 0,5%).
func sameK(a, b float64) bool { return a > 0 && b > 0 && math.Abs(a/b-1) < 0.005 }

// CounterCalView é o que o bloco "Calibrar o contador" mostra.
type CounterCalView struct {
	AppK      float64        `json:"appK"`      // µSv/h por CPM no fundo, em uso no app
	DevK      float64        `json:"devK"`      // o que o contador usa
	Source    string         `json:"source"`    // origem da versão em uso no app
	Since     int64          `json:"since"`     // desde quando ela vale (ms)
	Same      bool           `json:"same"`      // contador e app já iguais
	Device    []gmc.CalPoint `json:"device"`    // pontos no contador
	Suggested []gmc.CalPoint `json:"suggested"` // com o fator do app
	Factory   *SavedCal      `json:"factory,omitempty"`
	Previous  *SavedCal      `json:"previous,omitempty"`
}

// GetCounterCal compara a calibração do contador (pts, lidos agora pela
// tela) com o fator em uso no app e devolve os pontos sugeridos.
func (a *App) GetCounterCal(pts []gmc.CalPoint) CounterCalView {
	a.mu.Lock()
	if len(pts) == 0 {
		pts = a.cal.Points
	}
	var src string
	var since int64
	if last := a.calLastLocked(); last != nil {
		src, since = last.Source, last.Since
	}
	a.mu.Unlock()
	d := a.loadCounterCal()
	v := CounterCalView{AppK: a.curK(), Source: src, Since: since, Device: pts, Factory: d.Factory, Previous: d.Previous}
	if len(pts) > 0 && pts[0].CPM > 0 {
		v.DevK = pts[0].USv / pts[0].CPM
	}
	v.Same = sameK(v.AppK, v.DevK)
	if s, err := suggestCounterCal(pts, v.AppK); err == nil {
		v.Suggested = s
	}
	return v
}

// writeCounterPoints lê a configuração do contador, troca só os pontos e
// grava (com a cópia de segurança e a conferência de sempre).
func (a *App) writeCounterPoints(pts func(cur []gmc.CalPoint) ([]gmc.CalPoint, error)) (gmc.Config800, error) {
	cfg, err := a.GetDeviceConfig()
	if err != nil {
		return cfg, err
	}
	if !cfg.CalibrationEditable {
		return cfg, errors.New("a calibração do contador está travada (valores que o app não reconhece)")
	}
	next, err := pts(cfg.Calibration.Points)
	if err != nil {
		return cfg, err
	}
	cfg.Calibration.Points = next
	return a.SetDeviceConfig(cfg)
}

// CalibrateCounter grava no contador os pontos com o fator em uso no app.
func (a *App) CalibrateCounter() (gmc.Config800, error) {
	k := a.curK()
	out, err := a.writeCounterPoints(func(cur []gmc.CalPoint) ([]gmc.CalPoint, error) {
		if len(cur) > 0 && cur[0].CPM > 0 && sameK(cur[0].USv/cur[0].CPM, k) {
			return nil, errors.New("o contador já usa o mesmo fator do app")
		}
		return suggestCounterCal(cur, k)
	})
	if err == nil {
		a.logEvent(fmt.Sprintf("Contador calibrado com o fator do app: %s µSv/h por CPM no fundo", fmtBR(k, 5)))
	}
	return out, err
}

// RestoreCounterCal grava de volta a calibração guardada: "fabrica" ou
// "anterior". Só os 6 pontos mudam.
func (a *App) RestoreCounterCal(which string) (gmc.Config800, error) {
	d := a.loadCounterCal()
	var s *SavedCal
	name := ""
	switch which {
	case "fabrica":
		s, name = d.Factory, "de fábrica"
		if s != nil && !s.Factory {
			name = "original"
		}
	case "anterior":
		s, name = d.Previous, "anterior"
	}
	if s == nil || len(s.Points) == 0 {
		return gmc.Config800{}, errors.New("não há essa calibração guardada")
	}
	out, err := a.writeCounterPoints(func(cur []gmc.CalPoint) ([]gmc.CalPoint, error) {
		if samePoints(cur, s.Points) {
			return nil, errors.New("o contador já está com essa calibração")
		}
		return s.Points, nil
	})
	if err == nil {
		a.logEvent("Contador voltou à calibração " + name)
	}
	return out, err
}
