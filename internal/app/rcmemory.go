package app

import (
	"bytes"
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"

	"radlog/internal/radiacode"
)

// ---------- memória do Radiacode: o que ele gravou sem o app (2.30.0) ----------
//
// Sozinho, o Radiacode guarda na memória as últimas ~1000 h: a taxa de dose
// de 96 em 96 s (com as contagens), a dose acumulada, temperatura, bateria e
// os eventos. Quando um app conecta, ele manda esse atraso no mesmo DATA_BUF
// das leituras ao vivo, em lotes, com o bit "dado antigo" (0x8000) em cada
// registro, e apaga da memória o que mandou: quem conectar primeiro (o PC ou
// o celular) leva.
//
// Até a 2.29 o app jogava esses lotes fora e teria posto tudo na hora da
// reconexão. Agora cada registro antigo vai para o seu lugar, com a hora do
// próprio aparelho:
//   - taxa (96 s) → os minutos de radiacode/AAAA-MM.csv que não tinham
//     leitura, com a 5ª coluna "memoria" (a fração do minuto coberta vai no n,
//     em segundos);
//   - taxa, raros e eventos → o log do aparelho, com origem "memoria";
//   - dose acumulada → radiacode/dose-AAAA-MM.csv.
// O que o app já tinha gravado ao vivo nunca é trocado.

const (
	rcMemSrc    = "memoria"
	rcMemStep   = 96 * time.Second    // um ponto da taxa na memória do aparelho
	rcMemMaxAge = 60 * 24 * time.Hour // mais velho que isso é hora errada (a memória guarda ~42 dias)
)

// rcMemTally soma o que veio da memória nesta conexão (o atraso chega em
// várias leituras): vira um evento só quando para de chegar.
type rcMemTally struct {
	recs, mins  int
	first, last time.Time
}

// splitOldRecords separa os registros que vieram da memória do aparelho: os
// marcados como "dado antigo" e os medidos antes de since (o começo da
// conexão). Ao reconectar, o aparelho manda junto o que estava no buffer
// recente, ainda sem a marca: no teste de 10/11, 12:39–13:12 veio marcado e
// 13:14–13:46 não. since zero (ou 1970: fora de uma conexão) = só a marca.
func splitOldRecords(recs []radiacode.Record, since time.Time) (live, old []radiacode.Record) {
	for _, r := range recs {
		isOld := false
		switch v := r.(type) {
		case radiacode.DoseRateDB:
			isOld = v.Old()
		case radiacode.Rare:
			isOld = v.Old()
		case radiacode.Event:
			isOld = v.Old()
		default:
			live = append(live, r)
			continue
		}
		t := rcOldTime(r)
		if _, ok := r.(radiacode.DoseRateDB); ok {
			// o ponto de 96 s fecha em T: o primeiro depois da conexão ainda
			// cobre um pedaço de antes dela (o resto do minuto da conexão)
			t = t.Add(-rcMemStep)
		}
		before := since.Unix() > 0 && !r.When().IsZero() && t.Before(since)
		if isOld || before {
			old = append(old, r)
		} else {
			live = append(live, r)
		}
	}
	return live, old
}

// rcOldTime é a hora de um registro antigo: os tempos do DATA_BUF contam do
// instante em que o app zerou o relógio do aparelho ao conectar (o +128 s
// do início vale só para as leituras ao vivo).
func rcOldTime(r radiacode.Record) time.Time { return r.When().Add(-radiacode.BaseLead) }

// rcRecoverOld grava os registros que vieram da memória do aparelho.
func (a *App) rcRecoverOld(old []radiacode.Record, now time.Time) {
	a.rc.mu.Lock()
	since := time.UnixMilli(a.rc.st.Since)
	a.rc.mu.Unlock()
	n, mins, first, last := a.rcRecoverOldSince(old, now, since)
	if n == 0 {
		return
	}
	a.rc.mu.Lock()
	m := &a.rc.mem
	m.recs += n
	m.mins += mins
	if m.first.IsZero() || first.Before(m.first) {
		m.first = first
	}
	if last.After(m.last) {
		m.last = last
	}
	a.rc.mu.Unlock()
}

