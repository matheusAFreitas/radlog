package app

import (
	"bufio"
	"bytes"
	"compress/flate"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"

	"radlog/internal/radiacode"
)

// ---------- histórico da dose do próprio RadiaCode ----------
//
// Cada total que o aparelho manda (RareData, a cada 1 a 10 min) fica em
// radiacode/dose-AAAA-MM.csv (hora,dose µSv,segundos contando). Com isso o
// gráfico da dose do RadiaCode usa o número do aparelho, que continua
// contando mesmo desconectado: o que ele somou enquanto estava longe do PC
// aparece quando volta.

const rcDoseHeader = "hora,dose,seg"

// rcDosePt é um total do aparelho.
type rcDosePt struct {
	T    time.Time
	Dose float64 // µSv desde que foi zerado
	Sec  float64 // s contando
}

func rcDoseFileName(t time.Time) string { return "dose-" + t.Format("2006-01") + ".csv" }

// rcDoseLogAppend grava um total (se mudou desde o último gravado).
func (a *App) rcDoseLogAppend(t time.Time, dose, sec float64) {
	if a.dataDir == "" {
		return
	}
	a.rc.fileM.Lock()
	defer a.rc.fileM.Unlock()
	if l := a.rc.doseLast; !l.T.IsZero() && l.Dose == dose && sec-l.Sec < 600 {
		return // igual ao último (a cada ~10 min grava mesmo assim, para o tempo andar)
	}
	a.rc.doseLast = rcDosePt{t, dose, sec}
	dir := a.rcDir()
	_ = os.MkdirAll(dir, 0o755)
	path := filepath.Join(dir, rcDoseFileName(t))
	_, statErr := os.Stat(path)
	f, err := os.OpenFile(path, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o644)
	if err != nil {
		return
	}
	defer f.Close()
	if statErr != nil {
		fmt.Fprintln(f, rcDoseHeader)
	}
	fmt.Fprintf(f, "%s,%.4f,%.0f\n", t.Format(time.RFC3339), dose, sec)
}

// rcDoseLog lê os totais gravados entre from e to (em ordem, sem repetir).
func (a *App) rcDoseLog(from, to time.Time) []rcDosePt {
	a.rc.fileM.Lock()
	defer a.rc.fileM.Unlock()
	var out []rcDosePt
	for m := time.Date(from.Year(), from.Month(), 1, 0, 0, 0, 0, from.Location()); !m.After(to); m = m.AddDate(0, 1, 0) {
		b, err := os.ReadFile(filepath.Join(a.rcDir(), rcDoseFileName(m)))
		if err != nil {
			continue
		}
		for _, ln := range strings.Split(string(b), "\n") {
			p := strings.Split(strings.TrimSpace(ln), ",")
			if len(p) < 3 {
				continue
			}
			t, err := time.Parse(time.RFC3339, p[0])
			if err != nil || t.Before(from) || t.After(to) {
				continue
			}
			d, _ := strconv.ParseFloat(p[1], 64)
			s, _ := strconv.ParseFloat(p[2], 64)
			out = append(out, rcDosePt{t, d, s})
		}
	}
	sort.SliceStable(out, func(i, j int) bool { return out[i].T.Before(out[j].T) })
	dedup := out[:0]
	for _, p := range out {
		if n := len(dedup); n > 0 && dedup[n-1].T.Equal(p.T) {
			continue
		}
		dedup = append(dedup, p)
	}
	return dedup
}

// rcDoseCurve monta o "contador contínuo" a partir dos totais: soma só o que
// subiu (se o aparelho foi zerado, o total cai e a conta continua dali).
// Devolve os pontos com a dose acumulada desde o primeiro.
func rcDoseCurve(pts []rcDosePt) []rcDosePt {
	out := make([]rcDosePt, 0, len(pts))
	var v float64
	for i, p := range pts {
		if i > 0 {
			d := p.Dose - pts[i-1].Dose
			if d < 0 { // zerado no meio: conta o que veio depois do zero
				d = p.Dose
			}
			v += d
		}
		out = append(out, rcDosePt{p.T, v, p.Sec})
	}
	return out
}

// curveAt é o valor da curva no instante t (reta entre os pontos; antes do
// primeiro, o primeiro; depois do último, o último).
func curveAt(c []rcDosePt, t time.Time) float64 {
	if len(c) == 0 {
		return 0
	}
	i := sort.Search(len(c), func(i int) bool { return !c[i].T.Before(t) })
	switch {
	case i == 0:
		return c[0].Dose
	case i == len(c):
		return c[len(c)-1].Dose
	}
	a, b := c[i-1], c[i]
	f := float64(t.Sub(a.T)) / float64(b.T.Sub(a.T))
	return a.Dose + (b.Dose-a.Dose)*f
}

// ---------- recupera os totais dos dados brutos (uma vez) ----------

