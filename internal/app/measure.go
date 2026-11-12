package app

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

	"radlog/internal/radiacode"
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
	// "casa" = fundo da última hora gravada (não medido na hora)
	Source string `json:"source,omitempty"`
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
	// como parou: achou, nada, limite (automático) ou "" (tempo fixo)
	Stop string `json:"stop,omitempty"`
	// limite superior do líquido (CPM, ~97,5%) quando não deu "sim"
	Limit float64 `json:"limit,omitempty"`
	// automático: o que contava como "tem algo" (% do fundo e em CPM)
	Pct       float64 `json:"pct,omitempty"`
	Threshold float64 `json:"threshold,omitempty"`
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
	// relatório em PDF gerado sozinho no fim (caminho; "" = não gerou)
	ReportPath  string `json:"reportPath"`
	ReportError string `json:"reportError"`
	// automático: para sozinho quando der resposta (até MaxSec na fase do
	// objeto); Live é o resultado parcial e Eta, os segundos que faltam (-1 = ?)
	Auto     bool           `json:"auto"`
	MaxSec   int            `json:"maxSec"`
	Pct      float64        `json:"pct"`
	Distance string         `json:"distance"`
	Live     *MeasureResult `json:"live"`
	Eta      int            `json:"eta"`
	// aparelho que está medindo: "gmc" (ou "") e "radiacode"
	Device string `json:"device"`
	// espectro (Radiacode), quando a medição termina
	Spectrum *MeasureSpectrum `json:"spectrum,omitempty"`
	// com os dois aparelhos: "ambos" (os dois perto) ou "voce" (um com a
	// pessoa); Other é o segundo aparelho
	Mode  string       `json:"mode,omitempty"`
	Other *MeasureSide `json:"other,omitempty"`
	// 2.40: distância em cm (0 = encostado ou não informada), tamanho do
	// objeto (pequeno, espalhado), as medições anteriores do mesmo objeto em
	// outras distâncias e as contas do resultado
	DistCM  float64         `json:"distCm"`
	Size    string          `json:"size,omitempty"`
	Prev    []MeasureAt     `json:"prev,omitempty"`
	Derived *MeasureDerived `json:"derived,omitempty"`
	// 2.43 (ALARA): teste de distância ou de blindagem; a fase do objeto para
	// quando a incerteza da taxa líquida fica abaixo de RelErr (ou em Seconds)
	Test     string  `json:"test,omitempty"` // distancia, blindagem
	Material string  `json:"material,omitempty"`
	RelErr   float64 `json:"relErr,omitempty"`
	// Radiacode: soma do µSv/h do aparelho a cada segundo da fase atual
	usvSum float64
	// pediu para parar a fase do objeto agora (botão "Parar e ver o resultado")
	stopReq bool
}

// MeasureSpectrum são os espectros do Radiacode de cada fase (contagens por
// canal; energia = A0 + A1·canal + A2·canal²).
type MeasureSpectrum struct {
	A      [3]float64 `json:"a"`
	Bg     []uint32   `json:"bg,omitempty"`
	BgSec  float64    `json:"bgSec"`
	BgSrc  string     `json:"bgSrc,omitempty"` // "acumulado" = espectro acumulado do aparelho (fundo da casa)
	Obj    []uint32   `json:"obj"`
	ObjSec float64    `json:"objSec"`
	// linhas gama que subiram acima do fundo (pista do radionuclídeo)
	Peaks []SpecPeak `json:"peaks,omitempty"`
	// "voce" = espectro do RC102 que estava com a pessoa (não no objeto)
	Role string `json:"role,omitempty"`
}

// devName é o nome do aparelho da medição ("" conta como GMC).
func devName(d string) string {
	if d == "radiacode" {
		return d
	}
	return "gmc"
}

// GetMeasure devolve o estado atual.
func (a *App) GetMeasure() MeasureState {
	a.mu.Lock()
	defer a.mu.Unlock()
	return a.measureSnapLocked()
}