// rcRecoverOldSince grava os registros antigos de uma conexão que começou em
// since (só os minutos de antes dela podem ser preenchidos).
func (a *App) rcRecoverOldSince(old []radiacode.Record, now, since time.Time) (n, mins int, first, last time.Time) {
	if since.IsZero() || since.After(now) {
		since = now
	}
	var items []RcLogItem
	var rates []radiacode.DoseRateDB
	var doses []rcDosePt
	for _, r := range old {
		t := rcOldTime(r)
		if t.After(now) || now.Sub(t) > rcMemMaxAge {
			continue
		}
		n++
		if first.IsZero() || t.Before(first) {
			first = t
		}
		if t.After(last) {
			last = t
		}
		if it := rcLogFromRecord(r, t); it != nil {
			it.Src = rcMemSrc
			items = append(items, *it)
		}
		switch v := r.(type) {
		case radiacode.DoseRateDB:
			v.T = t
			rates = append(rates, v)
		case radiacode.Rare:
			doses = append(doses, rcDosePt{T: t, Dose: v.Dose, Sec: float64(v.Duration)})
		}
	}
	if n == 0 {
		return
	}
	a.rcLogMerge(items)
	mins = a.rcMinuteMerge(rcMinutesFromDB(rates, since), since)
	if len(doses) > 0 {
		sort.Slice(doses, func(i, j int) bool { return doses[i].T.Before(doses[j].T) })
		a.rcDoseMerge(doses)
	}
	a.rc.mu.Lock()
	if mins > 0 {
		a.rc.today.loaded = false // a dose de hoje relê os minutos
		a.rc.stats = nil
	}
	a.rc.mu.Unlock()
	if mins > 0 {
		a.firsts.mu.Lock()
		a.firsts.rcKnown = false
		a.firsts.mu.Unlock()
	}
	return n, mins, first, last
}

// rcMemFlush registra no histórico o que veio da memória (uma vez por lote).
func (a *App) rcMemFlush() {
	a.rc.mu.Lock()
	m := a.rc.mem
	a.rc.mem = rcMemTally{}
	a.rc.mu.Unlock()
	if m.recs == 0 {
		return
	}
	f := func(t time.Time) string { return t.Format("02/01 15:04") }
	a.logEvent(fmt.Sprintf("RC102: %d registros recuperados da memória do aparelho (%s a %s), %d minutos sem leitura preenchidos",
		m.recs, f(m.first), f(m.last), m.mins))
}

// rcMemMinute é um minuto montado a partir dos pontos de 96 s.
type rcMemMinute struct {
	t             time.Time
	cps, usv, sec float64 // somas ponderadas pelos segundos cobertos
}

// rcMinutesFromDB espalha cada ponto de 96 s (que fecha em T) pelos minutos
// que ele cobre, pesando pelos segundos de cada um. Só conta o que é de
// antes de until (o começo da conexão; zero = tudo): dali em diante vale a
// leitura ao vivo.
func rcMinutesFromDB(pts []radiacode.DoseRateDB, until time.Time) []rcMemMinute {
	by := map[int64]*rcMemMinute{}
	for _, p := range pts {
		if p.CPS < 0 || p.DoseRate < 0 || math.IsNaN(p.CPS) || math.IsNaN(p.DoseRate) {
			continue
		}
		from, to := p.T.Add(-rcMemStep), p.T
		if !until.IsZero() && to.After(until) {
			to = until
		}
		for m := from.Truncate(time.Minute); m.Before(to); m = m.Add(time.Minute) {
			a, b := m, m.Add(time.Minute)
			if from.After(a) {
				a = from
			}
			if to.Before(b) {
				b = to
			}
			s := b.Sub(a).Seconds()
			if s <= 0 {
				continue
			}
			k := m.Unix()
			mm := by[k]
			if mm == nil {
				mm = &rcMemMinute{t: m}
				by[k] = mm
			}
			mm.cps += p.CPS * s
			mm.usv += p.DoseRate * s
			mm.sec += s
		}
	}
	out := make([]rcMemMinute, 0, len(by))
	for _, m := range by {
		out = append(out, *m)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].t.Before(out[j].t) })
	return out
}

func (m rcMemMinute) csvLine() string {
	n := int(math.Round(m.sec))
	if n < 1 {
		n = 1
	}
	return fmt.Sprintf("%s,%.3f,%.5f,%d,%s", m.t.Format(time.RFC3339), m.cps/m.sec, m.usv/m.sec, n, rcMemSrc)
}

