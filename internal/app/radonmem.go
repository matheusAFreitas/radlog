package app

// Histórico da memória do GQ RadonScan (2.106.0).
//
// O RadonScan guarda quantas contagens teve em cada hora (ver
// radonscan/memory.go), mas não diz que hora é cada entrada. O app descobre
// comparando a tabela com as médias de 1 hora que ele mesmo gravou em
// radonscan.csv: procura o deslocamento (qual entrada é qual hora) e o
// minuto em que a hora do aparelho vira. Só usa o resultado quando um
// encaixe só bate com folga. Depois preenche as horas em que o app não
// estava lendo (PC desligado, app fechado, aparelho levado pra outro
// cômodo) em radonscan-memoria.csv, sem mexer no radonscan.csv.
//
// 2.106.2: o relógio do aparelho para quando ele fica sem energia, então
// cada trecho de leitura ao vivo é encaixado sozinho (radonFill), e as
// horas de quando ele ficou fora do PC entram logo que ele reconecta.
//
// Baixa sozinho quando o aparelho reconecta depois de um buraco (e tenta
// de novo a cada hora, até 12 vezes, se ainda não deu pra encaixar) e pelo
// botão "Baixar da memória do aparelho".

import (
	"bufio"
	"errors"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"

	"radlog/internal/radonscan"
)

const (
	radonGapMin     = 70 * time.Minute // sem linha por mais que isso = buraco
	radonMemTries   = 12
	radonMemRetry   = time.Hour
	radonMemHeader  = "inicio,fim,comodo,contagens,media_1h_pCiL"
	radonAlignDays  = 14
	radonAlignMinNZ = 3 // horas com contagem que precisam bater
)

// RadonMemResult é o resultado da última leitura da memória (vai pra tela).
type RadonMemResult struct {
	At    int64   `json:"at"`
	Hours int     `json:"hours"` // horas novas preenchidas
	From  int64   `json:"from"`  // início da primeira hora nova
	To    int64   `json:"to"`    // fim da última hora nova
	Mean  float64 `json:"mean"`  // média das horas novas, pCi/L
	Err   string  `json:"err"`   // por que não deu (texto pra tela)
}

// RadonGap é um buraco recente sem leitura que a memória pode cobrir.
type RadonGap struct {
	From int64 `json:"from"`
	To   int64 `json:"to"`
}

func (a *App) radonMemPath() string { return filepath.Join(a.dataDir, "radonscan-memoria.csv") }

// radonAlignment diz qual entrada da tabela é qual hora: a hora que termina
// em e (e = phase + k·3600 s, em Unix) fica na entrada shift + k.
type radonAlignment struct {
	phase int64 // segundos, 0..3599
	shift int64
}

func (al radonAlignment) index(end time.Time) int64 {
	return al.shift + (end.Unix()-al.phase)/3600
}

func (al radonAlignment) end(idx int64) time.Time {
	return time.Unix((idx-al.shift)*3600+al.phase, 0)
}

// radonValueAt é a média de 1 hora que o app leu ao vivo no instante t
// (ok = o aparelho estava conectado: há linha antes e depois, a menos de
// uma hora uma da outra).
func radonValueAt(rows []RadonRow, t int64) (float64, bool) {
	i := sort.Search(len(rows), func(i int) bool { return rows[i].T > t })
	if i == 0 || i == len(rows) {
		return 0, false
	}
	p, n := rows[i-1], rows[i]
	if n.T-p.T > int64(65*time.Minute/time.Millisecond) {
		return 0, false
	}
	return p.Hour, true
}

// radonCand é um encaixe possível: a hora que termina em e (Unix) fica na
// entrada (e + o)/3600, com o = s·3600 − ph. Encaixes com o perto um do
// outro são o mesmo encaixe visto em minutos vizinhos.
type radonCand struct {
	s, ph, o int64
	score    float64
}

// radonNewest é a entrada da hora mais nova já fechada (ver
// radonscan.HourNewest).
func radonNewest(hours []int) int64 {
	return int64(min(len(hours)-1, radonscan.HourNewest))
}

