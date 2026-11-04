package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

// ---------- medir um objeto ----------
//
// Duas fases do mesmo tamanho: primeiro o fundo (objeto longe), depois com o
// objeto encostado no contador. As contagens vêm do CPS de cada segundo. A
// diferença entre as taxas, comparada com a incerteza de Poisson das duas
// (√N de cada contagem), diz se o objeto é mesmo radioativo ou se é ruído.

// MeasurePhase é o resultado de uma fase.
type MeasurePhase struct {
	Counts  int     `json:"counts"`
	Seconds int     `json:"seconds"`
	CPM     float64 `json:"cpm"`
	ErrCPM  float64 `json:"errCPM"` // 1 desvio
	USv     float64 `json:"usv"`
	Start   int64   `json:"start"` // ms
	End     int64   `json:"end"`   // ms
}

// MeasureResult compara as duas fases.
type MeasureResult struct {
	NetCPM  float64 `json:"netCPM"`
	NetErr  float64 `json:"netErr"`
	NetUSv  float64 `json:"netUSv"`
	Ratio   float64 `json:"ratio"` // objeto / fundo
	Sigma   float64 `json:"sigma"`
	Verdict string  `json:"verdict"`
	Level   string  `json:"level"` // nada, talvez, sim
}

// MeasureState é o estado da medição, mandado para a tela a cada segundo.
type MeasureState struct {
	Phase   string         `json:"phase"` // "", fundo, objeto, pronto
	Seconds int            `json:"seconds"`
	Elapsed int            `json:"elapsed"`
	Counts  int            `json:"counts"`
	Name    string         `json:"name"`
	Demo    bool           `json:"demo"`
	Bg      *MeasurePhase  `json:"bg"`
	Obj     *MeasurePhase  `json:"obj"`
	Result  *MeasureResult `json:"result"`
	// quando a fase atual começou (ms) e o ID da medição salva no fim
	PhaseStart int64 `json:"phaseStart"`
	SavedID    int64 `json:"savedId"`
}

// GetMeasure devolve o estado atual.
func (a *App) GetMeasure() MeasureState {
	a.mu.Lock()
	defer a.mu.Unlock()
	return a.measure
}

// StartMeasurePhase começa a fase "fundo" ou "objeto". seconds vale para as
// duas fases (a do objeto usa o mesmo tempo do fundo).
func (a *App) StartMeasurePhase(phase string, seconds int, name string) (MeasureState, error) {
	a.mu.Lock()
	defer a.mu.Unlock()
	connected := a.dev != nil || a.demo.Active
	if !connected {
		return a.measure, errors.New("conecte o contador (ou ligue o modo demo) para medir")
	}
	switch phase {
	case "fundo":
		if seconds < 10 || seconds > 1800 {
			seconds = 60
		}
		a.measure = MeasureState{Phase: "fundo", Seconds: seconds, Name: strings.TrimSpace(name), Demo: a.demo.Active, PhaseStart: time.Now().UnixMilli()}
		a.measSamples = nil
	case "objeto":
		if a.measure.Bg == nil {
			return a.measure, errors.New("meça o fundo primeiro")
		}
		a.measure.Phase = "objeto"
		a.measure.Elapsed, a.measure.Counts = 0, 0
		a.measure.Obj, a.measure.Result = nil, nil
		a.measure.PhaseStart, a.measure.SavedID = time.Now().UnixMilli(), 0
		// medindo o objeto de novo: tira os segundos da tentativa anterior
		kept := a.measSamples[:0]
		for _, s := range a.measSamples {
			if s.Phase == "fundo" {
				kept = append(kept, s)
			}
		}
		a.measSamples = kept
		if n := strings.TrimSpace(name); n != "" {
			a.measure.Name = n
		}
	default:
		return a.measure, fmt.Errorf("fase desconhecida %q", phase)
	}
	st := a.measure
	go a.emit("measure", st)
	return st, nil
}

// CancelMeasure descarta a medição em andamento.
func (a *App) CancelMeasure() MeasureState {
	a.mu.Lock()
	a.measure = MeasureState{}
	a.mu.Unlock()
	a.emit("measure", MeasureState{})
	return MeasureState{}
}

