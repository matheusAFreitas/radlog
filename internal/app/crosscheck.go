package app

import (
	"fmt"
	"math"
	"sync"
	"time"
)

// ---------- confirmação cruzada (2.59) ----------
//
// Quando o GMC-800 ou o RC102 entra em atenção ou alarme e os dois estão no
// mesmo lugar, compara com o outro aparelho nos mesmos minutos, pela subida
// sobre o fundo de cada um em % (a calibração não interfere). Spec:
// ux/cruzada.html.
//   - fundo de cada um: contagens por minuto da hora antes do alerta (de 62 a
//     2 min antes; 20 min no mínimo, senão a mediana de 30 dias em casa);
//   - trecho: de 2 min antes do alerta até o fim dele (no máximo 15 min);
//   - confirmado: o outro passou de 3σ e subiu pelo menos 40% do que o que
//     apitou subiu;
//   - não confirmado: com a subida do que apitou o outro já daria 5σ e ficou
//     abaixo de 2σ;
//   - conferindo: o outro ainda não juntou contagens para ver uma subida
//     desse tamanho (mostra o tempo que falta);
//   - sem como conferir: lugares diferentes, outro desconectado, medindo
//     objeto, ou o alerta acabou antes de dar para ver.
// O resultado é refeito das leituras gravadas (dá o mesmo depois), e os de
// alertas que já acabaram ficam guardados na memória. Nunca mexe no som.

const (
	ccPre     = 2 * time.Minute  // o trecho começa 2 min antes do alerta
	ccBase    = 60 * time.Minute // fundo: a hora antes disso
	ccBaseMin = 20               // minutos de fundo no mínimo
	ccMax     = 15 * time.Minute // conferindo: no máximo isto depois do alerta
	ccOkZ     = 3.0
	ccOkFrac  = 0.4
	ccNoZ     = 2.0
	ccSeeZ    = 5.0
	ccChartB  = 10 * time.Minute // gráfico: 10 min antes
	ccChartA  = 5 * time.Minute  // e 5 min depois
)

// CrossCheck é o resultado da confirmação cruzada de um alerta.
type CrossCheck struct {
	Dev   string `json:"dev"`   // aparelho que apitou: "gmc" ou "radiacode"
	At    int64  `json:"at"`    // começo do alerta (ms)
	To    int64  `json:"to"`    // fim do trecho comparado (ms)
	State string `json:"state"` // ok, no, wait, na
	Why   string `json:"why,omitempty"`
	Text  string `json:"text"` // a linha da faixa
	Tag   string `json:"tag"`  // o selo da lista

	Rise  float64 `json:"rise"` // subida sobre o fundo (fração), do que apitou
	Z     float64 `json:"z"`
	ORise float64 `json:"oRise"` // e do outro
	OZ    float64 `json:"oZ"`
	Left  int     `json:"left"` // conferindo: segundos que faltam (0 = esperando o minuto)
	Frac  float64 `json:"frac"`

	Series *CrossSeries `json:"series,omitempty"`
}

// CrossSeries é o gráfico do alerta: subida por minuto sobre o fundo de cada
// um (fração; nil sem leitura), de 10 min antes a 5 min depois.
type CrossSeries struct {
	T0   int64      `json:"t0"`
	Step int64      `json:"step"`
	Me   []*float64 `json:"me"`
	Ot   []*float64 `json:"ot"`
	OtSd float64    `json:"otSd"` // 1σ de um minuto do outro (fração)
}

type ccCache struct {
	mu sync.Mutex
	m  map[string]CrossCheck
}

var crossCache = ccCache{m: map[string]CrossCheck{}}

// CrossReq pede a confirmação de um alerta (ou episódio) já na lista.
type CrossReq struct {
	Dev  string `json:"dev"`
	From int64  `json:"from"`
	To   int64  `json:"to"` // 0 = ainda ativo
}

