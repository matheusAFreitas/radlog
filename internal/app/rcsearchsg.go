package app

import (
	"math"
	"sync"
	"time"

	"radlog/internal/radiacode"
)

// ---------- espectrograma ao vivo do modo busca (2.100.0) ----------
//
// Com a aba Busca aberta (a tela pergunta GetRcSearchSg de tempos em
// tempos), o app lê o espectro do aparelho a cada 2 s no USB e a cada 3,5 s
// no Bluetooth e guarda na memória a diferença entre leituras: o que chegou
// naquele intervalo. Fechou a aba, volta a ler só de minuto em minuto (o
// espectrograma gravado). Nada disso vai para o disco.

const (
	rcSgFastUSB  = 2 * time.Second
	rcSgFastBLE  = 3500 * time.Millisecond
	rcSgFastSeen = 6 * time.Second // a tela parou de perguntar: para
	rcSgFastKeep = 5*time.Minute + 30*time.Second
	rcSgFastBins = 256
	rcSgFastEMax = 1500.0
	rcSgPeaksTTL = 8 * time.Second  // "Parece": refaz no máximo a cada 8 s
	rcSgBgTTL    = 10 * time.Minute // fundo da casa (7 dias): refaz a cada 10 min
)

type rcSgFastRow struct {
	t int64
	s float64
	a [3]float64
	c []uint32 // canais do aparelho
}

type rcSgFast struct {
	mu      sync.Mutex
	seen    time.Time // última pergunta da tela
	at      time.Time // última leitura (só a goroutine do rcRun mexe)
	prev    *rcSpecSnap
	rows    []rcSgFastRow
	peaks   []SpecPeak
	peaksAt time.Time
	peaksW  int
	bg      []uint32
	bgSec   float64
	bgAt    time.Time
}

// rcSgFastTick roda a cada volta do rcRun: lê o espectro se a aba Busca
// estiver aberta e já deu o intervalo.
func (a *App) rcSgFastTick(dev *radiacode.Device, ble bool) {
	f := &a.rc.sgFast
	every := rcSgFastUSB
	if ble {
		every = rcSgFastBLE
	}
	f.mu.Lock()
	on := time.Since(f.seen) < rcSgFastSeen
	if !on && f.prev != nil { // aba fechada: esquece tudo
		f.prev, f.rows, f.peaks = nil, nil, nil
	}
	due := on && time.Since(f.at) >= every
	if due {
		f.at = time.Now()
	}
	f.mu.Unlock()
	if !due {
		return
	}
	s, err := dev.Spectrum()
	if err != nil || len(s.Counts) == 0 {
		return
	}
	a.rcSgFastAdd(dev.Serial, time.Now(), s)
}

func (a *App) rcSgFastAdd(serial string, now time.Time, s radiacode.Spectrum) {
	f := &a.rc.sgFast
	f.mu.Lock()
	defer f.mu.Unlock()
	cur := &rcSpecSnap{Serial: serial, T: now.UnixMilli(), S: s.Duration.Seconds(), A: [3]float64{s.A0, s.A1, s.A2}, C: append([]uint32(nil), s.Counts...)}
	prev := f.prev
	f.prev = cur
	if prev == nil || prev.Serial != serial {
		return
	}
	if cur.T-prev.T > int64(3*rcSgFastBLE/time.Millisecond) {
		return // ficou um tempo sem ler (aba fechada, Bluetooth caiu): recomeça daqui
	}
	r := rcSpecDiff(prev, cur)
	if r == nil {
		return
	}
	f.rows = append(f.rows, rcSgFastRow{t: r.T, s: r.S, a: r.A, c: r.C})
	cut := now.Add(-rcSgFastKeep).UnixMilli()
	i := 0
	for i < len(f.rows) && f.rows[i].t < cut {
		i++
	}
	if i > 0 {
		f.rows = append([]rcSgFastRow(nil), f.rows[i:]...)
	}
}

// rcSgBin passa os canais do aparelho para rcSgFastBins faixas iguais de 0 a
// rcSgFastEMax keV (pela calibração de energia do próprio aparelho).
func rcSgBin(c []uint32, a [3]float64) []float64 {
	out := make([]float64, rcSgFastBins)
	for ch, v := range c {
		if v == 0 {
			continue
		}
		x := float64(ch) + 0.5
		e := a[0] + a[1]*x + a[2]*x*x
		if e < 0 || e >= rcSgFastEMax {
			continue
		}
		out[int(e/rcSgFastEMax*rcSgFastBins)] += float64(v)
	}
	return out
}

