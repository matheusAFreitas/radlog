package app

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"

	"radlog/internal/core/stats"
	"radlog/internal/radiacode"
)

// ---------- atividade (Bq) com o Radiacode: medição ----------
//
// Uma medição tem duas fases: o fundo (nada perto) e a amostra (na posição
// combinada). Em cada fase o app guarda o espectro do aparelho no começo e vai
// somando o que chega (se o espectro do aparelho for zerado no meio, junta o
// que já tinha). O fundo fica salvo para reaproveitar nas próximas.
//
// radiacode/atividade/: calibracao.json, fundo.json, medidas.jsonl e
// atual.json (a medição em andamento, para continuar se o app fechar).

const (
	actDir         = "atividade"
	actCalFile     = "calibracao.json"
	actBgFile      = "fundo.json"
	actResFile     = "medidas.jsonl"
	actCurFile     = "atual.json"
	actTickUSB     = 5 * time.Second
	actWeakSec     = 1200    // a partir de quando avisar que o RC102 não está vendo o sal (20 min: com +10% de subida, ~8% de alarme falso)
	actEffMin      = 0.00001 // 0,001%: abaixo disso o RC102 não está vendo o sal (2.101.11: era 0,03%)
	actWeakPeakSec = 1800    // e a partir de quando o pico sozinho decide
	actWeakRise    = 1.05    // a contagem de 300 a 1.400 keV com o sal tem que subir mais que isso
	actWeakLo      = 300.0   // faixa do espalhamento do K-40 (keV): o sal em volta do
	actWeakHi      = 1400.0  // aparelho blinda o fundo de baixa energia, então o total não serve
	actBgHighRise  = 1.25    // fundo acima disso (× o normal da casa): o sal está perto
	actCalMaxErr   = 0.20    // incerteza máxima da eficiência do KCl para virar calibração
	actTickBLE     = 15 * time.Second
	actMaxHist     = 20
	actBgMaxAge    = 30 * 24 * time.Hour
)

// ActOpts é o que a pessoa escolhe para medir.
type ActOpts struct {
	Kind     string   `json:"kind"` // "kcl" | "torio" | "amostra"
	Name     string   `json:"name"`
	MassG    float64  `json:"massG"`
	Pct      float64  `json:"pct"`  // % de KCl (kcl) ou de ThO₂ (tório)
	AgeY     float64  `json:"ageY"` // idade do eletrodo (0 = não sei)
	Nuclides []string `json:"nuclides"`
	Geometry string   `json:"geometry"`
	NewBg    bool     `json:"newBg"`
	BgSec    int      `json:"bgSec"`  // 0 = até parar
	SmpSec   int      `json:"smpSec"` // 0 = até parar
	// pedra solta (forma diferente do pote): massa opcional, resultado em Bq da peça
	Loose    bool   `json:"loose,omitempty"`
	SampleID string `json:"sampleId,omitempty"` // de "Suas amostras"
	// 2.101.12: o nome foi escrito pela pessoa (a tela diz). Os nomes que o app
	// dá (cartão pronto, amostra que vem com o app, calibração, "amostra")
	// traduzem nos Eventos; o da pessoa fica como está.
	NameOwn bool `json:"nameOwn,omitempty"`
}

// actSaved é um espectro somado guardado.
type actSaved struct {
	From int64      `json:"from"`
	T    int64      `json:"t"`
	Sec  float64    `json:"sec"`
	A    [3]float64 `json:"a"`
	C    []float64  `json:"c"`
}

func (s *actSaved) spec() actSpec {
	if s == nil {
		return actSpec{}
	}
	return actSpec{C: s.C, Sec: s.Sec, A: s.A}
}

// actAcc soma o espectro do aparelho desde o começo da fase.
type actAcc struct {
	From    int64      `json:"from"`
	Base    []float64  `json:"base"`
	BaseSec float64    `json:"baseSec"`
	Last    []float64  `json:"last"`
	LastSec float64    `json:"lastSec"`
	Acc     []float64  `json:"acc"`
	AccSec  float64    `json:"accSec"`
	A       [3]float64 `json:"a"`
}

func f64s(c []uint32) []float64 {
	out := make([]float64, len(c))
	for i, v := range c {
		out[i] = float64(v)
	}
	return out
}

