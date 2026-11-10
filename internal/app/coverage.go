package app

import (
	"math"
	"os"
	"regexp"
	"sort"
	"sync"
	"time"

	"radlog/internal/store"
)

// ---------- cobertura e maturidade do histórico (2.3) ----------
//
// Cobertura = minutos com leitura ÷ minutos do período, sempre só até agora
// (o resto de um período em andamento ainda não aconteceu). Vem em duas
// versões: desde o primeiro dado do aparelho (o tempo antes de ele existir
// no app não é lacuna) e no período inteiro. Quando o histórico cobre o
// período todo, as duas são iguais.

// covSpan são os denominadores da cobertura de [from, to).
type covSpan struct {
	EffTo    time.Time // fim efetivo: to ou agora, o que vier antes
	HistFrom time.Time // começo do histórico dentro do período
	Span     int       // minutos de from até EffTo
	Hist     int       // minutos de HistFrom até EffTo (0 = nenhum dado ainda)
	Pre      int       // minutos antes do primeiro dado
	Ongoing  bool      // o período ainda não terminou
}

func coverageSpan(from, to, first, now time.Time) covSpan {
	c := covSpan{EffTo: to, Ongoing: to.After(now)}
	if c.EffTo.After(now) {
		c.EffTo = now
	}
	if c.EffTo.Before(from) {
		c.EffTo = from
	}
	c.Span = int(math.Round(c.EffTo.Sub(from).Minutes()))
	switch {
	case first.IsZero() || !first.Before(c.EffTo):
		c.HistFrom = c.EffTo // sem dado até o fim do período
	case first.After(from):
		c.HistFrom = first.Truncate(time.Minute)
	default:
		c.HistFrom = from
	}
	c.Hist = int(math.Round(c.EffTo.Sub(c.HistFrom).Minutes()))
	c.Pre = max(0, c.Span-c.Hist)
	return c
}

func ratio(n, d int) float64 {
	if d <= 0 {
		return 0
	}
	return math.Min(1, float64(n)/float64(d))
}

// primeiro dado de cada aparelho (relido a cada 10 min: backup e importação
// podem trazer dado mais antigo)
type firstCache struct {
	mu       sync.Mutex
	gmc, rc  time.Time
	gmcAt    time.Time
	rcAt     time.Time
	gmcKnown bool
	rcKnown  bool
}

func (a *App) gmcFirst() time.Time {
	c := &a.firsts
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.gmcKnown && !c.gmc.IsZero() && time.Since(c.gmcAt) < 10*time.Minute {
		return c.gmc
	}
	c.gmc = time.Time{}
	if a.store != nil {
		if t, ok := a.store.FirstTime(); ok {
			c.gmc = t
		}
	}
	c.gmcAt, c.gmcKnown = time.Now(), true
	return c.gmc
}

var rcMonthFile = regexp.MustCompile(`^\d{4}-\d{2}\.csv$`)

func (a *App) rcFirst() time.Time {
	c := &a.firsts
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.rcKnown && !c.rc.IsZero() && time.Since(c.rcAt) < 10*time.Minute {
		return c.rc
	}
	c.rc = time.Time{}
	if a.dataDir != "" {
		entries, _ := os.ReadDir(a.rcDir())
		var names []string
		for _, e := range entries {
			if rcMonthFile.MatchString(e.Name()) {
				names = append(names, e.Name())
			}
		}
		sort.Strings(names)
		for _, n := range names {
			m, err := time.ParseInLocation("2006-01.csv", n, time.Local)
			if err != nil {
				continue
			}
			if pts := a.rcRange(m, m.AddDate(0, 1, 0)); len(pts) > 0 {
				c.rc = pts[0].T
				break
			}
		}
	}
	c.rcAt, c.rcKnown = time.Now(), true
	return c.rc
}

// ---------- maturidade: quanto histórico existe para cada janela ----------

// Estados de uma janela.
const (
	matReady   = "disponivel" // o histórico cobre a janela inteira
	matForming = "formacao"   // o primeiro dado está dentro da janela
	matWaiting = "aguardando" // nenhum dado ainda
)

var matWindows = []int{1, 7, 30, 90, 365}

// MatWin é uma janela (últimos N dias até agora).
type MatWin struct {
	Days        int     `json:"days"`
	State       string  `json:"state"`
	HistDays    float64 `json:"histDays"` // dias de histórico dentro da janela
	Minutes     int     `json:"minutes"`  // minutos com leitura
	HistMinutes int     `json:"histMinutes"`
	SpanMinutes int     `json:"spanMinutes"`
	Coverage    float64 `json:"coverage"`    // desde o primeiro dado
	CoverageAll float64 `json:"coverageAll"` // na janela inteira
}

// Maturity é a maturidade do histórico de cada aparelho.
type Maturity struct {
	GMCFirst int64    `json:"gmcFirst"` // ms; 0 = sem dado
	RCFirst  int64    `json:"rcFirst"`
	GMC      []MatWin `json:"gmc"`
	RC       []MatWin `json:"rc"` // vazio se o RC102 nunca gravou
}

// matWins monta as janelas a partir dos instantes dos minutos gravados.
func matWins(times []time.Time, first, now time.Time) []MatWin {
	sort.Slice(times, func(i, j int) bool { return times[i].Before(times[j]) })
	out := make([]MatWin, 0, len(matWindows))
	for _, d := range matWindows {
		from := now.AddDate(0, 0, -d)
		cs := coverageSpan(from, now, first, now)
		i := sort.Search(len(times), func(i int) bool { return !times[i].Before(from) })
		n := len(times) - i
		w := MatWin{Days: d, Minutes: n, HistMinutes: cs.Hist, SpanMinutes: cs.Span,
			HistDays: float64(cs.Hist) / 1440, Coverage: ratio(n, cs.Hist), CoverageAll: ratio(n, cs.Span)}
		switch {
		case cs.Hist == 0:
			w.State = matWaiting
		case cs.Pre > 0:
			w.State = matForming
		default:
			w.State = matReady
		}
		out = append(out, w)
	}
	return out
}

// GetMaturity diz, para 1, 7, 30, 90 e 365 dias, se já existe histórico
// para a janela inteira e quanto dela foi medido.
func (a *App) GetMaturity() Maturity { return a.maturityAt(time.Now()) }

// pointsFirst é o primeiro dado do GMC, contando os minutos do demo que
// estejam no período (o demo não grava no arquivo).
func (a *App) pointsFirst(pts []store.Point) time.Time {
	f := a.gmcFirst()
	if len(pts) > 0 && (f.IsZero() || pts[0].T.Before(f)) {
		return pts[0].T
	}
	return f
}