// StartMeasurePhase começa a fase "fundo" ou "objeto". seconds vale para as
// duas fases (a do objeto usa o mesmo tempo do fundo).
func (a *App) StartMeasurePhase(phase string, seconds int, name string) (MeasureState, error) {
	a.mu.Lock()
	dev := devName(a.measure.Device)
	rcIn := measRcInvolved(&a.measure)
	a.mu.Unlock()
	var spec0 *radiacode.Spectrum
	if rcIn {
		if spec0 = a.rcSpectrumNow(false); spec0 == nil && !a.rcConnected() {
			return a.GetMeasure(), errors.New("conecte o RC102 para medir")
		}
	}
	a.mu.Lock()
	defer a.mu.Unlock()
	if dev == "gmc" && a.dev == nil && !a.demo.Active {
		return a.measure, errors.New("conecte o contador (ou ligue o modo demo) para medir")
	}
	a.measSpec0 = spec0
	switch phase {
	case "fundo":
		if seconds < 10 || seconds > 1800 {
			seconds = 60
		}
		prev := a.measure
		a.measure = MeasureState{Phase: "fundo", Seconds: seconds, Name: strings.TrimSpace(name), Demo: a.demo.Active && dev == "gmc", PhaseStart: time.Now().UnixMilli(), Eta: -1, Device: dev, Mode: prev.Mode}
		if o := prev.Other; o != nil {
			a.measure.Other = &MeasureSide{Device: o.Device, Role: o.Role, BgUSv: o.BgUSv}
		}
		a.measSamples, a.measSamples2 = nil, nil
		a.measSpecBg = nil
	case "objeto":
		if a.measure.Bg == nil {
			return a.measure, errors.New("meça o fundo primeiro")
		}
		a.measure.Phase = "objeto"
		a.measure.Elapsed, a.measure.Counts = 0, 0
		a.measure.Obj, a.measure.Result, a.measure.Live, a.measure.Eta = nil, nil, nil, -1
		a.measure.Spectrum, a.measure.usvSum = nil, 0
		a.measure.Derived, a.measure.stopReq = nil, false
		startSideObjLocked(&a.measure)
		if a.measure.Auto {
			a.measure.Seconds = a.measure.MaxSec
		}
		a.measure.PhaseStart, a.measure.SavedID = time.Now().UnixMilli(), 0
		// medindo o objeto de novo: tira os segundos da tentativa anterior
		kept := a.measSamples[:0]
		for _, s := range a.measSamples {
			if s.Phase == "fundo" {
				kept = append(kept, s)
			}
		}
		a.measSamples = kept
		kept2 := a.measSamples2[:0]
		for _, s := range a.measSamples2 {
			if s.Phase == "fundo" {
				kept2 = append(kept2, s)
			}
		}
		a.measSamples2 = kept2
		if n := strings.TrimSpace(name); n != "" {
			a.measure.Name = n
		}
	default:
		return a.measure, fmt.Errorf("fase desconhecida %q", phase)
	}
	st := a.measureSnapLocked()
	go a.emit("measure", st)
	return st, nil
}

// measStopMinSec: antes disso o "parar agora" espera (poucas contagens).
const measStopMinSec = 5

// measRcInvolved diz se o RC102 participa da medição (no objeto ou com a
// pessoa): aí o app guarda o espectro de cada fase.
func measRcInvolved(m *MeasureState) bool {
	return devName(m.Device) == devRC || (m.Other != nil && m.Other.Device == devRC)
}

// StopMeasure termina a fase do objeto agora (com pelo menos alguns
// segundos), com o resultado do que já juntou.
func (a *App) StopMeasure() MeasureState {
	a.mu.Lock()
	if a.measure.Phase == "objeto" {
		a.measure.stopReq = true
	}
	st := a.measureSnapLocked()
	a.mu.Unlock()
	return st
}

// MeasureAgainAt mede o objeto de novo em outra distância, com o mesmo
// fundo: a medição que terminou fica em Prev para a conta do inverso do
// quadrado.
func (a *App) MeasureAgainAt(distance string, distCM float64) (MeasureState, error) {
	a.mu.Lock()
	m := &a.measure
	if m.Phase != "pronto" || m.Result == nil || m.Bg == nil || m.Obj == nil {
		st := a.measureSnapLocked()
		a.mu.Unlock()
		return st, errors.New("termine a medição antes de medir em outra distância")
	}
	k := 0.0
	if m.Obj.CPM > 0 {
		k = m.Obj.USv / m.Obj.CPM
	}
	m.Prev = append(m.Prev, MeasureAt{Distance: m.Distance, DistCM: m.DistCM, NetUSv: m.Result.NetUSv, ErrUSv: m.Result.NetErr * k,
		Sigma: m.Result.Sigma, Level: m.Result.Level, Seconds: m.Obj.Seconds})
	m.Distance = strings.TrimSpace(distance)
	if distCM < 0 {
		distCM = 0
	}
	m.DistCM = distCM
	a.mu.Unlock()
	return a.StartMeasurePhase("objeto", 0, "")
}