func (c *actAcc) feed(s radiacode.Spectrum) {
	cur, sec := f64s(s.Counts), s.Duration.Seconds()
	c.A = [3]float64{s.A0, s.A1, s.A2}
	if c.Base == nil || len(c.Base) != len(cur) {
		c.Base, c.BaseSec, c.Last, c.LastSec = cur, sec, cur, sec
		c.Acc, c.AccSec = make([]float64, len(cur)), 0
		return
	}
	reset := sec < c.LastSec
	for i := range cur {
		if cur[i] < c.Last[i] {
			reset = true
			break
		}
	}
	if reset { // zerou no aparelho: guarda o que tinha e recomeça do zero
		for i := range c.Acc {
			c.Acc[i] += c.Last[i] - c.Base[i]
		}
		c.AccSec += c.LastSec - c.BaseSec
		c.Base, c.BaseSec = make([]float64, len(cur)), 0
	}
	c.Last, c.LastSec = cur, sec
}

func (c *actAcc) sec() float64 { return c.AccSec + c.LastSec - c.BaseSec }

func (c *actAcc) saved(now time.Time) *actSaved {
	out := &actSaved{From: c.From, T: now.UnixMilli(), Sec: c.sec(), A: c.A, C: make([]float64, len(c.Last))}
	for i := range out.C {
		out.C[i] = c.Acc[i] + c.Last[i] - c.Base[i]
	}
	return out
}

// actRun é a medição em andamento.
type actRun struct {
	Opts    ActOpts    `json:"opts"`
	Phase   string     `json:"phase"` // fundo | trocar | amostra | pronto
	PhaseAt int64      `json:"phaseAt"`
	Cur     actAcc     `json:"cur"`
	Bg      *actSaved  `json:"bg"`  // fundo desta medição (novo ou o salvo)
	Smp     *actSaved  `json:"smp"` // amostra, ao terminar
	Result  *ActResult `json:"result"`
}

type actState struct {
	mu     sync.Mutex
	run    *actRun
	loaded bool
	tickAt time.Time
	saveAt time.Time
	// contagem normal da casa (cache para o aviso do fundo alto)
	homeCps    float64
	homeAt     time.Time
	homeBefore int64
}

// ActResult é uma medição terminada.
type ActResult struct {
	T        int64        `json:"t"`
	Kind     string       `json:"kind"`
	Name     string       `json:"name"`
	MassG    float64      `json:"massG"`
	Pct      float64      `json:"pct,omitempty"`
	AgeY     float64      `json:"ageY,omitempty"`
	Geometry string       `json:"geometry"`
	Sec      float64      `json:"sec"`
	BgSec    float64      `json:"bgSec"`
	CalSrc   string       `json:"calSrc"` // de onde veio a eficiência: kcl | torio | chute
	Nuclides []ActNucRes  `json:"nuclides"`
	Expected float64      `json:"expected,omitempty"` // Bq esperados (referência)
	KCl      *ActCalKCl   `json:"kcl,omitempty"`
	Th       *ActCalTh    `json:"th,omitempty"`
	ThLines  []ActLineRes `json:"thLines,omitempty"`
	Prev     float64      `json:"prev,omitempty"` // eficiência anterior em 1.460 keV (para comparar)
	Loose    bool         `json:"loose,omitempty"`
	SampleID string       `json:"sampleId,omitempty"`
	Nucs     []string     `json:"nucs,omitempty"`     // o que foi procurado
	SmpSecO  int          `json:"smpSecO,omitempty"`  // tempo escolhido (para medir de novo)
	NotSaved bool         `json:"notSaved,omitempty"` // calibração incerta demais: não foi guardada
	// 2.101.14 (item 35): saiu pior que a que está em uso, então não foi
	// trocada sozinha; PrevErr é o ± da atual (KCl: relativo; tório: da
	// inclinação) e Used diz que a pessoa escolheu usar a nova mesmo assim
	Worse   bool    `json:"worse,omitempty"`
	PrevErr float64 `json:"prevErr,omitempty"`
	Used    bool    `json:"used,omitempty"`
}

func (a *App) actPath(name string) string { return filepath.Join(a.rcDir(), actDir, name) }

func (a *App) actWriteJSON(name string, v any) {
	b, err := json.Marshal(v)
	if err != nil {
		return
	}
	os.MkdirAll(a.actPath(""), 0o755)
	_ = writeFileAtomic(a.actPath(name), b, 0o644)
}

func (a *App) actCal() ActCal {
	var c ActCal
	if b, err := os.ReadFile(a.actPath(actCalFile)); err == nil {
		json.Unmarshal(b, &c)
	}
	if c.Geometry == "" {
		c.Geometry = "pote encostado no RC102"
	}
	return c
}

