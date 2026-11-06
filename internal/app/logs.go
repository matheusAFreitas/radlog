package app

import (
	"bufio"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"time"

	"radlog/internal/i18n"
)

// ---------- visor do log técnico (Eventos → Log técnico) ----------
//
// Lê o log.txt (e o log-anterior.txt) e devolve as linhas com nível
// (DEBUG, INFO, WARN, ERROR), componente ("conexão", "leitura ao vivo"...),
// filtros e um histograma por nível, no estilo Kibana/Grafana.
//
// As linhas novas já saem com o nível escrito; as antigas (sem nível) são
// classificadas pelo texto, com as mesmas regras.

const (
	lvDebug = "DEBUG"
	lvInfo  = "INFO"
	lvWarn  = "WARN"
	lvError = "ERROR"
)

var logLevels = []string{lvDebug, lvInfo, lvWarn, lvError}

var (
	reErrN    = regexp.MustCompile(`erro (\d+)/(\d+)`)
	reNotHere = regexp.MustCompile(`^(re)?conex[aã]o: (n[aã]o abriu \S+: )?\S+ n[aã]o encontrada`)
)

// classifyLog dá o nível de uma mensagem pelo texto.
func classifyLog(msg string) string {
	m := strings.ToLower(msg)
	switch {
	// rotina quando o cabo está fora: a cada 10 s
	case strings.HasPrefix(m, "conexão: abrindo"), reNotHere.MatchString(m):
		return lvDebug
	case strings.Contains(m, "sem resposta"), strings.Contains(m, "porta travada"),
		strings.Contains(m, "parou de responder"), strings.Contains(m, "travou"),
		strings.Contains(m, "panic"), strings.Contains(m, "not functioning"),
		strings.HasPrefix(m, "evento: erro"), strings.Contains(m, "erro ao gravar"):
		return lvError
	}
	if g := reErrN.FindStringSubmatch(m); g != nil {
		// "access is denied" é o Windows avisando que o USB sumiu (cabo):
		// esperado ao mexer no cabo, não é pane
		if g[1] == g[2] && !strings.Contains(m, "access is denied") {
			return lvError // última tentativa
		}
		return lvWarn
	}
	switch {
	case strings.HasPrefix(m, "reconexão: falhou"), strings.Contains(m, "não abriu"),
		strings.Contains(m, "conexão perdida"), strings.Contains(m, "conexão com o contador perdida"),
		strings.HasPrefix(m, "portas ("), strings.Contains(m, "lenta"), strings.Contains(m, "em uso"),
		strings.Contains(m, "invalid"), strings.Contains(m, "falhou"), strings.Contains(m, "não consegui"),
		strings.Contains(m, "access is denied"), strings.Contains(m, "erro"), strings.Contains(m, "sumiu da lista"),
		strings.Contains(m, "interferência"):
		return lvWarn
	}
	return lvInfo
}

// logComponent é o "de onde veio": o texto antes do primeiro ": ".
func logComponent(msg string) string {
	if strings.HasPrefix(msg, "====") {
		return "app"
	}
	i := strings.Index(msg, ": ")
	if i <= 0 || i > 28 {
		return "app"
	}
	c := msg[:i]
	if j := strings.Index(c, " ("); j > 0 { // "portas (depois de 3 erros)"
		c = c[:j]
	}
	return c
}

// LogEntry é uma linha do log técnico.
type LogEntry struct {
	T     int64  `json:"t"` // ms
	Level string `json:"level"`
	Comp  string `json:"comp"`
	Msg   string `json:"msg"`
}

// LogQuery são os filtros do visor.
type LogQuery struct {
	From   int64    `json:"from"` // ms; 0 = desde o começo
	To     int64    `json:"to"`   // ms; 0 = agora
	Levels []string `json:"levels"`
	Comp   string   `json:"comp"`
	Text   string   `json:"text"`
	Limit  int      `json:"limit"`
}

// LogBucket é uma barra do histograma.
type LogBucket struct {
	T      int64          `json:"t"`
	Counts map[string]int `json:"counts"`
}

// LogComp conta as linhas de um componente.
type LogComp struct {
	Name  string `json:"name"`
	Count int    `json:"count"`
}

// LogResult é a resposta do visor.
type LogResult struct {
	Entries  []LogEntry     `json:"entries"` // mais nova primeiro
	Total    int            `json:"total"`   // linhas que passaram nos filtros
	Counts   map[string]int `json:"counts"`  // por nível (com os outros filtros)
	Comps    []LogComp      `json:"comps"`
	Buckets  []LogBucket    `json:"buckets"`
	BucketMs int64          `json:"bucketMs"`
	From     int64          `json:"from"`
	To       int64          `json:"to"`
}