// radonCands compara a tabela com as médias de 1 hora que o app leu ao vivo
// entre from e to e devolve os encaixes que batem.
func radonCands(hours []int, rows []RadonRow, from, to time.Time) []radonCand {
	type sample struct {
		k int64
		v int // centésimos de pCi/L
	}
	n := radonNewest(hours) + 1
	var cs []radonCand
	for ph := int64(0); ph < 3600; ph += 60 {
		var ss []sample
		kTo := (to.Unix() - ph) / 3600
		for k := kTo; ; k-- {
			e := k*3600 + ph
			if time.Unix(e-3600, 0).Before(from) {
				break
			}
			if e > to.Unix() {
				continue
			}
			if v, ok := radonValueAt(rows, e*1000); ok {
				ss = append(ss, sample{k, int(math.Round(v * 100))})
			}
		}
		if len(ss) < radonAlignMinNZ {
			continue
		}
		for s := -kTo; s+kTo < n; s++ {
			var score float64
			nz, bad := 0, 0
			for _, x := range ss {
				idx := s + x.k
				if idx < 0 || idx >= n {
					bad++
					continue
				}
				got := int(math.Round(float64(hours[idx]) * radonscan.PCiPerCount * 100))
				d := got - x.v
				if d < 0 {
					d = -d
				}
				switch {
				case d <= 1 && x.v > 0:
					nz++
					score++
				case d <= 1:
					score += 0.25
				default:
					bad++
					score -= 2
				}
			}
			if nz < radonAlignMinNZ || float64(bad) > 0.2*float64(len(ss)) {
				continue
			}
			cs = append(cs, radonCand{s, ph, s*3600 - ph, score})
		}
	}
	return cs
}

// radonPick escolhe o encaixe de nota maior com o entre oMin e oMax. Recusa
// quando outro encaixe, a uma hora ou mais de distância, bate quase igual.
func radonPick(all []radonCand, oMin, oMax int64) (radonAlignment, int64, error) {
	var cs []radonCand
	for _, c := range all {
		if c.o >= oMin && c.o <= oMax {
			cs = append(cs, c)
		}
	}
	if len(cs) == 0 {
		return radonAlignment{}, 0, errRadonNoFit
	}
	sort.SliceStable(cs, func(i, j int) bool { return cs[i].score > cs[j].score })
	best := cs[0]
	for _, c := range cs[1:] {
		if abs64(c.o-best.o) >= 3600 && best.score-c.score < 2 {
			return radonAlignment{}, 0, errRadonNoFit // outro encaixe bate quase igual
		}
	}
	// o minuto da virada: o do meio entre os de nota máxima desse encaixe
	var same []radonCand
	for _, c := range cs {
		if c.score == best.score && abs64(c.o-best.o) < 3600 {
			same = append(same, c)
		}
	}
	sort.Slice(same, func(i, j int) bool { return same[i].o < same[j].o })
	m := same[len(same)/2]
	return radonAlignment{phase: m.ph, shift: m.s}, m.o, nil
}

// radonOMin: o aparelho não conta mais tempo ligado do que passou no
// relógio, então a hora mais nova da tabela não pode ser mais nova que
// agora (folga de 10 min pro minuto da virada).
func radonOMin(hours []int, now time.Time) int64 {
	return radonNewest(hours)*3600 - (now.Unix() + 600)
}

// radonAlign encaixa a tabela nas leituras ao vivo entre since e now
// (since zero = os últimos radonAlignDays dias), como um trecho só.
func radonAlign(hours []int, rows []RadonRow, now, since time.Time) (radonAlignment, error) {
	from := now.Add(-radonAlignDays * 24 * time.Hour)
	if since.After(from) {
		from = since
	}
	al, _, err := radonPick(radonCands(hours, rows, from, now), radonOMin(hours, now), math.MaxInt64)
	return al, err
}

func abs64(x int64) int64 {
	if x < 0 {
		return -x
	}
	return x
}

var errRadonNoFit = errors.New("não deu pra encaixar a memória nas leituras do app")