// rcMinuteMerge põe os minutos que vieram da memória nos arquivos do mês,
// antes de until (o começo desta conexão), em ordem. Um minuto sem leitura
// entra inteiro; um que já tinha só um pedaço (o fim de um lote, ou o minuto
// da conexão, com os segundos ao vivo) é completado, pesando pelos segundos,
// desde que a soma caiba no minuto. Devolve quantos entraram.
func (a *App) rcMinuteMerge(mins []rcMemMinute, until time.Time) int {
	byMonth := map[string][]rcMemMinute{}
	for _, m := range mins {
		if m.sec <= 0 || !m.t.Before(until) {
			continue
		}
		n := m.t.Format("2006-01") + ".csv"
		byMonth[n] = append(byMonth[n], m)
	}
	if len(byMonth) == 0 || a.dataDir == "" {
		return 0
	}
	a.rc.fileM.Lock()
	defer a.rc.fileM.Unlock()
	_ = os.MkdirAll(a.rcDir(), 0o755)
	added := 0
	for name, add := range byMonth {
		path := filepath.Join(a.rcDir(), name)
		type line struct {
			t    int64
			s    string
			drop bool
		}
		var lines []line
		have := map[int64]*rcMemMinute{} // o que já está gravado em cada minuto
		idx := map[int64][]int{}
		var bad rcBad
		if b, err := os.ReadFile(path); err == nil {
			sc := rcScanner(bytes.NewReader(b))
			for sc.Scan() {
				l := sc.Text()
				f := strings.Split(l, ",")
				t, err := time.Parse(time.RFC3339, f[0])
				if err != nil {
					bad.add(l) // cabeçalho fica de fora sozinho
					continue
				}
				m := t.Truncate(time.Minute)
				k := m.Unix()
				h := have[k]
				if h == nil {
					h = &rcMemMinute{t: m}
					have[k] = h
				}
				if len(f) >= 4 {
					cps, e1 := strconv.ParseFloat(f[1], 64)
					usv, e2 := strconv.ParseFloat(f[2], 64)
					n, e3 := strconv.Atoi(f[3])
					if e1 == nil && e2 == nil && e3 == nil && n > 0 {
						h.cps += cps * float64(n)
						h.usv += usv * float64(n)
						h.sec += float64(n)
					} else {
						h.sec = 60 // linha que não dá para somar: fica como está
					}
				} else {
					h.sec = 60
				}
				idx[k] = append(idx[k], len(lines))
				lines = append(lines, line{t: t.Unix(), s: l})
			}
		}
		n := 0
		for _, m := range add {
			k := m.t.Unix()
			h := have[k]
			if h == nil {
				have[k] = &rcMemMinute{t: m.t, cps: m.cps, usv: m.usv, sec: m.sec}
				lines = append(lines, line{t: k, s: m.csvLine()})
				n++
				continue
			}
			if h.sec >= 59.5 || h.sec+m.sec > 61 {
				continue // já está cheio (ou isso já entrou)
			}
			h.cps += m.cps
			h.usv += m.usv
			h.sec += m.sec
			for _, i := range idx[k] {
				lines[i].drop = true
			}
			idx[k] = []int{len(lines)}
			lines = append(lines, line{t: k, s: h.csvLine()})
			n++
		}
		if n == 0 {
			continue
		}
		sort.SliceStable(lines, func(i, j int) bool { return lines[i].t < lines[j].t })
		var buf bytes.Buffer
		buf.WriteString(rcHeader + "\n")
		for _, l := range lines {
			if !l.drop {
				buf.WriteString(l.s + "\n")
			}
		}
		if writeFileAtomic(path, buf.Bytes(), 0o644) == nil {
			added += n
			bad.save(a, path)
		}
	}
	return added
}

// ---------- reposição a partir dos dados brutos (2.34.1, uma vez) ----------
//
// Até a 2.34.0 só ia para a hora certa o que vinha com a marca de "dado
// antigo"; o resto do atraso (o buffer recente do aparelho) entrava como se
// fosse ao vivo, na hora da reconexão. Os dados brutos guardam tudo: na
// primeira vez que abre, o app relê as leituras dos 2 primeiros minutos de
// cada conexão, põe o que foi medido antes dela na hora certa e tira do log
// as linhas que tinham caído na hora da reconexão.

const (
	rcMemRawMark   = "memoria-reposta"
	rcMemRawWindow = 2 * time.Minute
)

// rcConnStarts são os começos de conexão gravados nos dados brutos.
func (a *App) rcConnStarts() []time.Time {
	files, _ := filepath.Glob(a.rcRawPath("conexao-*.json"))
	var out []time.Time
	for _, f := range files {
		n := strings.TrimSuffix(strings.TrimPrefix(filepath.Base(f), "conexao-"), ".json")
		if t, err := time.ParseInLocation("20060102-150405", n, time.Local); err == nil {
			out = append(out, t)
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Before(out[j]) })
	return out
}

// rcRawRead é uma resposta do DATA_BUF guardada nos dados brutos.
type rcRawRead struct {
	pc   time.Time
	base time.Time
	buf  []byte
}

