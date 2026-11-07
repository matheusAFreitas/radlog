package app

import (
	"compress/gzip"
	"encoding/json"
	"math"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"sync"
	"time"

	"radlog/internal/radiacode"
)

// ---------- espectrograma do RadiaCode ----------
//
// A cada minuto, com o RadiaCode conectado, o app lê o espectro atual do
// aparelho e guarda a diferença para a leitura anterior (o que chegou naquele
// minuto), em radiacode/espectros/AAAA-MM-DD-HHMMSS.jsonl.gz (um arquivo por
// dia e por vez que o app abre). Cada linha:
//
//	{"t": fim (ms), "s": segundos, "a": [a0,a1,a2], "c": [contagens por canal]}
//
// A última leitura fica em radiacode/espectros/ultimo.json: se o app fechar
// ou o Bluetooth cair, a próxima linha cobre o buraco todo (o aparelho segue
// contando sozinho), e o espectrograma mostra a média daquele intervalo.

const (
	rcSpecDir   = "espectros"
	rcSpecLast  = "ultimo.json"
	rcSpecEvery = time.Minute
	rcSpecGap   = 3 * 60  // linha mais longa que isso: veio de um intervalo sem leitura
	rcSgBins    = 256     // faixas de energia no espectrograma
	rcSgMaxRows = 2000    // limite de linhas por pedido
	rcSgMaxSpan = 31 * 24 // horas
)

type rcSpecLog struct {
	mu     sync.Mutex
	day    string
	f      *os.File
	gz     *gzip.Writer
	last   *rcSpecSnap
	loaded bool
	at     time.Time // última leitura (só a goroutine do rcRun mexe)
}

// rcSpecSnap é uma leitura do espectro atual do aparelho.
type rcSpecSnap struct {
	Serial string     `json:"serie"`
	T      int64      `json:"t"`
	S      float64    `json:"s"`
	A      [3]float64 `json:"a"`
	C      []uint32   `json:"c"`
}

// rcSpecRow é o que chegou entre duas leituras.
type rcSpecRow struct {
	T int64      `json:"t"`
	S float64    `json:"s"`
	A [3]float64 `json:"a"`
	C []uint32   `json:"c"`
}

func (a *App) rcSpecPath(name string) string { return filepath.Join(a.rcDir(), rcSpecDir, name) }

// rcSpecTick roda a cada volta do rcRun e lê o espectro uma vez por minuto.
func (a *App) rcSpecTick(dev *radiacode.Device) {
	if a.dataDir == "" || time.Since(a.rc.spec.at) < rcSpecEvery {
		return
	}
	a.rc.spec.at = time.Now()
	s, err := dev.Spectrum()
	if err != nil || len(s.Counts) == 0 {
		return
	}
	a.rcSpecAdd(dev.Serial, time.Now(), s)
}

// rcSpecAdd guarda a diferença para a leitura anterior do mesmo aparelho.
func (a *App) rcSpecAdd(serial string, now time.Time, s radiacode.Spectrum) {
	l := &a.rc.spec
	l.mu.Lock()
	defer l.mu.Unlock()
	if !l.loaded {
		l.loaded = true
		if b, err := os.ReadFile(a.rcSpecPath(rcSpecLast)); err == nil {
			var p rcSpecSnap
			if json.Unmarshal(b, &p) == nil && len(p.C) > 0 {
				l.last = &p
			}
		}
	}
	cur := &rcSpecSnap{Serial: serial, T: now.UnixMilli(), S: s.Duration.Seconds(), A: [3]float64{s.A0, s.A1, s.A2}, C: s.Counts}
	prev := l.last
	l.last = cur
	os.MkdirAll(a.rcSpecPath(""), 0o755)
	if b, err := json.Marshal(cur); err == nil {
		tmp := a.rcSpecPath(rcSpecLast + ".tmp")
		if os.WriteFile(tmp, b, 0o644) == nil {
			os.Rename(tmp, a.rcSpecPath(rcSpecLast))
		}
	}
	if prev == nil || prev.Serial != serial {
		return // primeira leitura: só serve de referência
	}
	row := rcSpecDiff(prev, cur)
	if row == nil {
		return
	}
	day := now.Format("2006-01-02")
	if l.day != day || l.gz == nil {
		l.closeLocked()
		f, err := os.OpenFile(a.rcSpecPath(day+"-"+now.Format("150405")+".jsonl.gz"), os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o644)
		if err != nil {
			return
		}
		l.f, l.gz, l.day = f, gzip.NewWriter(f), day
	}
	b, _ := json.Marshal(row)
	l.gz.Write(append(b, '\n'))
	l.gz.Flush() // uma linha por minuto: já fica legível se o app fechar à força
}