// rcDoseBackfill lê os dados brutos já gravados (radiacode/bruto) e grava os
// totais do aparelho que ainda não estão no histórico. Roda uma vez.
func (a *App) rcDoseBackfill() {
	if a.dataDir == "" {
		return
	}
	mark := filepath.Join(a.rcDir(), "dose-recuperada")
	if _, err := os.Stat(mark); err == nil {
		return
	}
	files, _ := filepath.Glob(filepath.Join(a.rcDir(), rcRawDir, "*.jsonl.gz"))
	var pts []rcDosePt
	for _, fn := range files {
		b, err := os.ReadFile(fn)
		if err != nil {
			continue
		}
		pts = append(pts, rcRareFromRaw(gzipLinesTolerant(b))...)
	}
	if len(pts) > 0 {
		sort.Slice(pts, func(i, j int) bool { return pts[i].T.Before(pts[j].T) })
		a.rcDoseMerge(pts)
	}
	_ = os.WriteFile(mark, []byte(time.Now().Format(time.RFC3339)+"\n"), 0o644)
	if len(pts) > 0 {
		a.logf("RadiaCode: %d totais de dose recuperados dos dados brutos", len(pts))
	}
}

// rcDoseMerge junta totais ao histórico (sem repetir a hora).
func (a *App) rcDoseMerge(pts []rcDosePt) {
	byMonth := map[string][]rcDosePt{}
	for _, p := range pts {
		byMonth[rcDoseFileName(p.T)] = append(byMonth[rcDoseFileName(p.T)], p)
	}
	a.rc.fileM.Lock()
	defer a.rc.fileM.Unlock()
	_ = os.MkdirAll(a.rcDir(), 0o755)
	for name, add := range byMonth {
		path := filepath.Join(a.rcDir(), name)
		have := map[string]string{}
		if b, err := os.ReadFile(path); err == nil {
			for _, ln := range strings.Split(string(b), "\n") {
				if k, _, ok := strings.Cut(ln, ","); ok && k != "hora" {
					have[k] = ln
				}
			}
		}
		for _, p := range add {
			k := p.T.Format(time.RFC3339)
			if _, ok := have[k]; !ok {
				have[k] = fmt.Sprintf("%s,%.4f,%.0f", k, p.Dose, p.Sec)
			}
		}
		keys := make([]string, 0, len(have))
		for k := range have {
			keys = append(keys, k)
		}
		sort.Slice(keys, func(i, j int) bool {
			ti, _ := time.Parse(time.RFC3339, keys[i])
			tj, _ := time.Parse(time.RFC3339, keys[j])
			return ti.Before(tj)
		})
		var buf bytes.Buffer
		buf.WriteString(rcDoseHeader + "\n")
		for _, k := range keys {
			buf.WriteString(have[k] + "\n")
		}
		tmp := path + ".tmp"
		if os.WriteFile(tmp, buf.Bytes(), 0o644) == nil {
			_ = os.Rename(tmp, path)
		}
	}
}

// gzipLinesTolerant lê um .gz feito de vários pedaços emendados, mesmo com
// pedaços sem o fim (app fechado à força): procura cada cabeçalho gzip e lê
// o que der de cada um. Só devolve linhas completas.
func gzipLinesTolerant(b []byte) [][]byte {
	var lines [][]byte
	for off := 0; off+10 < len(b); {
		i := bytes.Index(b[off:], []byte{0x1f, 0x8b, 0x08})
		if i < 0 {
			break
		}
		start := off + i
		off = start + 3
		if b[start+3] != 0 { // o app nunca grava nome nem campos extras
			continue
		}
		data, _ := io.ReadAll(flate.NewReader(bytes.NewReader(b[start+10:])))
		if k := bytes.LastIndexByte(data, '\n'); k >= 0 {
			sc := bufio.NewScanner(bytes.NewReader(data[:k+1]))
			sc.Buffer(make([]byte, 1<<20), 1<<24)
			for sc.Scan() {
				lines = append(lines, append([]byte(nil), sc.Bytes()...))
			}
		}
	}
	return lines
}

// rcRareFromRaw remonta cada resposta do DATA_BUF (as linhas com o mesmo
// "pc") e tira os totais de dose.
func rcRareFromRaw(lines [][]byte) []rcDosePt {
	var out []rcDosePt
	var cur int64 = -1
	var buf []byte
	seen := map[int64]bool{}
	flush := func() {
		if len(buf) == 0 || seen[cur] {
			buf = nil
			return
		}
		seen[cur] = true
		for _, r := range radiacode.DecodeDataBuf(buf, time.Unix(0, 0)) {
			if v, ok := r.(radiacode.Rare); ok {
				out = append(out, rcDosePt{time.UnixMilli(cur), v.Dose, float64(v.Duration)})
			}
		}
		buf = nil
	}
	for _, ln := range lines {
		var r struct {
			PC  int64  `json:"pc"`
			Hex string `json:"hex"`
		}
		if json.Unmarshal(ln, &r) != nil || r.Hex == "" || r.PC == 0 {
			continue
		}
		if r.PC != cur {
			flush()
			cur = r.PC
		}
		if h, err := hex.DecodeString(r.Hex); err == nil {
			buf = append(buf, h...)
		}
	}
	flush()
	return out
}