// rcRawReads remonta as respostas (as linhas com o mesmo "pc"). A base de
// tempo sai do primeiro registro: t = base + deslocamento × 10 ms.
func rcRawReads(lines [][]byte) []rcRawRead {
	var out []rcRawRead
	for _, ln := range lines {
		var r struct {
			PC  int64  `json:"pc"`
			T   int64  `json:"t"`
			Hex string `json:"hex"`
		}
		if json.Unmarshal(ln, &r) != nil || r.Hex == "" || r.PC == 0 {
			continue
		}
		h, err := hex.DecodeString(r.Hex)
		if err != nil {
			continue
		}
		if n := len(out); n == 0 || out[n-1].pc.UnixMilli() != r.PC {
			if len(h) < 7 {
				continue
			}
			off := int32(binary.LittleEndian.Uint32(h[3:7]))
			base := time.UnixMilli(r.T).Add(-time.Duration(off) * 10 * time.Millisecond)
			out = append(out, rcRawRead{pc: time.UnixMilli(r.PC), base: base})
		}
		out[len(out)-1].buf = append(out[len(out)-1].buf, h...)
	}
	return out
}

// rcMemRawBackfill faz a reposição (uma vez).
func (a *App) rcMemRawBackfill() {
	if a.dataDir == "" {
		return
	}
	mark := filepath.Join(a.rcDir(), rcMemRawMark)
	if _, err := os.Stat(mark); err == nil {
		return
	}
	starts := a.rcConnStarts()
	files, _ := filepath.Glob(a.rcRawPath("*.jsonl.gz"))
	sort.Strings(files)
	total, mins := 0, 0
	var drop []RcLogItem
	for _, f := range files {
		b, err := os.ReadFile(f)
		if err != nil {
			continue
		}
		for _, rd := range rcRawReads(gzipLinesTolerant(b)) {
			i := sort.Search(len(starts), func(i int) bool { return starts[i].After(rd.pc) }) - 1
			if i < 0 || rd.pc.Sub(starts[i]) > rcMemRawWindow {
				continue
			}
			_, old := splitOldRecords(radiacode.DecodeDataBuf(rd.buf, rd.base), starts[i])
			if len(old) == 0 {
				continue
			}
			for _, r := range old {
				// a linha que a versão antiga gravou na hora da reconexão
				if it := rcLogFromRecord(r, rd.pc); it != nil {
					drop = append(drop, *it)
				}
			}
			n, m, _, _ := a.rcRecoverOldSince(old, rd.pc, starts[i])
			total += n
			mins += m
		}
	}
	nd := a.rcLogDropNear(drop, 5*time.Second)
	_ = os.MkdirAll(a.rcDir(), 0o755)
	_ = writeFileAtomic(mark, []byte(time.Now().Format(time.RFC3339)+"\n"), 0o644)
	if total > 0 {
		a.logf("RC102: %d registros da memória do aparelho repostos dos dados brutos (%d minutos preenchidos, %d linhas do log na hora errada retiradas)", total, mins, nd)
	}
}

// rcLogDropNear tira do log as linhas iguais às de items (menos a origem),
// com hora até tol de distância, que não vieram da memória.
func (a *App) rcLogDropNear(items []RcLogItem, tol time.Duration) int {
	if len(items) == 0 {
		return 0
	}
	key := func(it RcLogItem) string {
		it.Src, it.T = "", 0
		return it.csvLine()
	}
	want := map[string][]int64{}
	byFile := map[string]bool{}
	for _, it := range items {
		want[key(it)] = append(want[key(it)], it.T)
		byFile[rcLogFileName(time.UnixMilli(it.T))] = true
	}
	a.rc.fileM.Lock()
	defer a.rc.fileM.Unlock()
	n := 0
	for name := range byFile {
		path := filepath.Join(a.rcDir(), name)
		b, err := os.ReadFile(path)
		if err != nil {
			continue
		}
		var out bytes.Buffer
		out.WriteString(rcLogHeader + "\n")
		removed := 0
		var bad rcBad
		sc := rcScanner(bytes.NewReader(b))
		for sc.Scan() {
			l := sc.Text()
			it, ok := parseRcLogLine(l)
			if !ok {
				bad.add(l)
				continue
			}
			if it.Src != rcMemSrc {
				hit := false
				for _, t := range want[key(it)] {
					if d := it.T - t; d > -tol.Milliseconds() && d < tol.Milliseconds() {
						hit = true
						break
					}
				}
				if hit {
					removed++
					continue
				}
			}
			out.WriteString(l + "\n")
		}
		if removed == 0 {
			continue
		}
		if writeFileAtomic(path, out.Bytes(), 0o644) == nil {
			n += removed
			bad.save(a, path)
		}
	}
	return n
}