func (l *rcSpecLog) closeLocked() {
	if l.gz != nil {
		l.gz.Close()
	}
	if l.f != nil {
		l.f.Close()
	}
	l.f, l.gz = nil, nil
}

// rcSpecDiff é o que chegou entre as duas leituras. Se o espectro do aparelho
// foi zerado no meio, vale o atual inteiro (desde que ele foi zerado).
func rcSpecDiff(prev, cur *rcSpecSnap) *rcSpecRow {
	wall := float64(cur.T-prev.T) / 1000
	if wall <= 0 {
		return nil
	}
	ds := cur.S - prev.S
	reset := ds < 0 || len(cur.C) != len(prev.C)
	c := make([]uint32, len(cur.C))
	if !reset {
		for i, v := range cur.C {
			if v < prev.C[i] {
				reset = true
				break
			}
			c[i] = v - prev.C[i]
		}
	}
	if reset {
		copy(c, cur.C)
		ds = cur.S
	}
	if ds > wall { // relógio do aparelho e do PC não batem exatamente
		ds = wall
	}
	if ds < 1 {
		return nil
	}
	return &rcSpecRow{T: cur.T, S: ds, A: cur.A, C: c}
}

// ---------- leitura ----------

// RcSpectrogram são as linhas do espectrograma: cada uma cobre RowSec
// segundos a partir de From e tem Bins faixas iguais de energia até EMax.
type RcSpectrogram struct {
	From   int64       `json:"from"`
	To     int64       `json:"to"`
	RowSec int         `json:"rowSec"`
	EMax   float64     `json:"eMax"`
	Bins   int         `json:"bins"`
	Rows   [][]float64 `json:"rows"` // contagens por minuto em cada faixa (null = sem dados)
	Secs   []float64   `json:"secs"` // segundos cobertos
	CPS    []float64   `json:"cps"`  // contagem total (todas as energias) por segundo
	Gap    []bool      `json:"gap"`  // veio de um intervalo sem leitura (média dele)
	First  int64       `json:"first"`
}

var rcSpecName = regexp.MustCompile(`^(\d{4}-\d{2}-\d{2})-(\d{6})\.jsonl\.gz$`)

// rcSpecFiles lista os arquivos gravados, com o dia de cada um.
func (a *App) rcSpecFiles() (names []string, days []time.Time) {
	ents, _ := os.ReadDir(a.rcSpecPath(""))
	for _, e := range ents {
		m := rcSpecName.FindStringSubmatch(e.Name())
		if m == nil {
			continue
		}
		d, err := time.ParseInLocation("2006-01-02", m[1], time.Local)
		if err != nil {
			continue
		}
		names, days = append(names, e.Name()), append(days, d)
	}
	return
}

// GetRcSpectrogram monta o espectrograma de fromMs a toMs, com linhas de
// rowSec segundos e energia até eMax keV.
func (a *App) GetRcSpectrogram(fromMs, toMs int64, rowSec int, eMax float64) (RcSpectrogram, error) {
	if rowSec < 10 {
		rowSec = 60
	}
	if eMax < 100 || eMax > 4000 {
		eMax = 1500
	}
	if toMs-fromMs > int64(rcSgMaxSpan)*3600e3 {
		fromMs = toMs - int64(rcSgMaxSpan)*3600e3
	}
	n := int((toMs - fromMs + int64(rowSec)*1000 - 1) / (int64(rowSec) * 1000))
	if n <= 0 {
		return RcSpectrogram{}, nil
	}
	if n > rcSgMaxRows {
		n = rcSgMaxRows
		fromMs = toMs - int64(n*rowSec)*1000
	}
	out := RcSpectrogram{From: fromMs, To: toMs, RowSec: rowSec, EMax: eMax, Bins: rcSgBins,
		Rows: make([][]float64, n), Secs: make([]float64, n), CPS: make([]float64, n), Gap: make([]bool, n)}
	if a.dataDir == "" {
		return out, nil
	}
	a.rc.spec.mu.Lock()
	if a.rc.spec.gz != nil {
		a.rc.spec.gz.Flush()
	}
	a.rc.spec.mu.Unlock()
	names, days := a.rcSpecFiles()
	var rows []rcSpecRow
	for i, name := range names {
		d := days[i]
		// cada arquivo só tem linhas que terminam naquele dia
		if d.UnixMilli() > toMs || d.AddDate(0, 0, 1).UnixMilli() < fromMs {
			continue
		}
		b, err := os.ReadFile(a.rcSpecPath(name))
		if err != nil {
			continue
		}
		for _, ln := range gzipLinesTolerant(b) {
			var r rcSpecRow
			if json.Unmarshal(ln, &r) != nil || r.S <= 0 || len(r.C) == 0 {
				continue
			}
			if r.T <= fromMs || float64(r.T)-r.S*1000 >= float64(toMs) {
				continue
			}
			rows = append(rows, r)
		}
	}
	sort.Slice(rows, func(i, j int) bool { return rows[i].T < rows[j].T })
	out.First = a.rcSpecFirstRow(names, days)
	rcSgFill(&out, rows)
	return out, nil
}