func (a *App) actSavedBg() *actSaved {
	b, err := os.ReadFile(a.actPath(actBgFile))
	if err != nil {
		return nil
	}
	var s actSaved
	if json.Unmarshal(b, &s) != nil || len(s.C) == 0 || s.Sec <= 0 {
		return nil
	}
	return &s
}

// actLoadLocked lê a medição em andamento (se o app fechou no meio).
func (a *App) actLoadLocked() {
	st := &a.rc.act
	if st.loaded {
		return
	}
	st.loaded = true
	if b, err := os.ReadFile(a.actPath(actCurFile)); err == nil {
		var r actRun
		if json.Unmarshal(b, &r) == nil && r.Phase != "" {
			st.run = &r
		}
	}
}

func (a *App) actSaveLocked() {
	st := &a.rc.act
	if st.run == nil || st.run.Phase == "pronto" {
		os.Remove(a.actPath(actCurFile))
		return
	}
	a.actWriteJSON(actCurFile, st.run)
	st.saveAt = time.Now()
}

// actEvName é o nome nos Eventos: o da pessoa fica como está (asIs); os que o
// app dá traduzem (2.101.12).
func actEvName(o ActOpts) string {
	if o.NameOwn {
		return asIs(o.Name)
	}
	return o.Name
}

func actValidate(o *ActOpts) error {
	o.Name = strings.TrimSpace(o.Name)
	o.Geometry = strings.TrimSpace(o.Geometry)
	switch o.Kind {
	case "kcl":
		if o.Pct <= 0 || o.Pct > 100 {
			return errors.New("informe a % de KCl (100 para KCl puro)")
		}
		o.Nuclides = []string{"K-40"}
		if o.Name == "" {
			o.Name = fmt.Sprintf("KCl (%g%%)", o.Pct)
		}
		o.NameOwn = false
	case "torio":
		if o.Pct <= 0 || o.Pct > 10 {
			return errors.New("informe a % de ThO₂ do eletrodo (2 no WT20)")
		}
		o.Nuclides = []string{"Th-232"}
		if o.Name == "" {
			o.Name = fmt.Sprintf("eletrodo de tório (%g%% ThO₂)", o.Pct)
		}
		o.NameOwn = false
	case "amostra":
		var ok []string
		for _, id := range o.Nuclides {
			if actNuclide(id) != nil {
				ok = append(ok, id)
			}
		}
		if len(ok) == 0 {
			return errors.New("escolha pelo menos um radionuclídeo")
		}
		o.Nuclides = ok
		if o.Name == "" {
			o.Name, o.NameOwn = "amostra", false
		}
	default:
		return errors.New("tipo de medição desconhecido")
	}
	if o.Kind != "amostra" || !o.Loose {
		o.Loose = false
		if o.MassG <= 0 {
			return errors.New("informe a massa em gramas")
		}
	} else if o.MassG < 0 {
		o.MassG = 0
	}
	return nil
}

// StartActivity começa uma medição: pelo fundo (novo) ou direto pela amostra
// (com o fundo salvo).
func (a *App) StartActivity(o ActOpts) error {
	if err := actValidate(&o); err != nil {
		return err
	}
	if !o.NewBg {
		bg := a.actSavedBg()
		if bg == nil {
			return errors.New("não tem fundo salvo: meça um fundo novo")
		}
		if time.Since(time.UnixMilli(bg.T)) > actBgMaxAge {
			return errors.New("o fundo salvo tem mais de 30 dias: meça um novo")
		}
	}
	s := a.rcSpectrumNow(false)
	if s == nil {
		return errors.New("RC102 não conectado")
	}
	now := time.Now()
	r := &actRun{Opts: o, PhaseAt: now.UnixMilli()}
	r.Cur.From = now.UnixMilli()
	r.Cur.feed(*s)
	if o.NewBg {
		r.Phase = "fundo"
	} else {
		r.Phase, r.Bg = "amostra", a.actSavedBg()
	}
	st := &a.rc.act
	st.mu.Lock()
	defer st.mu.Unlock()
	a.actLoadLocked()
	st.run = r
	a.actSaveLocked()
	if o.Loose && o.MassG <= 0 { // pedra solta sem massa: não "0 g" (2.101.12)
		a.logEvent(fmt.Sprintf("Atividade: começou (%s, pedra solta)", actEvName(o)))
	} else {
		a.logEvent(fmt.Sprintf("Atividade: começou (%s, %g g)", actEvName(o), o.MassG))
	}
	return nil
}

