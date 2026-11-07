package app

import (
	"errors"
	"fmt"
	"math"
	"sort"
	"strings"
	"time"

	"radlog/internal/radiacode"
)

// ---------- medir um objeto: modo automático e fundo da casa ----------
//
// Fundo da casa: o contador fica medindo o fundo o dia inteiro, então a
// última hora gravada (em casa, fora de alertas e medições) já é um fundo
// bem mais preciso que 1–5 min medidos na hora. Só vale se o objeto for medido
// no lugar de sempre do contador.
//
// Automático: a fase do objeto para sozinha quando dá resposta:
//   - achou: líquido ≥ 4σ (4 e não 3 porque o teste é feito a cada segundo;
//     com 3σ, olhar tantas vezes daria alarme falso em ~6% das medições);
//   - nada: o limite superior (máx(líquido, 0) + 2σ) ficou abaixo do que
//     conta como "tem algo" (um % do fundo);
//   - limite: chegou ao tempo máximo sem decidir.

const (
	homeBgWindow  = 60 * time.Minute
	homeBgMinMins = 30 // minutos válidos para confiar no fundo da casa
	autoMinSec    = 20 // antes disso não decide (poucas contagens)
	autoDetectSig = 4.0
	autoBgSec     = 300 // fase do fundo medida na hora, no automático
)

// MeasureOpts são as escolhas da tela antes de começar.
type MeasureOpts struct {
	Name     string  `json:"name"`
	Distance string  `json:"distance"`
	Auto     bool    `json:"auto"`
	HomeBg   bool    `json:"homeBg"`
	Seconds  int     `json:"seconds"` // manual: cada fase
	MaxSec   int     `json:"maxSec"`  // automático: limite da fase do objeto
	Pct      float64 `json:"pct"`     // automático: % do fundo que conta como "tem algo"
	Device   string  `json:"device"`  // "gmc" (ou "") ou "radiacode": o que fica no objeto
	Mode     string  `json:"mode"`    // "", "ambos" (os dois perto) ou "voce" (o outro com a pessoa)
}

// HomeBg é o fundo da casa da última hora.
type HomeBg struct {
	OK      bool    `json:"ok"`
	CPM     float64 `json:"cpm"`
	ErrCPM  float64 `json:"errCPM"`
	Minutes int     `json:"minutes"`
	Counts  int     `json:"counts"`
	From    int64   `json:"from"`
	To      int64   `json:"to"`
	Why     string  `json:"why"` // por que não dá (OK = false)
	Demo    bool    `json:"demo"`
	// µSv/h do fundo (RadiaCode: média do aparelho; GMC: pela calibração)
	USv    float64 `json:"usv"`
	Device string  `json:"device"`
}

// GetHomeBackgroundDev devolve o fundo da casa da última hora pelo aparelho
// escolhido ("gmc" ou "radiacode").
func (a *App) GetHomeBackgroundDev(dev string) HomeBg {
	if devName(dev) == "radiacode" {
		return a.rcHomeBackground()
	}
	hb := a.GetHomeBackground()
	hb.Device = "gmc"
	if hb.OK {
		a.mu.Lock()
		hb.USv = a.convCalLocked().USv(hb.CPM)
		a.mu.Unlock()
	}
	return hb
}

// GetHomeBackground devolve o fundo da casa da última hora (GMC).
func (a *App) GetHomeBackground() HomeBg {
	now := time.Now()
	a.mu.Lock()
	demo := a.demo.Active
	var demoMins []float64
	var demoCPM float64
	if demo {
		demoCPM = a.demo.CPM
		for k, v := range a.demo.minutes {
			if t := time.Unix(k, 0); t.After(now.Add(-homeBgWindow)) {
				demoMins = append(demoMins, v)
			}
		}
	}
	a.mu.Unlock()
	if demo {
		// demo: os minutos simulados, ou o valor do demo se acabou de ligar
		if len(demoMins) < 10 {
			demoMins = make([]float64, 60)
			for i := range demoMins {
				demoMins[i] = demoCPM
			}
		}
		hb := homeBgFrom(demoMins, now.Add(-time.Duration(len(demoMins))*time.Minute), now)
		hb.Demo = true
		return hb
	}
	if a.store == nil {
		return HomeBg{Why: "sem dados gravados"}
	}
	pts, err := a.store.Range(now.Add(-homeBgWindow), now.Add(time.Minute))
	if err != nil {
		return HomeBg{Why: err.Error()}
	}
	pts = a.homeOnly(pts)
	inAlert := a.alertChecker()
	var mins []float64
	var from, to time.Time
	for _, p := range pts {
		if p.Src == "demo" || inAlert(p.T) {
			continue
		}
		if from.IsZero() {
			from = p.T
		}
		to = p.T
		mins = append(mins, p.CPM)
	}
	if len(mins) < homeBgMinMins {
		return HomeBg{Minutes: len(mins), Why: fmt.Sprintf("só %d min do fundo da casa na última hora (precisa de %d)", len(mins), homeBgMinMins)}
	}
	return homeBgFrom(mins, from, to.Add(time.Minute))
}