// GetCrossCheck devolve a confirmação cruzada do alerta do aparelho dev que
// começou em fromMs e acabou em toMs (0 = ativo). fromMs 0 = o episódio
// ativo agora desse aparelho. series inclui o gráfico.
func (a *App) GetCrossCheck(dev string, fromMs, toMs int64, series bool) CrossCheck {
	dev = devName(dev)
	now := time.Now()
	if fromMs <= 0 {
		fromMs = a.activeAlertStart(dev)
		toMs = 0
		if fromMs == 0 {
			return CrossCheck{Dev: dev, State: "na", Why: "sem"}
		}
	}
	key := fmt.Sprintf("%s|%s-%d-%d-%v", a.dataDir, dev, fromMs, toMs, series)
	done := toMs > 0 && now.Sub(time.UnixMilli(toMs)) > 2*time.Minute
	if done {
		crossCache.mu.Lock()
		c, ok := crossCache.m[key]
		crossCache.mu.Unlock()
		if ok {
			return c
		}
	}
	end := now
	if toMs > 0 && toMs < now.UnixMilli() {
		end = time.UnixMilli(toMs)
	}
	c := a.crossCheck(dev, time.UnixMilli(fromMs), end, toMs > 0, now, series)
	if done {
		crossCache.mu.Lock()
		if len(crossCache.m) > 2000 {
			crossCache.m = map[string]CrossCheck{}
		}
		crossCache.m[key] = c
		crossCache.mu.Unlock()
	}
	return c
}

// GetCrossChecks faz vários de uma vez (os selos da lista de alertas).
func (a *App) GetCrossChecks(reqs []CrossReq) []CrossCheck {
	out := make([]CrossCheck, len(reqs))
	for i, r := range reqs {
		out[i] = a.GetCrossCheck(r.Dev, r.From, r.To, false)
	}
	return out
}

// activeAlertStart é o começo do episódio ativo (real) do aparelho.
func (a *App) activeAlertStart(dev string) int64 {
	want := ""
	if dev == devRC {
		want = devRC
	}
	a.mu.Lock()
	defer a.mu.Unlock()
	var t int64
	for _, e := range a.events {
		if e.Kind == kindAlert && !e.Demo && e.Device == want && a.isActiveLocked(e.ID) && (t == 0 || e.Time < t) {
			t = e.Time
		}
	}
	return t
}

// ccMinutes são as contagens de cada minuto do aparelho.
func (a *App) ccMinutes(dev string, from, to time.Time) map[int64]float64 {
	m := map[int64]float64{}
	if dev == devRC {
		if a.dataDir == "" {
			return m
		}
		for _, p := range a.rcRange(from, to) {
			m[p.T.Truncate(time.Minute).UnixMilli()] = p.CPS * 60
		}
		return m
	}
	if a.store == nil {
		return m
	}
	if pts, err := a.points(from, to); err == nil {
		for _, p := range pts {
			if !p.T.Before(from) && p.T.Before(to) {
				m[p.T.Truncate(time.Minute).UnixMilli()] = p.CPM
			}
		}
	}
	return m
}

// ccSide é a conta de um aparelho no trecho.
type ccSide struct {
	b, nb    float64 // fundo (contagens/min) e contagens que o formaram
	s, exp   float64 // contagens no trecho e esperadas pelo fundo
	n        int     // minutos com leitura no trecho
	rise, z  float64
	min      map[int64]float64
	hasBase  bool
	lastData int64
}

func (a *App) ccSide(dev string, tA, wEnd time.Time) ccSide {
	w0 := tA.Add(-ccPre)
	from := w0.Add(-ccBase)
	cEnd := tA.Add(ccChartA)
	if wEnd.After(cEnd) {
		cEnd = wEnd
	}
	m := a.ccMinutes(dev, from, cEnd)
	sd := ccSide{min: m}
	var bs float64
	bn := 0
	for k, v := range m {
		t := time.UnixMilli(k)
		if t.Before(w0) {
			bs += v
			bn++
		}
		if k > sd.lastData {
			sd.lastData = k
		}
	}
	if bn >= ccBaseMin && bs > 0 {
		sd.b, sd.nb, sd.hasBase = bs/float64(bn), bs, true
	} else if ws := a.whySeries(dev, w0, w0); ws.norm > 0 {
		sd.b, sd.nb, sd.hasBase = ws.norm, 1e9, true
	}
	for k, v := range m {
		t := time.UnixMilli(k)
		if !t.Before(w0) && t.Add(time.Minute).Compare(wEnd) <= 0 {
			sd.s += v
			sd.n++
		}
	}
	if sd.hasBase && sd.n > 0 {
		sd.exp = sd.b * float64(sd.n)
		sd.rise = sd.s/sd.exp - 1
		sd.z = (sd.s - sd.exp) / math.Sqrt(sd.exp+sd.exp*sd.exp/sd.nb)
	}
	return sd
}

