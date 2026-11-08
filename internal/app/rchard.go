package app

import (
	"encoding/json"
	"math"
	"os"
	"sync"
	"time"

	"radlog/internal/core/stats"
)

// ---------- dureza ao longo do tempo (RadiaCode) ----------
//
// Dureza é a proporção de fótons acima de 1.500 keV sobre os de 150 a
// 1.500 keV, relativa ao fundo da casa (1,00 = igual ao fundo). Sai dos
// espectros de minuto em minuto do espectrograma. O "fundo da casa" é a
// mediana das horas dos últimos 7 dias. Como chegam poucos fótons acima de
// 1.500 keV, cada ponto usa uns 20 minutos em volta.

const (
	rcHardLo   = 150.0
	rcHardMid  = 1500.0
	rcHardWin  = 20 * 60 // segundos
	rcHardDays = 7
)

// rcHardRow é uma linha gravada resumida: só o que a dureza precisa.
type rcHardRow struct {
	T            int64
	S            float64
	Hi, Mid, Tot float64
}

// rcSpecSum é o resumo de um arquivo de espectros (fica em memória enquanto
// o arquivo não muda).
type rcSpecSum struct {
	size int64
	rows []rcHardRow
	sum  []float64 // contagens por canal, somadas no arquivo
	secs float64
	a    [3]float64
}

type rcSpecCache struct {
	mu sync.Mutex
	m  map[string]*rcSpecSum
}

func (a *App) rcSpecFlush() {
	a.rc.spec.mu.Lock()
	if a.rc.spec.gz != nil {
		a.rc.spec.gz.Flush()
	}
	a.rc.spec.mu.Unlock()
}

// rcSpecRead lê as linhas de um arquivo de espectros.
func (a *App) rcSpecRead(name string) []rcSpecRow {
	b, err := os.ReadFile(a.rcSpecPath(name))
	if err != nil {
		return nil
	}
	var out []rcSpecRow
	for _, ln := range gzipLinesTolerant(b) {
		var r rcSpecRow
		if json.Unmarshal(ln, &r) != nil || r.S <= 0 || len(r.C) == 0 {
			continue
		}
		out = append(out, r)
	}
	return out
}

// rcSpecSummary resume um arquivo (com cache pelo tamanho).
func (a *App) rcSpecSummary(name string) *rcSpecSum {
	fi, err := os.Stat(a.rcSpecPath(name))
	if err != nil {
		return nil
	}
	c := &a.rc.specSum
	c.mu.Lock()
	if s := c.m[name]; s != nil && s.size == fi.Size() {
		c.mu.Unlock()
		return s
	}
	c.mu.Unlock()
	s := &rcSpecSum{size: fi.Size()}
	var calA [3]float64
	var en []float64
	for _, r := range a.rcSpecRead(name) {
		if en == nil || r.A != calA || len(en) != len(r.C) {
			calA, en = r.A, make([]float64, len(r.C))
			for i := range en {
				x := float64(i) + 0.5
				en[i] = r.A[0] + r.A[1]*x + r.A[2]*x*x
			}
		}
		if len(s.sum) != len(r.C) {
			s.sum = make([]float64, len(r.C))
		}
		h := rcHardRow{T: r.T, S: r.S}
		for i, v := range r.C {
			f := float64(v)
			h.Tot += f
			s.sum[i] += f
			if en[i] >= rcHardMid {
				h.Hi += f
			} else if en[i] >= rcHardLo {
				h.Mid += f
			}
		}
		s.rows = append(s.rows, h)
		s.secs += r.S
		s.a = r.A
	}
	c.mu.Lock()
	if c.m == nil {
		c.m = map[string]*rcSpecSum{}
	}
	c.m[name] = s
	c.mu.Unlock()
	return s
}