// rcSpecFirstRow é o fim da primeira linha gravada (0 = nenhuma).
func (a *App) rcSpecFirstRow(names []string, days []time.Time) int64 {
	best := -1
	for i := range names {
		if best < 0 || days[i].Before(days[best]) || (days[i].Equal(days[best]) && names[i] < names[best]) {
			best = i
		}
	}
	if best < 0 {
		return 0
	}
	b, err := os.ReadFile(a.rcSpecPath(names[best]))
	if err != nil {
		return 0
	}
	for _, ln := range gzipLinesTolerant(b) {
		var r struct {
			T int64   `json:"t"`
			S float64 `json:"s"`
		}
		if json.Unmarshal(ln, &r) == nil && r.T > 0 {
			return r.T - int64(r.S*1000)
		}
	}
	return 0
}

// rcSgFill espalha cada linha gravada pelas linhas do espectrograma (pelo
// tempo) e pelas faixas de energia (cada canal dividido pela parte da faixa
// que ele cobre, para não aparecerem listras).
func rcSgFill(out *RcSpectrogram, rows []rcSpecRow) {
	n, nb := len(out.Rows), out.Bins
	if n == 0 || nb == 0 {
		return
	}
	acc := make([][]float64, n)
	tot := make([]float64, n)
	bw := out.EMax / float64(nb)
	rowMs := float64(out.RowSec) * 1000
	type split struct {
		bin  int
		frac float64
	}
	var calA [3]float64
	var calSplit [][]split
	for _, r := range rows {
		if calSplit == nil || r.A != calA || len(calSplit) != len(r.C) {
			calA, calSplit = r.A, make([][]split, len(r.C))
			e := func(c float64) float64 { return r.A[0] + r.A[1]*c + r.A[2]*c*c }
			for c := range r.C {
				lo, hi := e(float64(c)), e(float64(c+1))
				if hi <= lo || hi <= 0 || lo >= out.EMax {
					continue
				}
				for b := int(math.Max(lo, 0) / bw); b < nb && float64(b)*bw < hi; b++ {
					ov := math.Min(hi, float64(b+1)*bw) - math.Max(lo, float64(b)*bw)
					if ov > 0 {
						calSplit[c] = append(calSplit[c], split{b, ov / (hi - lo)})
					}
				}
			}
		}
		t1 := float64(r.T)
		t0 := t1 - r.S*1000
		sum := 0.0
		for _, v := range r.C {
			sum += float64(v)
		}
		k0 := int(math.Floor((t0 - float64(out.From)) / rowMs))
		k1 := int(math.Floor((t1 - float64(out.From)) / rowMs))
		for k := max(k0, 0); k <= k1 && k < n; k++ {
			a := math.Max(t0, float64(out.From)+float64(k)*rowMs)
			b := math.Min(t1, float64(out.From)+float64(k+1)*rowMs)
			if b <= a {
				continue
			}
			f := (b - a) / (t1 - t0)
			if acc[k] == nil {
				acc[k] = make([]float64, nb)
			}
			for c, v := range r.C {
				if v == 0 {
					continue
				}
				for _, s := range calSplit[c] {
					acc[k][s.bin] += float64(v) * f * s.frac
				}
			}
			tot[k] += sum * f
			out.Secs[k] += (b - a) / 1000
			if r.S > rcSpecGap {
				out.Gap[k] = true
			}
		}
	}
	for k := range acc {
		if acc[k] == nil || out.Secs[k] < 1 {
			continue
		}
		m := 60 / out.Secs[k]
		row := make([]float64, nb)
		for b, v := range acc[k] {
			row[b] = math.Round(v*m*100) / 100
		}
		out.Rows[k] = row
		out.CPS[k] = math.Round(tot[k]/out.Secs[k]*1000) / 1000
		out.Secs[k] = math.Round(out.Secs[k])
	}
}
