package app

import (
	"bufio"
	"bytes"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"radlog/internal/radiacode"
)

// ---------- memória do RadiaCode: o que ele gravou sem o app (2.30.0) ----------
//
// Sozinho, o RadiaCode guarda na memória as últimas ~1000 h: a taxa de dose
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

// splitOldRecords separa os registros que vieram da memória do aparelho.
func splitOldRecords(recs []radiacode.Record) (live, old []radiacode.Record) {
	for _, r := range recs {
		isOld := false
		switch v := r.(type) {
		case radiacode.DoseRateDB:
			isOld = v.Old()
		case radiacode.Rare:
			isOld = v.Old()
		case radiacode.Event:
			isOld = v.Old()
		}
		if isOld {
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
	if since.IsZero() || since.After(now) {
		since = now
	}
	var items []RcLogItem
	var rates []radiacode.DoseRateDB
	var doses []rcDosePt
	var first, last time.Time
	n := 0
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
	mins := a.rcMinuteMerge(rcMinutesFromDB(rates), since.Truncate(time.Minute))
	if len(doses) > 0 {
		sort.Slice(doses, func(i, j int) bool { return doses[i].T.Before(doses[j].T) })
		a.rcDoseMerge(doses)
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
	a.logEvent(fmt.Sprintf("RadiaCode: %d registros recuperados da memória do aparelho (%s a %s), %d minutos sem leitura preenchidos",
		m.recs, f(m.first), f(m.last), m.mins))
}

// rcMemMinute é um minuto montado a partir dos pontos de 96 s.
type rcMemMinute struct {
	t             time.Time
	cps, usv, sec float64 // somas ponderadas pelos segundos cobertos
}

// rcMinutesFromDB espalha cada ponto de 96 s (que fecha em T) pelos minutos
// que ele cobre, pesando pelos segundos de cada um.
func rcMinutesFromDB(pts []radiacode.DoseRateDB) []rcMemMinute {
	by := map[int64]*rcMemMinute{}
	for _, p := range pts {
		if p.CPS < 0 || p.DoseRate < 0 || math.IsNaN(p.CPS) || math.IsNaN(p.DoseRate) {
			continue
		}
		from, to := p.T.Add(-rcMemStep), p.T
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
// só onde não havia leitura e antes de until (o começo desta conexão), em
// ordem. Devolve quantos entraram.
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
			t int64
			s string
		}
		var lines []line
		have := map[int64]bool{}
		if b, err := os.ReadFile(path); err == nil {
			sc := bufio.NewScanner(bytes.NewReader(b))
			for sc.Scan() {
				l := sc.Text()
				k, _, ok := strings.Cut(l, ",")
				if !ok {
					continue
				}
				t, err := time.Parse(time.RFC3339, k)
				if err != nil {
					continue // cabeçalho
				}
				m := t.Truncate(time.Minute).Unix()
				have[m] = true
				lines = append(lines, line{t.Unix(), l})
			}
		}
		n := 0
		for _, m := range add {
			if k := m.t.Unix(); !have[k] {
				have[k] = true
				lines = append(lines, line{k, m.csvLine()})
				n++
			}
		}
		if n == 0 {
			continue
		}
		sort.SliceStable(lines, func(i, j int) bool { return lines[i].t < lines[j].t })
		var buf bytes.Buffer
		buf.WriteString(rcHeader + "\n")
		for _, l := range lines {
			buf.WriteString(l.s + "\n")
		}
		tmp := path + ".tmp"
		if os.WriteFile(tmp, buf.Bytes(), 0o644) == nil && os.Rename(tmp, path) == nil {
			added += n
		}
	}
	return added
}