// ActivityNext passa para a próxima fase: fundo → trocar a amostra →
// amostra → resultado.
func (a *App) ActivityNext() error {
	st := &a.rc.act
	st.mu.Lock()
	a.actLoadLocked()
	r := st.run
	phase := ""
	if r != nil {
		phase = r.Phase
	}
	st.mu.Unlock()
	if phase == "" || phase == "pronto" {
		return errors.New("nenhuma medição em andamento")
	}
	s := a.rcSpectrumNow(false)
	if s == nil {
		return errors.New("RC102 não conectado")
	}
	st.mu.Lock()
	defer st.mu.Unlock()
	if st.run != r || r.Phase != phase {
		return nil // outra chamada já passou
	}
	a.actAdvanceLocked(s, time.Now())
	return nil
}

// actAdvanceLocked muda de fase com uma leitura nova do espectro.
func (a *App) actAdvanceLocked(s *radiacode.Spectrum, now time.Time) {
	r := a.rc.act.run
	switch r.Phase {
	case "fundo":
		r.Cur.feed(*s)
		r.Bg = r.Cur.saved(now)
		a.actWriteJSON(actBgFile, r.Bg)
		r.Phase, r.PhaseAt, r.Cur = "trocar", now.UnixMilli(), actAcc{}
	case "trocar":
		r.Cur = actAcc{From: now.UnixMilli()}
		r.Cur.feed(*s)
		r.Phase, r.PhaseAt = "amostra", now.UnixMilli()
	case "amostra":
		r.Cur.feed(*s)
		r.Smp = r.Cur.saved(now)
		res := a.actFinish(r, now)
		r.Result = &res
		r.Phase, r.PhaseAt = "pronto", now.UnixMilli()
		a.actAppendResult(res)
		a.logEvent(fmt.Sprintf("Atividade: terminou (%s)", actEvName(r.Opts)))
	}
	a.actSaveLocked()
}

// actFinish calcula o resultado (e atualiza a calibração, se for o caso).
func (a *App) actFinish(r *actRun, now time.Time) ActResult {
	o := r.Opts
	cal := a.actCal()
	smp, bg := r.Smp.spec(), r.Bg.spec()
	res := ActResult{T: now.UnixMilli(), Kind: o.Kind, Name: o.Name, MassG: o.MassG, Geometry: o.Geometry, Sec: smp.Sec, BgSec: bg.Sec,
		Loose: o.Loose, SampleID: o.SampleID, Nucs: o.Nuclides, SmpSecO: o.SmpSec}
	if cal.KCl != nil {
		res.Prev = cal.KCl.Eff
	}
	switch o.Kind {
	case "kcl":
		res.Pct = o.Pct
		k, aK, _ := actCalKCl(smp, bg, o.MassG, o.Pct)
		res.Expected = aK
		if k != nil && k.EffErr > actCalMaxErr {
			// 2.101.4: o pico mal apareceu acima do fundo; guardar isso
			// estragaria todos os Bq daqui para frente
			k.T = res.T
			res.KCl, res.NotSaved = k, true
		} else if k != nil && cal.KCl != nil && k.EffErr > cal.KCl.EffErr {
			// 2.101.14 (item 35): pior que a atual: pergunta antes de trocar
			k.T = res.T
			res.KCl, res.Worse, res.PrevErr = k, true, cal.KCl.EffErr
		} else if k != nil {
			k.T = res.T
			res.KCl = k
			cal.KCl, cal.PrevKCl = k, cal.KCl
			if o.Geometry != "" {
				cal.Geometry = o.Geometry
			}
			a.actWriteJSON(actCalFile, cal)
		}
	case "torio":
		res.Pct, res.AgeY = o.Pct, o.AgeY
		th, lines := actCalTh(smp, bg, o.MassG, o.Pct, o.AgeY)
		res.ThLines = lines
		_, f228, _ := actThFactors(o.AgeY)
		res.Expected = o.MassG * o.Pct / 100 * actThPerTO2 * actBqPerGTh * f228
		if th != nil && cal.Th != nil && th.BErr > cal.Th.BErr {
			th.T = res.T
			res.Th, res.Worse, res.PrevErr = th, true, cal.Th.BErr
		} else if th != nil {
			th.T = res.T
			res.Th = th
			cal.Th, cal.PrevTh = th, cal.Th
			a.actWriteJSON(actCalFile, cal)
		}
	}
	_, _, res.CalSrc = cal.eff(1000)
	res.Nuclides = actAnalyze(smp, bg, o.Nuclides, cal, o.MassG)
	return res
}