func (a *App) crossCheck(dev string, tA, end time.Time, ended bool, now time.Time, series bool) CrossCheck {
	other := devRC
	if dev == devRC {
		other = "gmc"
	}
	me, on := whyDevName(dev), whyDevName(other)
	c := CrossCheck{Dev: dev, At: tA.UnixMilli()}
	wEnd := end
	if lim := tA.Add(ccMax); wEnd.After(lim) {
		wEnd = lim
	}
	c.To = wEnd.UnixMilli()
	final := ended || !now.Before(tA.Add(ccMax))
	na := func(why, text string) CrossCheck {
		c.State, c.Why, c.Text, c.Tag = "na", why, text, "sem como conferir"
		return c
	}
	ms, oth := a.ccSide(dev, tA, wEnd), a.ccSide(other, tA, wEnd)
	if series {
		c.Series = ccSeries(tA, now, ms, oth)
	}
	// mesmo lugar, nada medindo objeto
	meas := a.measuringChecker()
	for t := tA.Add(-ccPre); !t.After(wEnd); t = t.Add(time.Minute) {
		if meas(t) {
			return na("objeto", "Sem como conferir: medindo um objeto.")
		}
	}
	if !ended {
		if st := a.GetMeasure(); st.Phase == "fundo" || st.Phase == "objeto" {
			return na("objeto", "Sem como conferir: medindo um objeto.")
		}
	}
	awayMe, awayOt := a.awayCheckerFor(dev)(tA), a.awayCheckerFor(other)(tA)
	if awayMe != awayOt {
		if awayOt {
			return na("longe", fmt.Sprintf("Sem como conferir: o %s está com você, fora de casa.", on))
		}
		return na("casa", fmt.Sprintf("Sem como conferir: o %s ficou em casa.", on))
	}
	// ao vivo: conectado agora, ou com leitura gravada há pouco
	connected := true
	if !ended && now.Sub(time.UnixMilli(oth.lastData)) > 3*time.Minute {
		if other == devRC {
			connected = a.rcConnected()
		} else {
			a.mu.Lock()
			connected = a.dev != nil
			a.mu.Unlock()
		}
	}
	if !connected || !oth.hasBase || (oth.n == 0 && (final || now.Sub(tA) > 3*time.Minute)) {
		return na("desc", fmt.Sprintf("Sem como conferir: o %s está desconectado.", on))
	}
	c.Rise, c.Z, c.ORise, c.OZ = ms.rise, ms.z, oth.rise, oth.z
	// z que o outro teria com a mesma subida, e o tempo para chegar a 5σ
	R := ms.rise
	zexp := 0.0
	if R > 0 && oth.exp > 0 {
		zexp = R * oth.exp / math.Sqrt(oth.exp+oth.exp*oth.exp/oth.nb)
	}
	switch {
	case oth.n > 0 && oth.z >= ccOkZ && oth.rise >= ccOkFrac*R && R > 0:
		c.State = "ok"
		c.Text = fmt.Sprintf("Confirmado pelo %s: ele também subiu %s sobre o fundo dele.", on, ccPct(oth.rise))
		c.Tag = fmt.Sprintf("confirmado pelo %s", on)
	case zexp >= ccSeeZ && oth.z < ccNoZ:
		c.State = "no"
		c.Text = fmt.Sprintf("O %s não viu: %s sobre o fundo dele. Pode ser algo perto só do %s, interferência ou o aparelho.", on, ccPct(oth.rise), me)
		c.Tag = fmt.Sprintf("só no %s", me)
	case final && zexp >= ccSeeZ:
		// deu para ver e o outro subiu pouco (entre 2σ e 3σ, ou menos de 40%)
		c.State = "no"
		c.Text = fmt.Sprintf("O %s não viu: %s sobre o fundo dele. Pode ser algo perto só do %s, interferência ou o aparelho.", on, ccPct(oth.rise), me)
		c.Tag = fmt.Sprintf("só no %s", me)
	case final:
		return na("curta", fmt.Sprintf("Sem como conferir: a subida foi curta demais para o %s.", on))
	default:
		c.State = "wait"
		c.Tag = fmt.Sprintf("conferindo com o %s", on)
		need := 0.0 // segundos de contagem para 5σ com a subida R
		if R > 0 && oth.b > 0 {
			need = ccSeeZ * ccSeeZ / (R * R * oth.b / 60)
		}
		have := float64(oth.n) * 60
		left := need - have
		if lim := tA.Add(ccMax).Sub(now).Seconds(); left > lim {
			left = lim
		}
		if need > 0 {
			c.Frac = math.Min(1, have/need)
		}
		if left >= 1 && R > 0 {
			c.Left = int(math.Ceil(left))
			if other == "gmc" {
				c.Text = fmt.Sprintf("Conferindo com o %s: ele é menos sensível e precisa de mais uns %s.", on, ccDur(c.Left))
			} else {
				c.Text = fmt.Sprintf("Conferindo com o %s: precisa de mais uns %s.", on, ccDur(c.Left))
			}
		} else {
			c.Text = fmt.Sprintf("Conferindo com o %s…", on)
		}
	}
	return c
}