// radonStretch é um trecho de leituras ao vivo sem buraco. O relógio do
// aparelho é o tempo que ele ficou ligado: cada vez que ele fica sem
// energia (o RadonScan não tem bateria), a conta para e a virada da hora
// muda. Por isso cada trecho tem o seu encaixe.
type radonStretch struct {
	from, to int64 // ms: primeira e última leitura
	rows     []RadonRow
	al       radonAlignment
	ok       bool
}

func radonStretches(live []RadonRow, from time.Time) []radonStretch {
	gap := radonGapMin.Milliseconds()
	var out []radonStretch
	for _, r := range live {
		if r.T < from.UnixMilli() {
			continue
		}
		if n := len(out); n > 0 && r.T-out[n-1].to <= gap {
			out[n-1].to = r.T
			out[n-1].rows = append(out[n-1].rows, r)
			continue
		}
		out = append(out, radonStretch{from: r.T, to: r.T, rows: []RadonRow{r}})
	}
	return out
}

// radonOn é o tempo ligado do aparelho lido ao conectar
// (radonscan-ligado.csv).
type radonOn struct {
	T  int64 // ms
	Up int   // segundos
}

func (a *App) radonOnPath() string { return filepath.Join(a.dataDir, "radonscan-ligado.csv") }

func (a *App) radonOnLog(t time.Time, up int) {
	path := a.radonOnPath()
	_, statErr := os.Stat(path)
	f, err := os.OpenFile(path, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o644)
	if err != nil {
		a.logf("radonscan: não gravei o tempo ligado: %v", err)
		return
	}
	defer f.Close()
	if os.IsNotExist(statErr) {
		f.WriteString("hora,segundos_ligado\n")
	}
	f.WriteString(t.Format(time.RFC3339) + "," + strconv.Itoa(up) + "\n")
}

func (a *App) radonOns() []radonOn {
	b, err := os.ReadFile(a.radonOnPath())
	if err != nil {
		return nil
	}
	var out []radonOn
	for _, ln := range strings.Split(string(b), "\n") {
		f := strings.Split(strings.TrimSpace(ln), ",")
		if len(f) != 2 {
			continue
		}
		t, err1 := time.Parse(time.RFC3339, f[0])
		up, err2 := strconv.Atoi(f[1])
		if err1 == nil && err2 == nil && up >= 0 {
			out = append(out, radonOn{t.UnixMilli(), up})
		}
	}
	sort.SliceStable(out, func(i, j int) bool { return out[i].T < out[j].T })
	return out
}

// radonHoursSince: quantas horas o aparelho fechou desde a conexão que
// começou o trecho de agora (em S), pelo tempo ligado gravado ao conectar.
func radonHoursSince(ons []radonOn, S time.Time, upNow int) (int64, bool) {
	if upNow < 0 {
		return 0, false
	}
	for _, o := range ons {
		if d := o.T - S.UnixMilli(); d >= -2*60000 && d <= 15*60000 {
			if o.Up > upNow {
				return 0, false
			}
			return int64((upNow - o.Up) / 3600), true
		}
	}
	return 0, false
}

// radonFillInfo conta o que deu pra encaixar (vai pro log e pra mensagem).
type radonFillInfo struct {
	stretches, aligned int
	pending            bool // há buraco que só dá pra encaixar com mais horas ao vivo
	unpowered          int  // buracos deixados de fora: o aparelho ficou sem energia no meio
	notes              []string
}