func (a *App) actAppendResult(res ActResult) {
	b, err := json.Marshal(res)
	if err != nil {
		return
	}
	os.MkdirAll(a.actPath(""), 0o755)
	f, err := os.OpenFile(a.actPath(actResFile), os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o644)
	if err != nil {
		return
	}
	f.Write(append(b, '\n'))
	f.Close()
}

func (a *App) actHistory() []ActResult {
	out := []ActResult{}
	b, err := os.ReadFile(a.actPath(actResFile))
	if err != nil {
		return out
	}
	sc := rcScanner(bytes.NewReader(b))
	sc.Buffer(make([]byte, 1<<20), 1<<22)
	for sc.Scan() {
		var r ActResult
		if json.Unmarshal(sc.Bytes(), &r) == nil && r.T > 0 {
			out = append(out, r)
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].T > out[j].T })
	if len(out) > actMaxHist {
		out = out[:actMaxHist]
	}
	return out
}

// CancelActivity para a medição sem resultado (o fundo, se já terminou, fica salvo).
func (a *App) CancelActivity() {
	st := &a.rc.act
	st.mu.Lock()
	defer st.mu.Unlock()
	a.actLoadLocked()
	st.run = nil
	a.actSaveLocked()
}

// rcActTick roda a cada volta do rcRun: soma o espectro e passa de fase
// quando o tempo escolhido acaba.
func (a *App) rcActTick(dev *radiacode.Device) {
	a.rc.mu.Lock()
	ble := a.rc.st.Via == "bluetooth"
	a.rc.mu.Unlock()
	st := &a.rc.act
	st.mu.Lock()
	a.actLoadLocked()
	r := st.run
	every := actTickUSB
	if ble {
		every = actTickBLE
	}
	busy := r != nil && (r.Phase == "fundo" || r.Phase == "amostra") && time.Since(st.tickAt) >= every
	if busy {
		st.tickAt = time.Now()
	}
	st.mu.Unlock()
	if !busy {
		return
	}
	s, err := dev.Spectrum()
	if err != nil || len(s.Counts) == 0 {
		return
	}
	st.mu.Lock()
	defer st.mu.Unlock()
	if st.run != r {
		return
	}
	r.Cur.feed(s)
	target := r.Opts.SmpSec
	if r.Phase == "fundo" {
		target = r.Opts.BgSec
	}
	if target > 0 && r.Cur.sec() >= float64(target) {
		a.actAdvanceLocked(&s, time.Now())
		return
	}
	if time.Since(st.saveAt) > 30*time.Second {
		a.actSaveLocked()
	}
}

// ---------- tela ----------

// ActChart são os espectros (por segundo e por keV) para o gráfico.
type ActChart struct {
	EMax float64   `json:"eMax"`
	Smp  []float64 `json:"smp"`
	Bg   []float64 `json:"bg"`
}

// ActView é o que o cartão mostra.
type ActView struct {
	Phase     string          `json:"phase"` // "" | fundo | trocar | amostra | pronto
	Opts      *ActOpts        `json:"opts"`
	PhaseAt   int64           `json:"phaseAt"`
	PhaseSec  float64         `json:"phaseSec"` // tempo somado na fase (s do aparelho)
	Target    int             `json:"target"`
	BgSec     float64         `json:"bgSec"`
	BgT       int64           `json:"bgT"`
	Live      []ActNucRes     `json:"live"`
	LiveKCl   *ActCalKCl      `json:"liveKcl"`
	SmpCps    float64         `json:"smpCps,omitempty"`   // cps de 300 a 1.400 keV da amostra (calibração; 2.101.11: era o total)
	BgCps     float64         `json:"bgCps,omitempty"`    // e do fundo
	KCLWeak   bool            `json:"kclWeak,omitempty"`  // o pico do K-40 não está aparecendo
	HomeCps   float64         `json:"homeCps,omitempty"`  // contagem normal da casa (mediana dos 7 dias antes)
	PhaseCps  float64         `json:"phaseCps,omitempty"` // contagem do RC102 nesta fase (mediana dos minutos)
	BgHigh    bool            `json:"bgHigh,omitempty"`   // o fundo está bem acima do normal da casa
	LiveTh    *ActCalTh       `json:"liveTh"`
	Expected  float64         `json:"expected"`
	Result    *ActResult      `json:"result"`
	Cal       ActCal          `json:"cal"`
	CalSrc    string          `json:"calSrc"`
	CalEff    []float64       `json:"calEff"` // ε em 250, 500, 1000, 1500, 2000, 2500 keV
	CalErr    []float64       `json:"calErr"` // relativa
	SavedBg   *ActBgInfo      `json:"savedBg"`
	Samples   []ActSampleView `json:"samples"`
	History   []ActResult     `json:"history"`
	Chart     *ActChart       `json:"chart"`
	Nuclides  []ActNuclide    `json:"nuclides"`
	Connected bool            `json:"connected"`
}