// homeBgFrom faz a média dos minutos, tirando os que fogem muito (picos de
// interferência), com a incerteza de Poisson ou a do espalhamento real, a
// que for maior (chuva e variações do dia deixam o fundo mais solto).
func homeBgFrom(mins []float64, from, to time.Time) HomeBg {
	med := median(mins)
	lim := med + 5*math.Sqrt(math.Max(med, 1))
	var kept []float64
	for _, v := range mins {
		if v <= lim {
			kept = append(kept, v)
		}
	}
	n := float64(len(kept))
	if n == 0 {
		return HomeBg{Why: "sem minutos válidos"}
	}
	sum, sq := 0.0, 0.0
	for _, v := range kept {
		sum += v
	}
	mean := sum / n
	for _, v := range kept {
		sq += (v - mean) * (v - mean)
	}
	errP := math.Sqrt(math.Max(sum, 1)) / n
	errS := 0.0
	if n > 1 {
		errS = math.Sqrt(sq/(n-1)) / math.Sqrt(n)
	}
	return HomeBg{OK: true, CPM: mean, ErrCPM: math.Max(errP, errS), Minutes: len(kept), Counts: int(math.Round(sum)),
		From: from.UnixMilli(), To: to.UnixMilli()}
}

// alertChecker diz se um instante caiu dentro de um alerta (não é fundo).
func (a *App) alertChecker() func(time.Time) bool {
	a.mu.Lock()
	var spans [][2]int64
	for _, e := range a.events {
		if e.Kind != kindAlert || e.Demo {
			continue
		}
		end := e.End
		if end == 0 && e.Active {
			end = time.Now().UnixMilli()
		} else if end == 0 {
			end = e.Time + 5*60_000 // fim desconhecido
		}
		spans = append(spans, [2]int64{e.Time - 60_000, end + 60_000})
	}
	a.mu.Unlock()
	sort.Slice(spans, func(i, j int) bool { return spans[i][0] < spans[j][0] })
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

// StartMeasure começa uma medição nova com as escolhas da tela: mede o
// fundo agora ou já começa pelo objeto usando o fundo da casa.
func (a *App) StartMeasure(o MeasureOpts) (MeasureState, error) {
	if o.MaxSec < 60 || o.MaxSec > 3600 {
		o.MaxSec = 1800
	}
	if o.Pct <= 0 || o.Pct > 200 {
		o.Pct = 20
	}
	if o.Seconds < 10 || o.Seconds > 1800 {
		o.Seconds = 60
	}
	dev := devName(o.Device)
	if o.Mode != "ambos" && o.Mode != "voce" {
		o.Mode = ""
	}
	if o.Mode == "ambos" {
		dev = devRC // o RadiaCode comanda (conta muito mais, decide antes)
	}
	var side *MeasureSide
	if o.Mode != "" {
		var err error
		if side, err = a.newSide(dev, o.Mode, o.HomeBg); err != nil {
			return a.GetMeasure(), err
		}
	}
	var hb HomeBg
	if o.HomeBg {
		hb = a.GetHomeBackgroundDev(dev)
		if !hb.OK {
			return a.GetMeasure(), errors.New("fundo da casa indisponível: " + hb.Why)
		}
	}
	// RadiaCode: espectro no começo (e o acumulado, que serve de fundo da casa)
	var spec0 *radiacode.Spectrum
	var specBg *MeasureSpectrum
	if dev == "radiacode" {
		if !a.rcConnected() {
			return a.GetMeasure(), errors.New("conecte o RadiaCode para medir")
		}
		spec0 = a.rcSpectrumNow(false)
		if o.HomeBg {
			if acc := a.rcSpectrumNow(true); acc != nil && len(acc.Counts) > 0 {
				specBg = &MeasureSpectrum{A: [3]float64{acc.A0, acc.A1, acc.A2}, Bg: acc.Counts, BgSec: acc.Duration.Seconds(), BgSrc: "acumulado"}
			}
		}
	}
	a.mu.Lock()
	if dev == "gmc" && a.dev == nil && !a.demo.Active {
		st := a.measureSnapLocked()
		a.mu.Unlock()
		return st, errors.New("conecte o contador (ou ligue o modo demo) para medir")
	}
	now := time.Now().UnixMilli()
	m := MeasureState{Name: strings.TrimSpace(o.Name), Distance: strings.TrimSpace(o.Distance), Demo: a.demo.Active && dev == "gmc",
		Auto: o.Auto, MaxSec: o.MaxSec, Pct: o.Pct, PhaseStart: now, Eta: -1, Device: dev, Mode: o.Mode, Other: side}
	if side != nil {
		m.Demo = false
	}
	if o.HomeBg {
		bg := MeasurePhase{Counts: hb.Counts, Seconds: hb.Minutes * 60, CPM: hb.CPM, ErrCPM: hb.ErrCPM,
			USv: hb.USv, Start: hb.From, End: hb.To, Source: "casa"}
		m.Bg = &bg
		m.Phase = "objeto"
		m.Seconds = o.Seconds
		if o.Auto {
			m.Seconds = o.MaxSec
		}
	} else {
		m.Phase = "fundo"
		m.Seconds = o.Seconds
		if o.Auto {
			m.Seconds = autoBgSec
		}
	}
	a.measure = m
	a.measSamples = nil
	a.measSpec0, a.measSpecBg = spec0, specBg
	st := a.measureSnapLocked()
	a.mu.Unlock()
	go a.emit("measure", st)
	return st, nil
}

// autoDecide olha o resultado parcial da fase do objeto e diz se já dá para
// parar ("achou", "nada"), ou "" para continuar.
func autoDecide(r MeasureResult, bg MeasurePhase, elapsed int, pct float64) string {
	if elapsed < autoMinSec {
		return ""
	}
	if r.Sigma >= autoDetectSig {
		return "achou"
	}
	if math.Max(r.NetCPM, 0)+2*r.NetErr < autoThreshold(bg, pct) {
		return "nada"
	}
	return ""
}

// autoThreshold é o que conta como "tem algo", em CPM acima do fundo.
func autoThreshold(bg MeasurePhase, pct float64) float64 {
	return math.Max(1, bg.CPM*pct/100)
}

// autoEta estima quantos segundos faltam para decidir, supondo que as taxas
// fiquem como estão; -1 = não deve decidir antes do limite.
func autoEta(bg, obj MeasurePhase, elapsed, maxSec int, pct float64) int {
	if elapsed < 5 {
		return -1
	}
	net := obj.CPM - bg.CPM
	rate := math.Max(obj.CPM, bg.CPM) // CPM do objeto (o erro dele cai com √t)
	need := math.Inf(1)
	// achou: net / √(σb² + rate/t) ≥ 4
	if q := math.Pow(net/autoDetectSig, 2) - bg.ErrCPM*bg.ErrCPM; net > 0 && q > 0 {
		need = math.Min(need, rate/q*60)
	}
	// nada: net + 2√(σb² + rate/t) < limiar
	if d := autoThreshold(bg, pct) - math.Max(net, 0); d > 0 {
		if q := math.Pow(d/2, 2) - bg.ErrCPM*bg.ErrCPM; q > 0 {
			need = math.Min(need, rate/q*60)
		}
	}
	need = math.Max(need, autoMinSec)
	if math.IsInf(need, 1) || need > float64(maxSec) {
		return -1
	}
	return int(math.Ceil(need)) - elapsed
}

// finishResult completa o resultado final com o limite superior e o jeito
// que a medição parou.
func finishResult(r *MeasureResult, stop string, bg MeasurePhase, pct float64, auto bool) {
	r.Stop = stop
	if r.Level != "sim" {
		r.Limit = math.Max(r.NetCPM, 0) + 2*r.NetErr
	}
	switch stop {
	case "nada":
		r.Level = "nada"
		if bg.CPM > 0 && bg.USv > 0 { // em µSv/h, como o resto da tela
			r.Verdict = fmt.Sprintf("igual ao fundo: se o objeto emite algo, é menos que +%s µSv/h", usvTxt(r.Limit*bg.USv/bg.CPM))
		} else {
			r.Verdict = fmt.Sprintf("igual ao fundo: se o objeto emite algo, é menos que +%s CPM", br(r.Limit, false))
		}
	case "limite":
		if r.Level != "sim" {
			r.Verdict += " (chegou ao tempo máximo sem decidir)"
		}
	}
	if auto {
		r.Pct = pct
		r.Threshold = autoThreshold(bg, pct)
	}
}
