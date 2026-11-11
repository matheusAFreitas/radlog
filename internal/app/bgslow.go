package app

import (
	"errors"
	"math"
	"sort"
	"time"

	"radlog/internal/core/stats"
	"radlog/internal/store"
)

// ---------- subida lenta do fundo (2.58) ----------
//
// Compara as últimas 2 semanas com os meses antes delas (spec: ux/fundo.html):
//   - dia válido: mediana das contagens por minuto do GMC-800 em casa, sem
//     chuva no dia (≥ 0,5 mm), sem medir objeto, sem minutos de alerta,
//     saturação ou interferência; pelo menos 12 h medidas;
//   - referência: mediana dos dias válidos de 120 a 30 dias atrás (30 dias
//     no mínimo). Com "mudei o aparelho de lugar", recomeça nesse dia;
//   - agora: mediana dos dias válidos das últimas 2 semanas (10 no mínimo);
//   - vale olhar: agora ≥ +10% e 10 dos últimos 14 dias acima de +5%;
//   - degrau: a mediana de 7 dias pula ≥ 8% de uma semana para a seguinte e
//     o salto explica quase toda a subida;
//   - ambiente ou aparelho: a mesma conta com a contagem do RC102 nos minutos
//     com os dois em casa. RC102 subiu ≥ 60% do que o GMC-800 subiu: ambiente.
// Tudo em contagens (trocar a calibração não pode virar subida); a tela
// mostra µSv/h com a calibração de hoje. Recalcula junto da Saúde do tubo.

const (
	bsDays      = 120
	bsRefSkip   = 30 // os últimos 30 dias ficam fora da referência
	bsRecent    = 14
	bsRefMin    = 30
	bsRecentMin = 10
	bsDayMin    = 720 // minutos: 12 h
	bsRCDayMin  = 360 // minutos com os dois em casa
	bsRise      = 0.10
	bsAbove     = 0.05
	bsAboveN    = 10
	bsStep      = 0.08
	bsEnv       = 0.6
)

type BgSlowDay struct {
	Day   int64   `json:"day"`
	GM    float64 `json:"gm,omitempty"` // contagens por minuto, mediana do dia
	RC    float64 `json:"rc,omitempty"` // contagens por minuto do RC102 com os dois em casa
	Rain  bool    `json:"rain,omitempty"`
	Valid bool    `json:"valid,omitempty"`
}

type BgSlow struct {
	At    int64       `json:"at"`
	Today int64       `json:"today"`
	Moved int64       `json:"moved"` // dia marcado "mudei o aparelho de lugar" (0 = nenhum)
	Days  []BgSlowDay `json:"days"`

	State string `json:"state"` // ok, warn, step, na
	Left  int    `json:"left"`  // dias que faltam (na)

	RefN    int     `json:"refN"`
	RecN    int     `json:"recN"`
	RefGM   float64 `json:"refGm"`
	RecGM   float64 `json:"recGm"`
	RefRC   float64 `json:"refRc"`
	RecRC   float64 `json:"recRc"`
	RefUSv  float64 `json:"refUSv"`
	RecUSv  float64 `json:"recUSv"`
	Rise    float64 `json:"rise"`
	RCRise  float64 `json:"rcRise"`
	RCOK    bool    `json:"rcOk"` // há RC102 em casa nos dois trechos
	Above   int     `json:"above"`
	Env     bool    `json:"env"`
	Weeks   int     `json:"weeks"`
	StepDay int64   `json:"stepDay"`
	StepPct float64 `json:"stepPct"`
}

// GetBgSlow devolve a subida lenta do fundo; force recalcula agora.
func (a *App) GetBgSlow(force bool) (BgSlow, error) {
	now := time.Now()
	a.tubeMu.Lock()
	defer a.tubeMu.Unlock()
	if !force && a.bgSlowCache != nil && !tubeDue(time.UnixMilli(a.bgSlowCache.At), now) {
		return *a.bgSlowCache, nil
	}
	b, err := a.bgSlow(now)
	if err != nil {
		return b, err
	}
	a.bgSlowCache = &b
	return b, nil
}