// ActBgInfo resume o fundo salvo.
type ActBgInfo struct {
	T   int64   `json:"t"`
	Sec float64 `json:"sec"`
}

var actCalE = []float64{250, 500, 1000, 1500, 2000, 2500}

func actChart(smp, bg actSpec) *ActChart {
	const bins, eMax = 300, 3000.0
	ch := &ActChart{EMax: eMax}
	rebin := func(s actSpec) []float64 {
		if s.Sec <= 0 || len(s.C) == 0 {
			return nil
		}
		out := make([]float64, bins)
		bw := eMax / bins
		for i, v := range s.C {
			e := s.energy(float64(i) + 0.5)
			if e < 0 || e >= eMax {
				continue
			}
			out[int(e/bw)] += v
		}
		for i := range out {
			out[i] = math.Round(out[i]/s.Sec/bw*1e5) / 1e5
		}
		return out
	}
	ch.Smp, ch.Bg = rebin(smp), rebin(bg)
	return ch
}

// GetActivity devolve o estado da medição de atividade (com a conta parcial).
func (a *App) GetActivity() ActView {
	v := ActView{Nuclides: actNuclides, Connected: a.rcConnected()}
	v.Cal = a.actCal()
	for _, e := range actCalE {
		eps, rel, src := v.Cal.eff(e)
		v.CalEff, v.CalErr, v.CalSrc = append(v.CalEff, eps), append(v.CalErr, rel), src
	}
	if bg := a.actSavedBg(); bg != nil {
		v.SavedBg = &ActBgInfo{T: bg.T, Sec: bg.Sec}
	}
	v.History = a.actHistory()
	v.Samples = a.actSampleViews()
	st := &a.rc.act
	st.mu.Lock()
	a.actLoadLocked()
	r := st.run
	if r == nil {
		st.mu.Unlock()
		return v
	}
	o := r.Opts
	v.Opts, v.Phase, v.PhaseAt = &o, r.Phase, r.PhaseAt
	var smp, bg actSpec
	if r.Bg != nil {
		bg = r.Bg.spec()
		v.BgSec, v.BgT = r.Bg.Sec, r.Bg.T
	}
	switch r.Phase {
	case "fundo":
		v.PhaseSec, v.Target = r.Cur.sec(), o.BgSec
		bg = r.Cur.saved(time.Now()).spec()
	case "amostra":
		v.PhaseSec, v.Target = r.Cur.sec(), o.SmpSec
		smp = r.Cur.saved(time.Now()).spec()
	case "pronto":
		v.Result = r.Result
		smp = r.Smp.spec()
	}
	st.mu.Unlock()
	if smp.Sec > 0 || bg.Sec > 0 {
		v.Chart = actChart(smp, bg)
	}
	// 2.101.4: compara com a contagem normal da casa (minutos gravados pelo
	// RC102), para avisar de um fundo medido com o sal perto
	if (r.Phase == "fundo" || r.Phase == "amostra") && v.PhaseSec >= actWeakSec {
		at := time.UnixMilli(r.PhaseAt)
		v.HomeCps = a.actHomeCps(at)
		v.PhaseCps = a.actPhaseCps(at.Add(time.Minute))
		v.BgHigh = r.Phase == "fundo" && actBgHigh(v.PhaseCps, v.HomeCps)
	}
	if r.Phase == "amostra" && smp.Sec >= 10 && bg.Sec > 0 {
		switch o.Kind {
		case "kcl":
			v.LiveKCl, v.Expected, _ = actCalKCl(smp, bg, o.MassG, o.Pct)
			v.SmpCps, v.BgCps = smp.bandCps(actWeakLo, actWeakHi), bg.bandCps(actWeakLo, actWeakHi)
			v.KCLWeak = actKCLWeak(smp.Sec, v.SmpCps, v.BgCps, v.LiveKCl)
		case "torio":
			v.LiveTh, _ = actCalTh(smp, bg, o.MassG, o.Pct, o.AgeY)
			_, f228, _ := actThFactors(o.AgeY)
			v.Expected = o.MassG * o.Pct / 100 * actThPerTO2 * actBqPerGTh * f228
		}
		v.Live = actAnalyze(smp, bg, o.Nuclides, v.Cal, o.MassG)
	}
	return v
}