// MeasureLive é o espectro que está se formando na fase atual (RC102).
type MeasureLive struct {
	Phase    string           `json:"phase"`
	Spectrum *MeasureSpectrum `json:"spectrum,omitempty"`
}

// GetMeasureLive lê o espectro do aparelho e devolve o que entrou desde o
// começo da fase (na fase do objeto, com o fundo e os picos).
func (a *App) GetMeasureLive() MeasureLive {
	a.mu.Lock()
	m := a.measure
	spec0, bg := a.measSpec0, a.measSpecBg
	a.mu.Unlock()
	out := MeasureLive{Phase: m.Phase}
	if (m.Phase != "fundo" && m.Phase != "objeto") || !measRcInvolved(&m) || spec0 == nil {
		return out
	}
	part := specDiff(spec0, a.rcSpectrumNow(false))
	if part == nil {
		return out
	}
	if devName(m.Device) != devRC {
		part.Role = "voce"
	}
	if m.Phase == "objeto" && bg != nil && len(bg.Bg) == len(part.Obj) {
		part.Bg, part.BgSec, part.BgSrc = bg.Bg, bg.BgSec, bg.BgSrc
		if part.ObjSec >= 5 {
			part.Peaks = part.findPeaks()
		}
	}
	out.Spectrum = part
	return out
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
	// µSv/h desse segundo (segundo aparelho, para o gráfico)
	USv float64 `json:"usv,omitempty"`
}

// measT0 é o zero do tempo dos segundos guardados (começo do fundo medido,
// ou do objeto quando o fundo é o da casa).
func measT0(m *MeasureState) int64 {
	if m.Bg != nil && m.Phase == "objeto" && m.Bg.Source != "casa" {
		return m.Bg.Start
	}
	return m.PhaseStart
}

// feedMeasure recebe a leitura de um segundo do GMC (real ou do demo).
func (a *App) feedMeasure(cpm float64, cps uint32, demo bool) {
	a.feedMeasureDev(measFeed{dev: "gmc", cpm: cpm, counts: int(cps), demo: demo})
}

// measFeed é um segundo de leitura de um aparelho.
type measFeed struct {
	dev    string  // gmc ou radiacode
	cpm    float64 // leitura ao vivo (para o gráfico)
	counts int     // contagens desse segundo
	usv    float64 // RC102: µSv/h do aparelho nesse segundo
	demo   bool
}

// measPhaseLocked é o resultado da fase atual até agora. No GMC o µSv/h vem
// da calibração; no Radiacode, da média do que o próprio aparelho calculou.
func (a *App) measPhaseLocked(m *MeasureState) MeasurePhase {
	ph := phaseResult(m.Counts, m.Elapsed, a.convCalLocked().USv)
	if devName(m.Device) == "radiacode" && m.Elapsed > 0 {
		ph.USv = m.usvSum / float64(m.Elapsed)
	}
	return ph
}