// parseLogLine lê "2026-11-03 14:17:23.735  WARN  texto" (ou sem o nível).
func parseLogLine(line string) (LogEntry, bool) {
	if len(line) < 25 {
		return LogEntry{}, false
	}
	t, err := time.ParseInLocation("2006-01-02 15:04:05.000", line[:23], time.Local)
	if err != nil {
		return LogEntry{}, false
	}
	rest := strings.TrimLeft(line[23:], " ")
	lvl := ""
	for _, l := range logLevels {
		if strings.HasPrefix(rest, l+" ") {
			lvl = l
			rest = strings.TrimLeft(rest[len(l):], " ")
			break
		}
	}
	if lvl == "" {
		lvl = classifyLog(rest)
	}
	return LogEntry{T: t.UnixMilli(), Level: lvl, Comp: logComponent(rest), Msg: rest}, true
}

func (a *App) readLogEntries() []LogEntry {
	var out []LogEntry
	for _, name := range []string{"log-anterior.txt", "log.txt"} {
		f, err := os.Open(filepath.Join(a.dataDir, name))
		if err != nil {
			continue
		}
		sc := bufio.NewScanner(f)
		sc.Buffer(make([]byte, 64*1024), 1024*1024)
		for sc.Scan() {
			if e, ok := parseLogLine(sc.Text()); ok {
				out = append(out, e)
			}
		}
		f.Close()
	}
	sort.SliceStable(out, func(i, j int) bool { return out[i].T < out[j].T })
	return out
}

func logBucketSize(span time.Duration) time.Duration {
	for _, b := range []time.Duration{time.Minute, 5 * time.Minute, 15 * time.Minute, time.Hour, 3 * time.Hour, 6 * time.Hour, 24 * time.Hour} {
		if span/b <= 72 {
			return b
		}
	}
	return 7 * 24 * time.Hour
}

// GetLogs devolve as linhas do log técnico com os filtros e o histograma.
func (a *App) GetLogs(q LogQuery) (LogResult, error) {
	all := a.readLogEntries()
	now := time.Now()
	to := now
	if q.To > 0 {
		to = time.UnixMilli(q.To)
	}
	from := time.Time{}
	if q.From > 0 {
		from = time.UnixMilli(q.From)
	} else if len(all) > 0 {
		from = time.UnixMilli(all[0].T)
	} else {
		from = to.Add(-time.Hour)
	}
	if q.Limit <= 0 || q.Limit > 5000 {
		q.Limit = 1000
	}
	want := map[string]bool{}
	for _, l := range q.Levels {
		want[strings.ToUpper(l)] = true
	}
	text := strings.ToLower(strings.TrimSpace(q.Text))

	step := logBucketSize(to.Sub(from))
	start := from.Truncate(step)
	n := int(to.Sub(start)/step) + 1
	res := LogResult{Counts: map[string]int{}, BucketMs: step.Milliseconds(), From: from.UnixMilli(), To: to.UnixMilli(), Entries: []LogEntry{}}
	res.Buckets = make([]LogBucket, n)
	for i := range res.Buckets {
		res.Buckets[i] = LogBucket{T: start.Add(time.Duration(i) * step).UnixMilli(), Counts: map[string]int{}}
	}
	comps := map[string]int{}
	var hits []LogEntry
	for _, e := range all {
		if e.T < res.From || e.T > res.To {
			continue
		}
		if text != "" && !strings.Contains(strings.ToLower(e.Msg), text) {
			continue
		}
		comps[e.Comp]++
		if q.Comp != "" && e.Comp != q.Comp {
			continue
		}
		res.Counts[e.Level]++ // conta antes do filtro de nível (para os botões)
		if len(want) > 0 && !want[e.Level] {
			continue
		}
		if i := int(time.UnixMilli(e.T).Sub(start) / step); i >= 0 && i < n {
			res.Buckets[i].Counts[e.Level]++
		}
		hits = append(hits, e)
	}
	res.Total = len(hits)
	lang := a.uiLang()
	for i := len(hits) - 1; i >= 0 && len(res.Entries) < q.Limit; i-- {
		e := hits[i]
		if lang != "pt" {
			// traduz a linha inteira; o "componente: " do começo volta em
			// português, porque é ele que a tela usa para filtrar e separar
			t := i18n.T(lang, e.Msg)
			if pre := e.Comp + ": "; strings.HasPrefix(e.Msg, pre) {
				if _, rest, ok := strings.Cut(t, ": "); ok {
					t = pre + rest
				}
			}
			e.Msg = t
		}
		res.Entries = append(res.Entries, e)
	}
	for name, c := range comps {
		res.Comps = append(res.Comps, LogComp{Name: name, Count: c})
	}
	sort.Slice(res.Comps, func(i, j int) bool { return res.Comps[i].Count > res.Comps[j].Count })
	return res, nil
}