// MeasureSample é um segundo da medição, guardado para rever o gráfico depois.
type MeasureSample struct {
	T     int64   `json:"t"`     // ms desde o início do fundo
	Phase string  `json:"phase"` // fundo ou objeto
	CPM   float64 `json:"cpm"`   // leitura ao vivo do contador (60 s)
	CPS   uint32  `json:"cps"`   // contagens desse segundo
}

// feedMeasure recebe a leitura de um segundo (real ou do demo).
func (a *App) feedMeasure(cpm float64, cps uint32, demo bool) {
	a.mu.Lock()
	m := &a.measure
	if (m.Phase != "fundo" && m.Phase != "objeto") || m.Demo != demo {
		a.mu.Unlock()
		return
	}
	m.Counts += int(cps)
	m.Elapsed++
	start := m.PhaseStart
	if m.Bg != nil && m.Phase == "objeto" {
		start = m.Bg.Start
	}
	a.measSamples = append(a.measSamples, MeasureSample{T: time.Now().UnixMilli() - start, Phase: m.Phase, CPM: cpm, CPS: cps})
	var logText string
	var save *SavedMeasure
	if m.Elapsed >= m.Seconds {
		ph := phaseResult(m.Counts, m.Elapsed, a.cal.USv)
		ph.Start, ph.End = m.PhaseStart, time.Now().UnixMilli()
		if m.Phase == "fundo" {
			m.Bg = &ph
			m.Phase = "fundo-pronto"
		} else {
			m.Obj = &ph
			r := compareMeasure(*m.Bg, ph, a.cal.USv)
			m.Result = &r
			m.Phase = "pronto"
			name := m.Name
			if name == "" {
				name = "objeto"
			}
			logText = fmt.Sprintf("Medição de %s: %s ± %s CPM sobre o fundo (%sσ) — %s",
				name, br(r.NetCPM, true), br(r.NetErr, false), br(r.Sigma, false), r.Verdict)
			if !demo { // o demo não fica salvo
				save = &SavedMeasure{Name: name, Time: m.Bg.Start, Bg: *m.Bg, Obj: ph, Result: r,
					Samples: append([]MeasureSample(nil), a.measSamples...)}
			}
		}
	}
	a.mu.Unlock()
	if save != nil {
		if id, err := a.addMeasurement(*save); err == nil {
			a.mu.Lock()
			a.measure.SavedID = id
			a.mu.Unlock()
		}
	}
	a.mu.Lock()
	st := a.measure
	a.mu.Unlock()
	if logText != "" {
		if demo {
			a.logDemoEvent(logText, kindEvent)
		} else {
			a.logEvent(logText)
		}
	}
	a.emit("measure", st)
}

func phaseResult(counts, seconds int, usv func(float64) float64) MeasurePhase {
	minutes := float64(seconds) / 60
	cpm := float64(counts) / minutes
	n := math.Max(float64(counts), 1) // com 0 contagens, a incerteza ainda é ~1
	return MeasurePhase{
		Counts: counts, Seconds: seconds,
		CPM: cpm, ErrCPM: math.Sqrt(n) / minutes, USv: usv(cpm),
	}
}

func compareMeasure(bg, obj MeasurePhase, usv func(float64) float64) MeasureResult {
	r := MeasureResult{
		NetCPM: obj.CPM - bg.CPM,
		NetErr: math.Sqrt(bg.ErrCPM*bg.ErrCPM + obj.ErrCPM*obj.ErrCPM),
		NetUSv: obj.USv - bg.USv,
	}
	if bg.CPM > 0 {
		r.Ratio = obj.CPM / bg.CPM
	}
	if r.NetErr > 0 {
		r.Sigma = r.NetCPM / r.NetErr
	}
	switch {
	case r.Sigma >= 3:
		r.Level, r.Verdict = "sim", "radioativo: a diferença para o fundo é clara"
	case r.Sigma >= 2:
		r.Level, r.Verdict = "talvez", "provavelmente um pouco radioativo; meça por mais tempo para confirmar"
	default:
		r.Level, r.Verdict = "nada", "nenhuma diferença que dê para separar do fundo"
	}
	return r
}

// br formata com uma casa e vírgula decimal (ex.: +63,0).
func br(v float64, sign bool) string {
	f := "%.1f"
	if sign {
		f = "%+.1f"
	}
	return strings.Replace(fmt.Sprintf(f, v), ".", ",", 1)
}