// radonFill escolhe as horas da memória que entram nos buracos entre os
// trechos de leitura ao vivo:
//
//   - cada trecho é encaixado sozinho, do mais velho pro mais novo; um trecho
//     não pode estar adiantado em relação ao anterior (o aparelho não conta
//     mais tempo ligado do que o relógio);
//   - as entradas depois do fim de um trecho, até o começo do próximo, são as
//     horas do buraco. Ficam com a hora contada a partir do trecho de antes:
//     se o aparelho foi desligado pra mudar de lugar, o erro é só o tempo da
//     mudança;
//   - quando o trecho de agora ainda não tem encaixe, só dá pra saber onde
//     param as horas do buraco logo depois de conectar (antes de o aparelho
//     fechar uma hora nova) ou se ele ficou ligado direto.
func radonFill(mem radonscan.Memory, live []RadonRow, ons []radonOn, now time.Time, place func(start, end int64) string) ([]RadonMemRow, radonFillInfo) {
	var info radonFillInfo
	sts := radonStretches(live, now.Add(-radonAlignDays*24*time.Hour))
	info.stretches = len(sts)
	newest := radonNewest(mem.Hours)
	oMin := radonOMin(mem.Hours, now)
	oMax, prev := int64(math.MaxInt64), false
	for i := range sts {
		st := &sts[i]
		cs := radonCands(mem.Hours, st.rows, time.UnixMilli(st.from), time.UnixMilli(st.to))
		al, o, err := radonPick(cs, oMin, oMax)
		if err != nil {
			continue
		}
		st.al, st.ok = al, true
		info.aligned++
		info.notes = append(info.notes, fmt.Sprintf("%s–%s: virada aos %d min, última hora na entrada %d",
			time.UnixMilli(st.from).Format("02/01 15:04"), time.UnixMilli(st.to).Format("02/01 15:04"), al.phase/60, al.index(time.UnixMilli(st.to))))
		if !prev || o+600 < oMax {
			oMax, prev = o+600, true
		}
	}
	var out []RadonMemRow
	last := -1
	for i := 0; i+1 < len(sts); i++ {
		if sts[i].ok {
			last = i
		}
		if last < 0 {
			continue
		}
		A := sts[last].al
		E, S := time.UnixMilli(sts[i].to), time.UnixMilli(sts[i+1].from)
		if now.Sub(S) > radonWeek+24*time.Hour {
			continue
		}
		lo, hi := A.index(E), min(A.index(S), newest)
		for j := i + 1; j < len(sts); j++ {
			if sts[j].ok {
				hi = min(hi, sts[j].al.index(time.UnixMilli(sts[j].from)))
				break
			}
		}
		if i+1 == len(sts)-1 && !sts[i+1].ok {
			// o trecho de agora ainda não tem encaixe
			if c, ok := radonHoursSince(ons, S, mem.Uptime); ok {
				// o tempo ligado gravado ao conectar diz quantas horas
				// são desta conexão
				hi = min(hi, newest-c)
			} else {
				switch {
				case now.Sub(S) <= 15*time.Minute:
					// acabou de conectar: a mais nova ainda é do buraco
				case A.index(now) <= newest+1:
					// ficou ligado direto, ou parou menos de uma hora: as
					// horas desta conexão caem em cima das leituras ao
					// vivo e ficam de fora
				default:
					info.pending = true
					continue
				}
			}
		}
		// o relógio do aparelho é o tempo ligado: se o buraco durou bem mais no
		// relógio do PC do que as horas que o aparelho fechou nele, ele ficou
		// sem energia no meio e não dá pra saber onde. Contadas a partir do
		// trecho de antes, as horas de depois da parada sairiam adiantadas
		// por todo o tempo sem energia (e podiam cair no cômodo de antes).
		// Até umas 3 h (a mudança de cômodo de 25/11 ficou 1h30 sem energia)
		// o erro é pequeno e as horas entram; mais que isso, ficam de fora
		// (QA, pente fino da v3)
		if off := S.Sub(E) - time.Duration(hi-lo)*time.Hour; off > 3*time.Hour {
			info.unpowered++
			info.notes = append(info.notes, fmt.Sprintf("%s–%s: %d hora(s) do aparelho em %.1f h de relógio (~%.1f h sem energia): ficaram de fora",
				E.Format("02/01 15:04"), S.Format("02/01 15:04"), hi-lo, S.Sub(E).Hours(), off.Hours()))
			continue
		}
		for idx := lo + 1; idx <= hi; idx++ {
			end := A.end(idx)
			start := end.Add(-time.Hour)
			if end.After(now.Add(-3*time.Minute)) || now.Sub(end) > radonWeek+24*time.Hour {
				continue
			}
			l, h := start.Add(15*time.Minute).UnixMilli(), end.Add(5*time.Minute).UnixMilli()
			k := sort.Search(len(live), func(k int) bool { return live[k].T > l })
			if k < len(live) && live[k].T <= h {
				continue // o app estava lendo nessa hora
			}
			c := mem.Hours[idx]
			out = append(out, RadonMemRow{Start: start.UnixMilli(), End: end.UnixMilli(), Place: place(start.UnixMilli(), end.UnixMilli()),
				Counts: c, PCiL: float64(c) * radonscan.PCiPerCount})
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].End < out[j].End })
	return out, info
}