// actKCLWeak avisa cedo (2.101.4) que o RC102 não está vendo o sal: depois
// de 20 min, a contagem de 300 a 1.400 keV (o espalhamento do K-40) quase
// não subiu em relação ao fundo; depois de 30 min, também se nem a
// eficiência mais 2σ chega ao mínimo plausível em 1.460 keV.
// 2.101.11: a calibração do ACE (18/11, 250 g de sal light no Marinelli)
// mostrou o total CAINDO 1% (o sal blinda abaixo de 100 keV) e a faixa de
// 300 a 1.400 keV subindo 20%, com eficiência de 0,0045% no pico.
func actKCLWeak(sec, smpCps, bgCps float64, k *ActCalKCl) bool {
	if sec < actWeakSec {
		return false
	}
	if bgCps > 0 && smpCps < bgCps*actWeakRise {
		return true
	}
	// o pico sozinho só decide com mais tempo (aos 20 min o ruído ainda é grande)
	return sec >= actWeakPeakSec && (k == nil || k.Eff*(1+2*k.EffErr) < actEffMin)
}

// actBgHigh: o fundo está mais de 25% acima do normal da casa (o sal, ou
// outra amostra, ficou perto do aparelho).
func actBgHigh(phaseCps, homeCps float64) bool {
	return homeCps > 0 && phaseCps > homeCps*actBgHighRise
}

// actHomeCps é a mediana da contagem por segundo do RC102 nos 7 dias antes de
// before (sem minutos incompletos); 0 se tiver menos de 1 h gravada.
func (a *App) actHomeCps(before time.Time) float64 {
	st := &a.rc.act
	st.mu.Lock()
	if st.homeBefore == before.Unix() && time.Since(st.homeAt) < 10*time.Minute {
		v := st.homeCps
		st.mu.Unlock()
		return v
	}
	st.mu.Unlock()
	v := actMedianCps(a.rcRange(before.Add(-7*24*time.Hour), before), 60)
	st.mu.Lock()
	st.homeCps, st.homeAt, st.homeBefore = v, time.Now(), before.Unix()
	st.mu.Unlock()
	return v
}

// actPhaseCps é a mediana da contagem por segundo desde from.
func (a *App) actPhaseCps(from time.Time) float64 {
	return actMedianCps(a.rcRange(from, time.Now().Add(time.Minute)), 3)
}

func actMedianCps(pts []RcPoint, min int) float64 {
	var xs []float64
	for _, p := range pts {
		if p.N >= 20 && p.CPS > 0 {
			xs = append(xs, p.CPS)
		}
	}
	if len(xs) < min {
		return 0
	}
	return stats.Median(xs)
}

// CloseActivity fecha o resultado (volta para o formulário).
func (a *App) CloseActivity() { a.CancelActivity() }

// ForgetActivityCal apaga a calibração (para recomeçar).
func (a *App) ForgetActivityCal(which string) {
	c := a.actCal()
	switch which {
	case "kcl":
		c.KCl, c.PrevKCl = nil, nil
	case "torio":
		c.Th, c.PrevTh = nil, nil
	default:
		return
	}
	a.actWriteJSON(actCalFile, c)
}