func (a *App) feedMeasureDev(f measFeed) {
	a.mu.Lock()
	m := &a.measure
	if a.feedSideLocked(m, f) {
		a.mu.Unlock()
		return
	}
	if (m.Phase != "fundo" && m.Phase != "objeto") || m.Demo != f.demo || devName(m.Device) != f.dev {
		a.mu.Unlock()
		return
	}
	m.Counts += f.counts
	m.usvSum += f.usv
	m.Elapsed++
	start := measT0(m)
	a.measSamples = append(a.measSamples, MeasureSample{T: time.Now().UnixMilli() - start, Phase: m.Phase, CPM: f.cpm, CPS: uint32(max(f.counts, 0))})
	var logText string
	var save *SavedMeasure
	stop, ended := "", ""
	if m.Phase == "objeto" {
		part := a.measPhaseLocked(m)
		live := compareMeasure(*m.Bg, part, a.convCalLocked().USv)
		m.Live = &live
		if m.stopReq && m.Elapsed >= measStopMinSec {
			stop = "manual"
		} else if m.RelErr > 0 && m.Elapsed >= relErrMinSec && live.NetCPM > 0 && live.NetErr <= m.RelErr*live.NetCPM {
			stop = "precisao" // teste do ALARA: a incerteza já ficou pequena
		} else if m.Auto {
			stop = autoDecide(live, *m.Bg, m.Elapsed, m.Pct)
			// os dois perto: só para quando os dois já tiverem resposta
			if stop != "" && sideDecide(m) == "" {
				stop = ""
			}
			m.Eta = autoEta(*m.Bg, part, m.Elapsed, m.MaxSec, m.Pct)
			if stop == "" && m.Elapsed >= m.Seconds {
				stop = "limite"
			}
		}
	}
	if m.Elapsed >= m.Seconds || stop != "" {
		ph := a.measPhaseLocked(m)
		ph.Start, ph.End = m.PhaseStart, time.Now().UnixMilli()
		ended = m.Phase
		a.endSidePhaseLocked(m, m.Phase, ph.Start, ph.End)
		if m.Phase == "fundo" {
			m.Bg = &ph
			m.Phase = "fundo-pronto"
		} else {
			m.Obj = &ph
			r := compareMeasure(*m.Bg, ph, a.convCalLocked().USv)
			finishResult(&r, stop, *m.Bg, m.Pct, m.Auto)
			m.Result, m.Live, m.Eta, m.stopReq = &r, nil, -1, false
			a.finishSideLocked(m)
			m.Phase = "pronto"
			name := m.Name
			if name == "" {
				name = "objeto"
			}
			via := ""
			if f.dev == "radiacode" {
				via = " (RC102)"
			}
			k := 0.0
			if ph.CPM > 0 {
				k = ph.USv / ph.CPM
			}
			sign := "+"
			if r.NetUSv < 0 {
				sign = "−"
			}
			logText = fmt.Sprintf("Medição de %s%s: %s%s ± %s µSv/h sobre o fundo (%sσ) — %s",
				name, via, sign, usvTxt(math.Abs(r.NetUSv)), usvTxt(r.NetErr*k), br(r.Sigma, false), r.Verdict)
			if !f.demo { // o demo não fica salvo
				when := m.Bg.Start
				if m.Bg.Source == "casa" {
					when = ph.Start
				}
				save = &SavedMeasure{Name: name, Time: when, Bg: *m.Bg, Obj: ph, Result: r, Distance: m.Distance,
					Samples: append([]MeasureSample(nil), a.measSamples...),
					DistCM:  m.DistCM, Size: m.Size, Prev: append([]MeasureAt(nil), m.Prev...),
					Test: m.Test, Material: m.Material}
				if f.dev == "radiacode" {
					save.Device = "radiacode"
				}
				save.Mode, save.Other = m.Mode, m.Other.clone()
				if save.Other != nil {
					save.Other.Samples = append([]MeasureSample(nil), a.measSamples2...)
				}
			}
		}
	}
	rcIn := measRcInvolved(m)
	a.mu.Unlock()
	if ended != "" && rcIn {
		// espectro da fase = o do aparelho agora menos o do começo da fase
		sp := a.rcSpectrumNow(false)
		a.mu.Lock()
		part := specDiff(a.measSpec0, sp)
		if ended == "fundo" {
			a.measSpecBg = nil
			if part != nil {
				a.measSpecBg = &MeasureSpectrum{A: part.A, Bg: part.Obj, BgSec: part.ObjSec}
			}
		} else if part != nil {
			ms := *part
			if b := a.measSpecBg; b != nil {
				ms.Bg, ms.BgSec, ms.BgSrc = b.Bg, b.BgSec, b.BgSrc
			}
			ms.Peaks = ms.findPeaks()
			if devName(a.measure.Device) != devRC {
				ms.Role = "voce" // o RC102 estava com a pessoa
			}
			a.measure.Spectrum = &ms
			if save != nil {
				save.Spectrum = &ms
			}
		}
		a.measSpec0 = nil
		a.mu.Unlock()
	}
	if ended == "objeto" {
		a.mu.Lock()
		if m := &a.measure; m.Result != nil && m.Bg != nil && m.Obj != nil {
			m.Derived = deriveMeasure(deriveIn{device: m.Device, mode: m.Mode, bg: *m.Bg, obj: *m.Obj, r: *m.Result,
				spec: m.Spectrum, distCM: m.DistCM, size: m.Size, prev: m.Prev, test: m.Test})
			if save != nil {
				save.Derived = m.Derived
			}
		}
		a.mu.Unlock()
	}
	if save != nil {
		if id, err := a.addMeasurement(*save); err == nil {
			a.mu.Lock()
			a.measure.SavedID = id
			auto := !a.settings.NoMeasureReport
			a.mu.Unlock()
			if auto {
				a.goSafe("autoMeasureReport", func() { a.autoMeasureReport(id) })
			}
		}
	}
	a.mu.Lock()
	st := a.measureSnapLocked()
	a.mu.Unlock()
	if logText != "" {
		if f.demo {
			a.logDemoEvent(logText, kindEvent)
		} else {
			a.logEvent(logText)
		}
	}
	a.emit("measure", st)
}