// RadonMemRow é uma hora trazida da memória do aparelho.
type RadonMemRow struct {
	Start  int64   `json:"start"`
	End    int64   `json:"end"`
	Place  string  `json:"place"`
	Counts int     `json:"counts"`
	PCiL   float64 `json:"pciL"`
}

func formatRadonMem(r RadonMemRow) string {
	return time.UnixMilli(r.Start).Format(time.RFC3339) + "," + time.UnixMilli(r.End).Format(time.RFC3339) + "," + r.Place + "," +
		strconv.Itoa(r.Counts) + "," + strconv.FormatFloat(math.Round(r.PCiL*1e4)/1e4, 'f', -1, 64)
}

func parseRadonMem(line string) (RadonMemRow, bool) {
	f := strings.Split(strings.TrimSpace(line), ",")
	if len(f) != 5 {
		return RadonMemRow{}, false
	}
	s, err1 := time.Parse(time.RFC3339, f[0])
	e, err2 := time.Parse(time.RFC3339, f[1])
	c, err3 := strconv.Atoi(f[3])
	if err1 != nil || err2 != nil || err3 != nil || c < 0 {
		return RadonMemRow{}, false
	}
	return RadonMemRow{Start: s.UnixMilli(), End: e.UnixMilli(), Place: f[2], Counts: c, PCiL: float64(c) * radonscan.PCiPerCount}, true
}

func (a *App) radonMemRows() []RadonMemRow {
	b, err := os.ReadFile(a.radonMemPath())
	if err != nil {
		return nil
	}
	var out []RadonMemRow
	for _, ln := range strings.Split(string(b), "\n") {
		if r, ok := parseRadonMem(ln); ok {
			out = append(out, r)
		}
	}
	sort.SliceStable(out, func(i, j int) bool { return out[i].End < out[j].End })
	return out
}

// radonMemSave junta as horas novas (as que já existem ficam como estão;
// a mesma hora é a que termina no mesmo minuto) e devolve só as novas.
func (a *App) radonMemSave(rows []RadonMemRow) ([]RadonMemRow, error) {
	old := a.radonMemRows()
	have := map[int64]bool{}
	for _, r := range old {
		have[r.End/60000] = true
	}
	var add []RadonMemRow
	for _, r := range rows {
		k := r.End / 60000
		if have[k] || have[k-1] || have[k+1] {
			continue
		}
		have[k] = true
		add = append(add, r)
	}
	if len(add) == 0 {
		return nil, nil
	}
	path := a.radonMemPath()
	_, statErr := os.Stat(path)
	f, err := os.OpenFile(path, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o644)
	if err != nil {
		return nil, err
	}
	w := bufio.NewWriter(f)
	if os.IsNotExist(statErr) {
		w.WriteString(radonMemHeader + "\n")
	}
	for _, r := range add {
		w.WriteString(formatRadonMem(r) + "\n")
	}
	if err := w.Flush(); err != nil {
		f.Close()
		return nil, err
	}
	return add, f.Close()
}

// radonGaps: buracos nas leituras ao vivo dos últimos 7 dias que nenhuma
// hora da memória cobre ainda (o mais recente por último).
func radonGaps(live []RadonRow, mem []RadonMemRow, now time.Time) []RadonGap {
	cut := now.Add(-radonWeek).UnixMilli()
	var out []RadonGap
	for i := 1; i < len(live); i++ {
		p, n := live[i-1].T, live[i].T
		if n < cut || time.Duration(n-p)*time.Millisecond <= radonGapMin {
			continue
		}
		covered := false
		for _, m := range mem {
			if m.End > p && m.Start < n {
				covered = true
				break
			}
		}
		if !covered {
			out = append(out, RadonGap{From: p, To: n})
		}
	}
	return out
}

