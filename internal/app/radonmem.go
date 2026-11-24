package app

// Histórico da memória do GQ RadonScan (2.106.0).
//
// O RadonScan guarda quantas contagens teve em cada hora (ver
// radonscan/memory.go), mas não diz que hora é cada entrada. O app descobre
// comparando a tabela com as médias de 1 hora que ele mesmo gravou em
// radonscan.csv: procura o deslocamento (qual entrada é qual hora) e o
// minuto em que a hora do aparelho vira. Só usa o resultado quando um
// encaixe só bate com folga. Depois preenche as horas em que o app não
// estava lendo (PC desligado, app fechado) em radonscan-memoria.csv, sem
// mexer no radonscan.csv.
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

// radonAlign encaixa a tabela nas leituras ao vivo. since limita às horas
// desde que o aparelho foi ligado (zero = sem limite).
func radonAlign(hours []int, rows []RadonRow, now, since time.Time) (radonAlignment, error) {
	type sample struct {
		k int64
		v int // centésimos de pCi/L
	}
	from := now.Add(-radonAlignDays * 24 * time.Hour)
	if since.After(from) {
		from = since
	}
	n := int64(len(hours))
	// cada encaixe (deslocamento s, virada ph) leva a hora que termina em e
	// para a entrada (e + o)/3600, com o = s·3600 − ph: encaixes com o perto
	// um do outro são o mesmo encaixe visto em minutos vizinhos
	type cand struct {
		s, ph, o int64
		score    float64
	}
	var cs []cand
	for ph := int64(0); ph < 3600; ph += 60 {
		var ss []sample
		kNow := (now.Unix() - ph) / 3600
		for k := kNow; ; k-- {
			e := k*3600 + ph
			if time.Unix(e-3600, 0).Before(from) {
				break
			}
			if e > now.Unix() {
				continue
			}
			if v, ok := radonValueAt(rows, e*1000); ok {
				ss = append(ss, sample{k, int(math.Round(v * 100))})
			}
		}
		if len(ss) < radonAlignMinNZ {
			continue
		}
		for s := -kNow; s+kNow < n; s++ {
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
			cs = append(cs, cand{s, ph, s*3600 - ph, score})
		}
	}
	if len(cs) == 0 {
		return radonAlignment{}, errRadonNoFit
	}
	sort.Slice(cs, func(i, j int) bool { return cs[i].score > cs[j].score })
	best := cs[0]
	for _, c := range cs[1:] {
		if abs64(c.o-best.o) >= 3600 && best.score-c.score < 2 {
			return radonAlignment{}, errRadonNoFit // outro encaixe bate quase igual
		}
	}
	// o minuto da virada: o do meio entre os de nota máxima desse encaixe
	var same []cand
	for _, c := range cs {
		if c.score == best.score && abs64(c.o-best.o) < 3600 {
			same = append(same, c)
		}
	}
	sort.Slice(same, func(i, j int) bool { return same[i].o < same[j].o })
	m := same[len(same)/2]
	return radonAlignment{phase: m.ph, shift: m.s}, nil
}

func abs64(x int64) int64 {
	if x < 0 {
		return -x
	}
	return x
}

var errRadonNoFit = errors.New("não deu pra encaixar a memória nas leituras do app")

// radonFill escolhe as horas da memória que entram: dentro do tempo em que o
// aparelho estava ligado, já fechadas e sem leitura ao vivo do app nelas.
func radonFill(mem radonscan.Memory, al radonAlignment, live []RadonRow, now, since time.Time, place func(t int64) string) []RadonMemRow {
	var out []RadonMemRow
	idxNow := al.index(now.Add(-3 * time.Minute))
	for idx := idxNow; idx >= 0 && idx < int64(len(mem.Hours)); idx-- {
		end := al.end(idx)
		start := end.Add(-time.Hour)
		if start.Before(since.Add(-5*time.Minute)) || now.Sub(end) > radonWeek+24*time.Hour {
			break
		}
		lo, hi := start.Add(15*time.Minute).UnixMilli(), end.Add(5*time.Minute).UnixMilli()
		i := sort.Search(len(live), func(i int) bool { return live[i].T > lo })
		if i < len(live) && live[i].T <= hi {
			continue // o app estava lendo nessa hora
		}
		c := mem.Hours[idx]
		out = append(out, RadonMemRow{Start: start.UnixMilli(), End: end.UnixMilli(), Place: place(end.UnixMilli()), Counts: c, PCiL: float64(c) * radonscan.PCiPerCount})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].End < out[j].End })
	return out
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

// radonPlaceAt é o cômodo da última leitura ao vivo antes de t.
func radonPlaceAt(live []RadonRow, fallback string) func(int64) string {
	return func(t int64) string {
		i := sort.Search(len(live), func(i int) bool { return live[i].T > t })
		if i == 0 {
			return fallback
		}
		return live[i-1].Place
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
	var since time.Time
	if mem.Uptime >= 0 {
		since = now.Add(-time.Duration(mem.Uptime+3600) * time.Second)
	}
	al, err := radonAlign(mem.Hours, live, now, since)
	if err != nil {
		a.logf("radonscan: memória: %v (ligado há %d s)", err, mem.Uptime)
		res.Err = "Ainda não dá pra saber a hora de cada registro da memória. O app precisa de algumas horas lendo o aparelho ao vivo; ele tenta de novo sozinho."
		return a.radonMemDone(res, why)
	}
	a.mu.Lock()
	cur := a.settings.RadonPlace
	a.mu.Unlock()
	if since.IsZero() {
		since = now.Add(-radonWeek)
	}
	fill := radonFill(mem, al, live, now, since, radonPlaceAt(live, cur))
	add, err := a.radonMemSave(fill)
	if err != nil {
		a.logf("radonscan: memória: não gravei: %v", err)
		res.Err = "Não consegui gravar as horas da memória."
		return a.radonMemDone(res, why)
	}
	a.logf("radonscan: memória: encaixe na entrada %d às %s (virada da hora aos %d min), ligado há %d h; %d hora(s) nova(s)",
		al.index(now), now.Format("15:04"), al.phase/60, mem.Uptime/3600, len(add))
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