// RcHardness é a contagem e a dureza do RadiaCode ao longo do tempo, em
// linhas de RowSec segundos a partir de From.
type RcHardness struct {
	From    int64     `json:"from"`
	To      int64     `json:"to"`
	RowSec  int       `json:"rowSec"`
	WinSec  int       `json:"winSec"`  // janela de cada ponto da dureza
	CPS     []float64 `json:"cps"`     // 0 = sem dados
	Hard    []float64 `json:"hard"`    // relativa ao fundo; 0 = pouco dado
	HardErr []float64 `json:"hardErr"` // incerteza (1σ)
	Gap     []bool    `json:"gap"`     // veio de um intervalo sem leitura
	Rain    []float64 `json:"rain"`    // chuva (mm/h, o máximo na linha)
	Wet     []bool    `json:"wet"`     // choveu de verdade
	Ref     float64   `json:"ref"`     // acima/meio do fundo da casa
	RefCPS  float64   `json:"refCps"`  // cps do fundo da casa
	First   int64     `json:"first"`
}

// GetRcHardness monta a contagem e a dureza de fromMs a toMs.
func (a *App) GetRcHardness(fromMs, toMs int64, rowSec int) (RcHardness, error) {
	if rowSec < 10 {
		rowSec = 60
	}
	if toMs-fromMs > int64(rcSgMaxSpan)*3600e3 {
		fromMs = toMs - int64(rcSgMaxSpan)*3600e3
	}
	rowMs := int64(rowSec) * 1000
	n := int((toMs - fromMs + rowMs - 1) / rowMs)
	if n > rcSgMaxRows {
		n = rcSgMaxRows
		fromMs = toMs - int64(n)*rowMs
	}
	if n <= 0 {
		return RcHardness{}, nil
	}
	win := max(rcHardWin, rowSec)
	half := int(float64(win) / 2 / float64(rowSec))
	out := RcHardness{From: fromMs, To: toMs, RowSec: rowSec, WinSec: (2*half + 1) * rowSec,
		CPS: make([]float64, n), Hard: make([]float64, n), HardErr: make([]float64, n), Gap: make([]bool, n),
		Rain: make([]float64, n), Wet: make([]bool, n)}
	if a.dataDir == "" {
		return out, nil
	}
	a.rcSpecFlush()
	// linhas com folga de meia janela dos dois lados
	m := n + 2*half
	x0 := float64(fromMs - int64(half)*rowMs)
	hi, mid, tot, secs := make([]float64, m), make([]float64, m), make([]float64, m), make([]float64, m)
	type hour struct{ hi, mid, tot, secs float64 }
	hours := map[int64]*hour{}
	refFrom := toMs - rcHardDays*24*3600e3
	names, days := a.rcSpecFiles()
	lo := min(refFrom, int64(x0))
	for i, name := range names {
		if days[i].UnixMilli() > toMs || days[i].AddDate(0, 0, 1).UnixMilli() < lo {
			continue
		}
		sm := a.rcSpecSummary(name)
		if sm == nil {
			continue
		}
		for _, r := range sm.rows {
			t1 := float64(r.T)
			t0 := t1 - r.S*1000
			if r.T > refFrom && r.T <= toMs && r.S <= rcSpecGap {
				h := hours[r.T/3600e3]
				if h == nil {
					h = &hour{}
					hours[r.T/3600e3] = h
				}
				h.hi, h.mid, h.tot, h.secs = h.hi+r.Hi, h.mid+r.Mid, h.tot+r.Tot, h.secs+r.S
			}
			k0, k1 := int(math.Floor((t0-x0)/float64(rowMs))), int(math.Floor((t1-x0)/float64(rowMs)))
			for k := max(k0, 0); k <= k1 && k < m; k++ {
				a0 := math.Max(t0, x0+float64(k)*float64(rowMs))
				b0 := math.Min(t1, x0+float64(k+1)*float64(rowMs))
				if b0 <= a0 {
					continue
				}
				f := (b0 - a0) / (t1 - t0)
				hi[k] += r.Hi * f
				mid[k] += r.Mid * f
				tot[k] += r.Tot * f
				secs[k] += (b0 - a0) / 1000
				if j := k - half; r.S > rcSpecGap && j >= 0 && j < n {
					out.Gap[j] = true
				}
			}
		}
	}
	out.First = a.rcSpecFirstRow(names, days)
	// fundo da casa: mediana das horas completas
	var ratios, rates []float64
	var sh, sm, st, ss float64
	for _, h := range hours {
		sh, sm, st, ss = sh+h.hi, sm+h.mid, st+h.tot, ss+h.secs
		if h.secs >= 1800 && h.mid > 0 {
			ratios = append(ratios, h.hi/h.mid)
			rates = append(rates, h.tot/h.secs)
		}
	}
	if len(ratios) >= 3 {
		out.Ref, out.RefCPS = stats.Median(ratios), stats.Median(rates)
	} else if sm > 0 && ss > 0 {
		out.Ref, out.RefCPS = sh/sm, st/ss
	}
	for k := 0; k < n; k++ {
		j := k + half
		if secs[j] < 1 {
			continue
		}
		out.CPS[k] = math.Round(tot[j]/secs[j]*1000) / 1000
		var H, M float64
		for i := j - half; i <= j+half; i++ {
			H, M = H+hi[i], M+mid[i]
		}
		if out.Ref <= 0 || H < 5 || M < 50 {
			continue
		}
		v := H / M / out.Ref
		out.Hard[k] = math.Round(v*1000) / 1000
		out.HardErr[k] = math.Round(v*math.Sqrt(1/H+1/M)*1000) / 1000
	}
	out.Ref = math.Round(out.Ref*1e5) / 1e5
	out.RefCPS = math.Round(out.RefCPS*1000) / 1000
	// chuva (blocos de 10 min)
	for s, r := range a.rainSlots(time.UnixMilli(fromMs), time.UnixMilli(toMs)) {
		t0, t1 := s*1000, (s+int64(rainSlot/time.Second))*1000
		for k := max(int((t0-fromMs)/rowMs), 0); k < n && fromMs+int64(k)*rowMs < t1; k++ {
			if fromMs+int64(k+1)*rowMs <= t0 {
				continue
			}
			out.Rain[k] = math.Max(out.Rain[k], math.Round(r.mmh*10)/10)
			out.Wet[k] = out.Wet[k] || r.wet()
		}
	}
	return out, nil
}