// radonPlaceEvent é uma troca de cômodo (radonscan-comodos.csv). Grava
// também as trocas feitas com o aparelho desconectado, que não viram linha
// no radonscan.csv.
type radonPlaceEvent struct {
	T     int64
	Place string
}

func (a *App) radonPlacePath() string { return filepath.Join(a.dataDir, "radonscan-comodos.csv") }

func (a *App) radonPlaceLog(place string, t time.Time) {
	path := a.radonPlacePath()
	_, statErr := os.Stat(path)
	f, err := os.OpenFile(path, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o644)
	if err != nil {
		a.logf("radonscan: não gravei a troca de cômodo: %v", err)
		return
	}
	defer f.Close()
	if os.IsNotExist(statErr) {
		f.WriteString("hora,comodo\n")
	}
	f.WriteString(t.Format(time.RFC3339) + "," + place + "\n")
}

func (a *App) radonPlaceEvents() []radonPlaceEvent {
	b, err := os.ReadFile(a.radonPlacePath())
	if err != nil {
		return nil
	}
	var out []radonPlaceEvent
	for _, ln := range strings.Split(string(b), "\n") {
		f := strings.Split(strings.TrimSpace(ln), ",")
		if len(f) != 2 || !radonPlaces[f[1]] {
			continue
		}
		if t, err := time.Parse(time.RFC3339, f[0]); err == nil {
			out = append(out, radonPlaceEvent{T: t.UnixMilli(), Place: f[1]})
		}
	}
	sort.SliceStable(out, func(i, j int) bool { return out[i].T < out[j].T })
	return out
}

// radonPlaceAt diz o cômodo de uma hora da memória, olhando o meio dela:
// vale a última leitura ao vivo ou a última troca de cômodo, a mais nova.
// Num buraco sem troca gravada (antes da 2.106.2) em que o cômodo mudou de
// um lado pro outro, vale o de quando o aparelho voltou: a troca foi feita
// com ele fora.
func radonPlaceAt(live []RadonRow, evs []radonPlaceEvent, fallback string) func(start, end int64) string {
	return func(start, end int64) string {
		t := (start + end) / 2
		i := sort.Search(len(live), func(i int) bool { return live[i].T > t })
		place, at := fallback, int64(math.MinInt64)
		if i > 0 {
			place, at = live[i-1].Place, live[i-1].T
		}
		j := sort.Search(len(evs), func(j int) bool { return evs[j].T > t })
		if j > 0 && evs[j-1].T > at {
			return evs[j-1].Place
		}
		if i > 0 && i < len(live) && live[i].T-at > radonGapMin.Milliseconds() && live[i].Place != place {
			if j < len(evs) && evs[j].T < live[i].T {
				return place // a troca gravada veio depois desta hora
			}
			return live[i].Place
		}
		return place
	}
}