// MarkBgMoved marca que o GMC-800 mudou de lugar no dia dayMs (0 = hoje):
// a referência do fundo recomeça nesse dia. Deixa uma nota no gráfico.
func (a *App) MarkBgMoved(dayMs int64) (BgSlow, error) {
	now := time.Now()
	t := now
	if dayMs > 0 {
		t = time.UnixMilli(dayMs)
	}
	if t.After(now) {
		return BgSlow{}, errors.New("o dia não pode ficar no futuro")
	}
	y, m, d := t.Date()
	day := time.Date(y, m, d, 0, 0, 0, 0, time.Local)
	a.mu.Lock()
	a.settings.BgMoved = day.UnixMilli()
	a.saveSettingsLocked()
	a.mu.Unlock()
	at := day.Add(12 * time.Hour)
	if dayMs <= 0 || at.After(now) {
		at = now
	}
	_, _ = a.SaveNote(Note{Time: at.UnixMilli(), Text: "Mudei o GMC-800 de lugar (o fundo de referência recomeça aqui)"})
	return a.GetBgSlow(true)
}

// ClearBgMoved desfaz a marca de mudança de lugar.
func (a *App) ClearBgMoved() (BgSlow, error) {
	a.mu.Lock()
	a.settings.BgMoved = 0
	a.saveSettingsLocked()
	a.mu.Unlock()
	return a.GetBgSlow(true)
}

func bsMedian(v []float64) float64 {
	if len(v) == 0 {
		return 0
	}
	return stats.Median(append([]float64(nil), v...))
}

func (a *App) bgSlow(now time.Time) (BgSlow, error) {
	y, m, d := now.Date()
	today := time.Date(y, m, d, 0, 0, 0, 0, time.Local)
	from := today.AddDate(0, 0, -(bsDays - 1))
	to := today.AddDate(0, 0, 1)
	a.mu.Lock()
	moved := a.settings.BgMoved
	tl := a.calTLLocked()
	rain := map[string]bool{}
	if a.rainDays != nil {
		for k, r := range a.rainDays.Days {
			rain[k] = r.MM >= yearRainMM
		}
	}
	a.mu.Unlock()
	out := BgSlow{At: now.UnixMilli(), Today: today.UnixMilli(), Moved: moved, Days: []BgSlowDay{}, State: "na"}
	idx := map[int64]int{}
	for t := from; t.Before(to); t = t.AddDate(0, 0, 1) {
		idx[t.UnixMilli()] = len(out.Days)
		out.Days = append(out.Days, BgSlowDay{Day: t.UnixMilli(), Rain: rain[t.Format("2006-01-02")]})
	}
	dayOf := func(t time.Time) (int, bool) {
		y, m, d := t.In(time.Local).Date()
		i, ok := idx[time.Date(y, m, d, 0, 0, 0, 0, time.Local).UnixMilli()]
		return i, ok
	}
	if a.store == nil {
		return out, nil
	}
	raw, err := a.store.Range(from, to)
	if err != nil {
		return out, err
	}
	inAlert := a.alertChecker()
	per := make([][]float64, len(out.Days))
	for _, p := range bgClean(a.homeOnly(raw)) {
		if inAlert(p.T) {
			continue
		}
		if i, ok := dayOf(p.T); ok {
			per[i] = append(per[i], bsCounts(p))
		}
	}
	rc := make([][]float64, len(out.Days))
	for _, p := range a.intercalPairs(from, to, true) {
		if inAlert(p.T) {
			continue
		}
		if i, ok := dayOf(p.T); ok {
			rc[i] = append(rc[i], p.CPS*60)
		}
	}
	for i := range out.Days {
		dd := &out.Days[i]
		if len(per[i]) >= bsDayMin {
			dd.GM = bsMedian(per[i])
			dd.Valid = !dd.Rain && dd.GM > 0
		}
		if len(rc[i]) >= bsRCDayMin {
			dd.RC = bsMedian(rc[i])
		}
	}
	bsAnalyze(&out)
	if out.RefGM > 0 {
		out.RefUSv = tl.USvAt(now, out.RefGM)
		out.RecUSv = tl.USvAt(now, out.RecGM)
	}
	return out, nil
}

// contagens do minuto sem a correção de tempo morto
func bsCounts(p store.Point) float64 {
	if p.Has {
		return p.Raw
	}
	return p.CPM
}