// ---------- medições salvas ----------

// SavedMeasure é uma medição de objeto terminada, guardada em medicoes.json.
type SavedMeasure struct {
	ID     int64         `json:"id"`
	Name   string        `json:"name"`
	Time   int64         `json:"time"` // início do fundo (ms)
	Bg     MeasurePhase  `json:"bg"`
	Obj    MeasurePhase  `json:"obj"`
	Result MeasureResult `json:"result"`
	Note   string        `json:"note"`
	// um registro por segundo (medições a partir da 0.20.3)
	Samples []MeasureSample `json:"samples,omitempty"`
}

func (a *App) measurementsPath() string { return filepath.Join(a.dataDir, "medicoes.json") }

func (a *App) loadMeasurements() {
	b, err := os.ReadFile(a.measurementsPath())
	if err != nil {
		return
	}
	var list []SavedMeasure
	if json.Unmarshal(b, &list) == nil {
		a.measurements = list
	}
}

func (a *App) saveMeasurementsLocked() error {
	if a.dataDir == "" {
		return nil
	}
	b, _ := json.MarshalIndent(a.measurements, "", "  ")
	tmp := a.measurementsPath() + ".tmp"
	if err := os.WriteFile(tmp, b, 0o644); err != nil {
		return err
	}
	return os.Rename(tmp, a.measurementsPath())
}

func (a *App) addMeasurement(m SavedMeasure) (int64, error) {
	a.mu.Lock()
	var max int64
	for _, q := range a.measurements {
		if q.ID > max {
			max = q.ID
		}
	}
	m.ID = max + 1
	a.measurements = append(a.measurements, m)
	sort.Slice(a.measurements, func(i, j int) bool { return a.measurements[i].Time < a.measurements[j].Time })
	a.bgAt = time.Time{} // os minutos com o objeto saem do fundo
	err := a.saveMeasurementsLocked()
	a.mu.Unlock()
	a.emit("measurements", a.GetMeasurements())
	return m.ID, err
}

// GetMeasurements devolve as medições salvas, da mais antiga à mais nova.
func (a *App) GetMeasurements() []SavedMeasure {
	a.mu.Lock()
	defer a.mu.Unlock()
	return append([]SavedMeasure{}, a.measurements...)
}

// UpdateMeasurement troca o nome e a anotação de uma medição salva.
func (a *App) UpdateMeasurement(id int64, name, note string) error {
	a.mu.Lock()
	found := false
	for i := range a.measurements {
		if a.measurements[i].ID == id {
			if n := strings.TrimSpace(name); n != "" {
				a.measurements[i].Name = n
			}
			a.measurements[i].Note = strings.TrimSpace(note)
			found = true
		}
	}
	var err error
	if !found {
		err = errors.New("medição não encontrada")
	} else {
		err = a.saveMeasurementsLocked()
	}
	a.mu.Unlock()
	a.emit("measurements", a.GetMeasurements())
	return err
}

// DeleteMeasurement apaga uma medição salva.
func (a *App) DeleteMeasurement(id int64) error {
	a.mu.Lock()
	kept := a.measurements[:0]
	for _, q := range a.measurements {
		if q.ID != id {
			kept = append(kept, q)
		}
	}
	a.measurements = kept
	a.bgAt = time.Time{}
	err := a.saveMeasurementsLocked()
	a.mu.Unlock()
	a.emit("measurements", a.GetMeasurements())
	return err
}

// measuringChecker diz se um instante caiu na fase "com o objeto" de uma
// medição salva (esses minutos não são o fundo da casa).
func (a *App) measuringChecker() func(time.Time) bool {
	a.mu.Lock()
	var spans [][2]int64
	for _, m := range a.measurements {
		// o minuto inteiro em volta, porque o CPM gravado é de 60 s
		spans = append(spans, [2]int64{m.Obj.Start - 60_000, m.Obj.End + 60_000})
	}
	a.mu.Unlock()
	return func(t time.Time) bool {
		ms := t.UnixMilli()
		for _, s := range spans {
			if ms >= s[0] && ms < s[1] {
				return true
			}
		}
		return false
	}
}