// UseActivityCal usa a calibração que acabou de sair pior que a atual
// (2.101.14, item 35): a atual vira a anterior, para poder voltar.
func (a *App) UseActivityCal(t int64) error {
	st := &a.rc.act
	st.mu.Lock()
	defer st.mu.Unlock()
	r := st.run
	if r == nil || r.Result == nil || r.Result.T != t || !r.Result.Worse || r.Result.Used {
		return errors.New("essa calibração não está mais na tela")
	}
	res := r.Result
	c := a.actCal()
	switch {
	case res.KCl != nil && res.Kind == "kcl":
		c.KCl, c.PrevKCl = res.KCl, c.KCl
		if res.Geometry != "" {
			c.Geometry = res.Geometry
		}
	case res.Th != nil && res.Kind == "torio":
		c.Th, c.PrevTh = res.Th, c.Th
	default:
		return errors.New("essa medição não tem calibração")
	}
	a.actWriteJSON(actCalFile, c)
	res.Used = true
	_, _, res.CalSrc = c.eff(1000)
	a.actRewriteResult(*res)
	a.actSaveLocked()
	a.logEvent(fmt.Sprintf("Atividade: calibração trocada pela nova (%s)", actEvName(r.Opts)))
	return nil
}

// RestoreActivityCal volta para a calibração de antes (KCl ou tório).
func (a *App) RestoreActivityCal(which string) error {
	c := a.actCal()
	switch which {
	case "kcl":
		if c.PrevKCl == nil {
			return errors.New("não há calibração anterior do KCl")
		}
		c.KCl, c.PrevKCl = c.PrevKCl, c.KCl
	case "torio":
		if c.PrevTh == nil {
			return errors.New("não há calibração anterior do tório")
		}
		c.Th, c.PrevTh = c.PrevTh, c.Th
	default:
		return errors.New("calibração desconhecida")
	}
	a.actWriteJSON(actCalFile, c)
	return nil
}

// actRewriteResult troca, no histórico, o resultado com o mesmo horário.
func (a *App) actRewriteResult(res ActResult) {
	p := a.actPath(actResFile)
	b, err := os.ReadFile(p)
	if err != nil {
		return
	}
	nb, err := json.Marshal(res)
	if err != nil {
		return
	}
	lines := bytes.Split(b, []byte("\n"))
	for i, ln := range lines {
		var q struct {
			T    int64  `json:"t"`
			Kind string `json:"kind"`
		}
		if json.Unmarshal(ln, &q) == nil && q.T == res.T && q.Kind == res.Kind {
			lines[i] = nb
		}
	}
	writeFileAtomic(p, bytes.Join(lines, []byte("\n")), 0o644)
}

// ---------- backup ----------

func (a *App) actBackupFiles() []string {
	var out []string
	for _, n := range []string{actCalFile, actBgFile, actResFile, actSamplesFile} {
		if _, err := os.Stat(a.actPath(n)); err == nil {
			out = append(out, rcDirName+"/"+actDir+"/"+n)
		}
	}
	return out
}

// actMerge junta um arquivo da atividade vindo de um backup: calibração e
// fundo ficam com o mais novo; as medidas juntam sem repetir.
func (a *App) actMerge(name string, data []byte) {
	newer := func(cur []byte, in []byte, at func([]byte) int64) bool { return at(in) > at(cur) }
	switch name {
	case actCalFile:
		at := func(b []byte) int64 {
			var c ActCal
			json.Unmarshal(b, &c)
			t := int64(0)
			if c.KCl != nil {
				t = c.KCl.T
			}
			if c.Th != nil && c.Th.T > t {
				t = c.Th.T
			}
			return t
		}
		cur, _ := os.ReadFile(a.actPath(name))
		if cur == nil || newer(cur, data, at) {
			os.MkdirAll(a.actPath(""), 0o755)
			_ = writeFileAtomic(a.actPath(name), data, 0o644)
		}
	case actBgFile:
		at := func(b []byte) int64 {
			var s struct {
				T int64 `json:"t"`
			}
			json.Unmarshal(b, &s)
			return s.T
		}
		cur, _ := os.ReadFile(a.actPath(name))
		if cur == nil || newer(cur, data, at) {
			os.MkdirAll(a.actPath(""), 0o755)
			_ = writeFileAtomic(a.actPath(name), data, 0o644)
		}
	case actResFile:
		cur, _ := os.ReadFile(a.actPath(name))
		have := map[string]bool{}
		var lines []string
		for _, src := range [][]byte{cur, data} {
			sc := rcScanner(bytes.NewReader(src))
			sc.Buffer(make([]byte, 1<<20), 1<<22)
			for sc.Scan() {
				l := strings.TrimSpace(sc.Text())
				if l == "" || have[l] {
					continue
				}
				have[l] = true
				lines = append(lines, l)
			}
		}
		os.MkdirAll(a.actPath(""), 0o755)
		_ = writeFileAtomic(a.actPath(name), []byte(strings.Join(lines, "\n")+"\n"), 0o644)
	}
}