// bsAnalyze faz a conta com os dias já resumidos (separado para os testes).
func bsAnalyze(o *BgSlow) {
	n := len(o.Days)
	if n == 0 {
		return
	}
	rec0 := n - bsRecent
	var refGM, refRC, recGM, recRC []float64
	inRef := func(i int) bool {
		if o.Moved > 0 {
			return o.Days[i].Day >= o.Moved && i < rec0
		}
		return i < n-bsRefSkip
	}
	valid := 0
	for i, d := range o.Days {
		if !d.Valid {
			continue
		}
		valid++
		if i >= rec0 {
			recGM = append(recGM, d.GM)
			if d.RC > 0 {
				recRC = append(recRC, d.RC)
			}
		} else if inRef(i) {
			refGM = append(refGM, d.GM)
			if d.RC > 0 {
				refRC = append(refRC, d.RC)
			}
		}
	}
	o.RefN, o.RecN = len(refGM), len(recGM)
	o.RefGM, o.RecGM = bsMedian(refGM), bsMedian(recGM)
	o.RefRC, o.RecRC = bsMedian(refRC), bsMedian(recRC)
	if o.RefN < bsRefMin || o.RecN < bsRecentMin {
		o.State = "na"
		o.Left = max(bsRefMin-o.RefN, bsRecentMin-o.RecN, 1)
		if o.Moved == 0 && o.RefN < bsRefMin {
			// sem marca, a referência só fecha 30 dias depois dela
			o.Left = max(o.Left, bsRefMin+bsRefSkip-valid)
		}
		if o.RefGM <= 0 {
			// ainda dá para desenhar: usa todos os dias válidos como régua
			var all []float64
			for _, d := range o.Days {
				if d.Valid && (o.Moved == 0 || d.Day >= o.Moved) {
					all = append(all, d.GM)
				}
			}
			o.RefGM = bsMedian(all)
			var rc []float64
			for _, d := range o.Days {
				if d.Valid && d.RC > 0 && (o.Moved == 0 || d.Day >= o.Moved) {
					rc = append(rc, d.RC)
				}
			}
			o.RefRC = bsMedian(rc)
		}
		return
	}
	o.Rise = o.RecGM/o.RefGM - 1
	o.RCOK = len(refRC) >= 7 && len(recRC) >= 5
	if o.RCOK {
		o.RCRise = o.RecRC/o.RefRC - 1
	}
	for _, d := range o.Days[rec0:] {
		if d.Valid && d.GM/o.RefGM-1 > bsAbove {
			o.Above++
		}
	}
	// degrau: mediana dos 7 dias antes × 7 dias depois, nos últimos 60 dias
	win := func(a, b int) []float64 {
		var v []float64
		for i := max(a, 0); i < min(b, n); i++ {
			if o.Days[i].Valid {
				v = append(v, o.Days[i].GM)
			}
		}
		return v
	}
	mean := func(v []float64) float64 {
		s := 0.0
		for _, x := range v {
			s += x
		}
		return s / float64(len(v))
	}
	// o salto é medido pela mediana; o dia, pela média (a mediana fica
	// igual por alguns dias em volta do degrau e o ruído escolheria um deles)
	best, bestI, bestM := 0.0, -1, 0.0
	for i := max(n-60, 7); i < n-7; i++ {
		x, y := win(i-7, i), win(i, i+7)
		if len(x) < 4 || len(y) < 4 {
			continue
		}
		j := bsMedian(y)/bsMedian(x) - 1
		if j < bsStep {
			continue
		}
		if m := mean(y)/mean(x) - 1; m > bestM {
			best, bestI, bestM = j, i, m
		}
	}
	o.State = "ok"
	if o.Above >= bsAboveN && o.Rise >= bsRise {
		o.State = "warn"
	}
	if bestI >= 0 && o.Above >= bsAboveN && o.Rise >= bsStep*0.9 && best >= 0.75*o.Rise {
		o.State = "step"
		o.StepDay, o.StepPct = o.Days[bestI].Day, best
	}
	o.Env = o.RCOK && o.RCRise >= bsEnv*o.Rise
	// há quantas semanas a mediana de 7 dias passou de +5%
	o.Weeks = 2
	for k := max(n-60, 0); k < n; k++ {
		w := win(k-6, k+1)
		if len(w) >= 4 && bsMedian(w)/o.RefGM-1 > bsAbove {
			o.Weeks = max(2, int(math.Round(float64(n-k)/7)))
			break
		}
	}
}

// bsSort é usado pelos testes para conferir a ordem dos dias.
func bsSorted(d []BgSlowDay) bool {
	return sort.SliceIsSorted(d, func(i, j int) bool { return d[i].Day < d[j].Day })
}