// GetRcWindowPeaks diz com o que se parece o que chegou de fromMs a toMs:
// soma os espectros gravados nesse intervalo e procura as linhas gama acima
// do fundo da casa (a soma dos últimos 7 dias), igual à medição de objeto.
func (a *App) GetRcWindowPeaks(fromMs, toMs int64) []SpecPeak {
	out := []SpecPeak{}
	if a.dataDir == "" || toMs <= fromMs {
		return out
	}
	a.rcSpecFlush()
	names, days := a.rcSpecFiles()
	refFrom := toMs - rcHardDays*24*3600e3
	var bg []float64
	var bgSec float64
	var obj []uint32
	var objSec float64
	var cal [3]float64
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
		if d0 > toMs || d1 < fromMs {
			continue
		}
		for _, r := range a.rcSpecRead(name) {
			t0 := r.T - int64(r.S*1000)
			if r.T <= fromMs || t0 >= toMs || r.S > rcSpecGap {
				continue
			}
			if obj == nil {
				obj = make([]uint32, len(r.C))
			}
			if len(r.C) != len(obj) {
				continue
			}
			for c, v := range r.C {
				obj[c] += v
			}
			objSec += r.S
			cal = r.A
		}
	}
	if obj == nil || len(bg) != len(obj) || objSec <= 0 || bgSec <= 0 {
		return out
	}
	bgc := make([]uint32, len(bg))
	for c, v := range bg {
		bgc[c] = uint32(math.Round(v))
	}
	s := MeasureSpectrum{A: cal, Obj: obj, ObjSec: objSec, Bg: bgc, BgSec: bgSec}
	if p := s.findPeaks(); p != nil {
		out = p
	}
	return out
}