// radonReadMemory lê a memória do aparelho conectado e grava as horas que
// faltam. Usado pela leitura automática e pelo botão.
func (a *App) radonReadMemory(why string) RadonMemResult {
	now := time.Now()
	res := RadonMemResult{At: now.UnixMilli()}
	a.radon.mu.Lock()
	d := a.radon.dev
	a.radon.mu.Unlock()
	if d == nil {
		res.Err = "O RadonScan não está conectado."
		return a.radonMemDone(res, why)
	}
	mem, err := withTimeout(60*time.Second, d.ReadMemory)
	if err != nil {
		a.logf("radonscan: memória: leitura falhou: %v", err)
		res.Err = "Não consegui ler a memória do RadonScan. Tente de novo daqui a pouco."
		return a.radonMemDone(res, why)
	}
	live := a.radonRows()
	a.mu.Lock()
	cur := a.settings.RadonPlace
	a.mu.Unlock()
	fill, info := radonFill(mem, live, a.radonOns(), now, radonPlaceAt(live, a.radonPlaceEvents(), cur))
	for _, n := range info.notes {
		a.logf("radonscan: memória: trecho %s", n)
	}
	if info.aligned == 0 || (info.pending && len(fill) == 0) {
		a.logf("radonscan: memória: %v (%d trecho(s), %d encaixado(s), ligado %d h no total)", errRadonNoFit, info.stretches, info.aligned, mem.Uptime/3600)
		res.Err = "Ainda não dá pra saber a hora de cada registro da memória. O app precisa de algumas horas lendo o aparelho ao vivo; ele tenta de novo sozinho."
		return a.radonMemDone(res, why)
	}
	add, err := a.radonMemSave(fill)
	if err != nil {
		a.logf("radonscan: memória: não gravei: %v", err)
		res.Err = "Não consegui gravar as horas da memória."
		return a.radonMemDone(res, why)
	}
	a.logf("radonscan: memória: %d de %d trecho(s) encaixado(s), ligado %d h no total; %d hora(s) nova(s)",
		info.aligned, info.stretches, mem.Uptime/3600, len(add))
	if len(add) == 0 && info.unpowered > 0 {
		res.Err = "O RadonScan ficou um tempo sem energia enquanto estava fora do PC. Sem relógio, não dá pra saber a hora de cada registro desse buraco, e essas horas ficaram de fora."
	}
	if len(add) > 0 {
		res.Hours = len(add)
		res.From, res.To = add[0].Start, add[len(add)-1].End
		var s float64
		for _, r := range add {
			s += r.PCiL
		}
		res.Mean = s / float64(len(add))
		a.logEvent(fmt.Sprintf("RadonScan: %d hora(s) trazida(s) da memória (%s a %s)", len(add),
			time.UnixMilli(res.From).Format("02/01 15:04"), time.UnixMilli(res.To).Format("02/01 15:04")))
	}
	return a.radonMemDone(res, why)
}

func (a *App) radonMemDone(res RadonMemResult, why string) RadonMemResult {
	if res.Err != "" {
		a.logf("radonscan: memória (%s): %s", why, res.Err)
	}
	a.radon.mu.Lock()
	r := res
	a.radon.memLast = &r
	a.radon.mu.Unlock()
	a.emitRadon()
	return res
}

// DownloadRadonMemory é o botão "Baixar da memória do aparelho".
func (a *App) DownloadRadonMemory() (RadonState, error) {
	a.radonReadMemory("botão")
	return a.GetRadonScan(), nil
}

// radonMemAuto decide, a cada leitura, se é hora de ler a memória sozinho:
// depois de conectar, quando há buraco recente que a memória não cobriu.
func (a *App) radonMemAuto(now time.Time) {
	a.radon.mu.Lock()
	due := a.radon.memTries < radonMemTries && !now.Before(a.radon.memNext)
	a.radon.mu.Unlock()
	if !due {
		return
	}
	if len(radonGaps(a.radonRows(), a.radonMemRows(), now)) == 0 {
		a.radon.mu.Lock()
		a.radon.memTries = radonMemTries // nada pra trazer nesta conexão
		a.radon.mu.Unlock()
		return
	}
	res := a.radonReadMemory("automático")
	a.radon.mu.Lock()
	a.radon.memTries++
	if res.Hours > 0 || res.Err == "" {
		a.radon.memTries = radonMemTries
	}
	a.radon.memNext = now.Add(radonMemRetry)
	a.radon.mu.Unlock()
}

// radonMergeMem põe as horas da memória entre as leituras ao vivo (cada
// uma vira uma linha no fim da hora, com a média daquela hora).
func radonMergeMem(live []RadonRow, mem []RadonMemRow) []RadonRow {
	if len(mem) == 0 {
		return live
	}
	out := append([]RadonRow{}, live...)
	for _, m := range mem {
		out = append(out, RadonRow{T: m.End, Place: m.Place, Hour: m.PCiL, Mem: true, Counts: m.Counts})
	}
	sort.SliceStable(out, func(i, j int) bool { return out[i].T < out[j].T })
	return out
}