// specDiff é o espectro acumulado entre duas leituras do aparelho (se ele
// foi zerado no meio, vale a segunda inteira).
func specDiff(s0, s1 *radiacode.Spectrum) *MeasureSpectrum {
	if s1 == nil || len(s1.Counts) == 0 {
		return nil
	}
	out := &MeasureSpectrum{A: [3]float64{s1.A0, s1.A1, s1.A2}, Obj: make([]uint32, len(s1.Counts)), ObjSec: s1.Duration.Seconds()}
	copy(out.Obj, s1.Counts)
	if s0 == nil || len(s0.Counts) != len(s1.Counts) || s1.Duration < s0.Duration {
		return out
	}
	for i, c := range s0.Counts {
		if out.Obj[i] < c { // zerou no meio
			copy(out.Obj, s1.Counts)
			return out
		}
		out.Obj[i] -= c
	}
	out.ObjSec = (s1.Duration - s0.Duration).Seconds()
	return out
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
	// distância do objeto ao tubo, como foi escrita (ex.: encostado, 5 cm)
	Distance string `json:"distance,omitempty"`
	// um registro por segundo (medições a partir da 0.20.3)
	Samples []MeasureSample `json:"samples,omitempty"`
	// aparelho que mediu ("" = GMC) e os espectros (Radiacode)
	Device   string           `json:"device,omitempty"`
	Spectrum *MeasureSpectrum `json:"spectrum,omitempty"`
	// com os dois aparelhos (ver measuredual.go)
	Mode  string       `json:"mode,omitempty"`
	Other *MeasureSide `json:"other,omitempty"`
	// 2.7: marcada à mão como não comparável com as outras do mesmo objeto
	// (outra geometria, outro recipiente…) e anexos (fotos, documentos)
	NotComparable bool                `json:"notComparable,omitempty"`
	NCReason      string              `json:"ncReason,omitempty"`
	Attachments   []MeasureAttachment `json:"attachments,omitempty"`
	// 2.40: ver MeasureState
	DistCM  float64         `json:"distCm,omitempty"`
	Size    string          `json:"size,omitempty"`
	Prev    []MeasureAt     `json:"prev,omitempty"`
	Derived *MeasureDerived `json:"derived,omitempty"`
	// 2.43: teste do ALARA (distancia, blindagem) e o material da blindagem
	Test     string `json:"test,omitempty"`
	Material string `json:"material,omitempty"`
}

func (a *App) measurementsPath() string { return filepath.Join(a.dataDir, "medicoes.json") }

func (a *App) loadMeasurements() {
	var list []SavedMeasure
	if a.readStateJSON(a.measurementsPath(), "medições", &list) {
		a.measurements = list
	}
}

func (a *App) saveMeasurementsLocked() error {
	if a.dataDir == "" {
		return nil
	}
	b, _ := json.MarshalIndent(a.measurements, "", "  ")
	return writeFileAtomic(a.measurementsPath(), b, 0o644)
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
	if a.dataDir != "" {
		_ = os.RemoveAll(a.attachDir(id))
	}
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

// onObjectChecker diz se o aparelho dev estava encostado num objeto: da
// fase do fundo ao fim da última distância de uma medição (ou teste do
// ALARA) em que ele era o aparelho principal ou o segundo "no objeto". O
// aparelho "com você" do modo um-no-objeto-outro-com-você não conta.
func (a *App) onObjectChecker(dev string) func(time.Time) bool {
	dev = devName(dev)
	a.mu.Lock()
	var spans [][2]int64
	add := func(m SavedMeasure) {
		on := devName(m.Device) == dev
		if o := m.Other; o != nil && o.Role == "objeto" && devName(o.Device) == dev {
			on = true
		}
		if on && m.Obj.End > 0 {
			from := m.Time
			if from == 0 || from > m.Obj.Start {
				from = m.Obj.Start
			}
			spans = append(spans, [2]int64{from - 60_000, m.Obj.End + 60_000})
		}
	}
	for _, m := range a.measurements {
		add(m)
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

// autoMeasureReport gera o PDF da medição que acabou de terminar (sem abrir:
// a tela mostra "Abrir relatório").
func (a *App) autoMeasureReport(id int64) {
	path, err := a.saveMeasureReport(id)
	a.mu.Lock()
	if a.measure.SavedID == id {
		a.measure.ReportPath = path
		if err != nil {
			a.measure.ReportError = err.Error()
		}
	}
	st := a.measureSnapLocked()
	a.mu.Unlock()
	if err != nil {
		a.logf("relatório da medição %d: %v", id, err)
	}
	a.emit("measure", st)
}