func ccSeries(tA, now time.Time, ms, oth ccSide) *CrossSeries {
	t0 := tA.Truncate(time.Minute).Add(-ccChartB)
	t1 := tA.Truncate(time.Minute).Add(ccChartA)
	s := &CrossSeries{T0: t0.UnixMilli(), Step: 60_000, Me: []*float64{}, Ot: []*float64{}}
	if oth.b > 0 {
		s.OtSd = 1 / math.Sqrt(oth.b)
	}
	for t := t0; t.Before(t1) && t.Before(now); t = t.Add(time.Minute) {
		k := t.UnixMilli()
		for i, sd := range []ccSide{ms, oth} {
			var p *float64
			if v, ok := sd.min[k]; ok && sd.b > 0 {
				r := v/sd.b - 1
				p = &r
			}
			if i == 0 {
				s.Me = append(s.Me, p)
			} else {
				s.Ot = append(s.Ot, p)
			}
		}
	}
	return s
}

func ccPct(x float64) string {
	s := "+"
	if x < 0 {
		s = "−"
	}
	return s + fmt.Sprintf("%.0f%%", math.Abs(x*100))
}

func ccDur(s int) string {
	if s < 60 {
		return fmt.Sprintf("%d s", s)
	}
	return fmt.Sprintf("%d min", (s+30)/60)
}

// alertSpanAt acha o episódio de alerta (real) do aparelho que cobre t:
// começo do primeiro e fim do último (0 = ainda ativo).
func (a *App) alertSpanAt(dev string, t time.Time) (from, to int64, ok bool) {
	want := ""
	if devName(dev) == devRC {
		want = devRC
	}
	ms := t.UnixMilli()
	a.mu.Lock()
	defer a.mu.Unlock()
	active := false
	for _, e := range a.events {
		if e.Kind != kindAlert || e.Demo || e.Retro || e.Device != want {
			continue
		}
		act := a.isActiveLocked(e.ID)
		end := e.End
		if act || end == 0 {
			end = time.Now().UnixMilli()
		}
		// encosta no episódio (1 min de folga, como na lista)
		if e.Time <= ms+60_000 && ms <= end+60_000 {
			if !ok || e.Time < from {
				from = e.Time
			}
			if end > to {
				to = end
			}
			active = active || act
			ok = true
		}
	}
	if active {
		to = 0
	}
	return
}