// RcSearchSgRow é uma coluna do espectrograma da busca.
type RcSearchSgRow struct {
	T int64     `json:"t"` // fim (ms)
	S float64   `json:"s"` // segundos cobertos
	C []float64 `json:"c"` // contagens em cada faixa
}

// RcSearchSg é o que a tela da busca precisa para o espectrograma.
type RcSearchSg struct {
	On    bool            `json:"on"`
	EMax  float64         `json:"eMax"`
	Bins  int             `json:"bins"`
	Every float64         `json:"every"` // segundos entre colunas
	Rows  []RcSearchSgRow `json:"rows"`  // colunas depois de sinceMs
	Peaks []SpecPeak      `json:"peaks"` // o que parece, na janela (só o que passou do fundo)
}

// GetRcSearchSg devolve as colunas depois de sinceMs e o que parece o que
// chegou nos últimos windowSec segundos. Chamar mantém a leitura rápida ligada.
func (a *App) GetRcSearchSg(sinceMs int64, windowSec int) RcSearchSg {
	a.rc.mu.Lock()
	live := a.rc.dev != nil
	ble := a.rc.st.Via == "bluetooth"
	a.rc.mu.Unlock()
	out := RcSearchSg{On: live, EMax: rcSgFastEMax, Bins: rcSgFastBins, Every: rcSgFastUSB.Seconds(), Rows: []RcSearchSgRow{}, Peaks: []SpecPeak{}}
	if ble {
		out.Every = rcSgFastBLE.Seconds()
	}
	f := &a.rc.sgFast
	f.mu.Lock()
	f.seen = time.Now()
	if !live {
		f.mu.Unlock()
		return out
	}
	for _, r := range f.rows {
		if r.t > sinceMs {
			out.Rows = append(out.Rows, RcSearchSgRow{T: r.t, S: r.s, C: rcSgBin(r.c, r.a)})
		}
	}
	if windowSec < 30 || windowSec > 600 {
		windowSec = 120
	}
	need := len(f.rows) > 0 && (time.Since(f.peaksAt) > rcSgPeaksTTL || f.peaksW != windowSec)
	var obj []uint32
	var objSec float64
	var cal [3]float64
	if need {
		f.peaksAt, f.peaksW = time.Now(), windowSec
		from := f.rows[len(f.rows)-1].t - int64(windowSec)*1000
		for _, r := range f.rows {
			if r.t <= from {
				continue
			}
			if obj == nil {
				obj = make([]uint32, len(r.c))
			}
			if len(r.c) != len(obj) {
				continue
			}
			for c, v := range r.c {
				obj[c] += v
			}
			objSec += r.s
			cal = r.a
		}
	}
	bg, bgSec, bgAt := f.bg, f.bgSec, f.bgAt
	f.mu.Unlock()
	if need && objSec > 0 {
		if time.Since(bgAt) > rcSgBgTTL {
			bg, bgSec = a.rcHomeBg(time.Now().UnixMilli())
			f.mu.Lock()
			f.bg, f.bgSec, f.bgAt = bg, bgSec, time.Now()
			f.mu.Unlock()
		}
		var p []SpecPeak
		if len(bg) == len(obj) && bgSec > 0 {
			s := MeasureSpectrum{A: cal, Obj: obj, ObjSec: objSec, Bg: bg, BgSec: bgSec}
			p = s.findPeaks()
		}
		f.mu.Lock()
		f.peaks = p
		f.mu.Unlock()
	}
	f.mu.Lock()
	if f.peaks != nil {
		out.Peaks = f.peaks
	}
	f.mu.Unlock()
	return out
}

// rcHomeBg é o fundo da casa: a soma dos espectros gravados nos últimos
// rcHardDays dias (o mesmo que "Com o que se parece" usa).
func (a *App) rcHomeBg(toMs int64) ([]uint32, float64) {
	if a.dataDir == "" {
		return nil, 0
	}
	a.rcSpecFlush()
	names, days := a.rcSpecFiles()
	refFrom := toMs - rcHardDays*24*3600e3
	var bg []float64
	var bgSec float64
	for i, name := range names {
		d0, d1 := days[i].UnixMilli(), days[i].AddDate(0, 0, 1).UnixMilli()
		if d0 > toMs || d1 < refFrom {
			continue
		}
		if sm := a.rcSpecSummary(name); sm != nil && len(sm.sum) > 0 {
			if bg == nil {
				bg = make([]float64, len(sm.sum))
			}
			if len(sm.sum) == len(bg) {
				for c, v := range sm.sum {
					bg[c] += v
				}
				bgSec += sm.secs
			}
		}
	}
	if bg == nil {
		return nil, 0
	}
	out := make([]uint32, len(bg))
	for c, v := range bg {
		out[c] = uint32(math.Round(v))
	}
	return out, bgSec
}
