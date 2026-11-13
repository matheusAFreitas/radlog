package app

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"radlog/internal/core/intercal"
	"radlog/internal/store"
)

// ---------- intercalibração com o Radiacode e vigia semanal (ideia 6) ----------
//
// "Intercalibrar com o Radiacode" marca o começo de uma coleta de 24 h com
// os dois aparelhos encostados. No fim, a conta do core/intercal dá o fator
// novo do GMC; aplicar vira uma versão da calibração (origem
// "intercalibração"). O contador nunca é mexido.
//
// A vigia compara, nos minutos em que os dois estavam em casa, a razão de
// contagens com o normal das 4 primeiras semanas da calibração em uso.

const (
	intercalFile = "intercalibracao.json"
	intercalDur  = 24 * time.Hour
	srcIntercal  = "intercalibração"
	rcNMin       = 20 // minuto do RC102 com menos leituras = pedaço (conectou/caiu)
)

type intercalData struct {
	Start   int64            `json:"inicio,omitempty"` // ms; 0 = nenhuma coleta
	End     int64            `json:"fim,omitempty"`
	K0      float64          `json:"fatorAntes,omitempty"` // fator em uso quando começou
	Result  *intercal.Result `json:"resultado,omitempty"`
	Applied bool             `json:"aplicada,omitempty"`
	Skipped bool             `json:"dispensada,omitempty"` // "Agora não"
}

// IntercalView é o estado para a tela.
type IntercalView struct {
	State   string           `json:"state"` // parado | coletando | pronta | poucos
	Start   int64            `json:"start"`
	End     int64            `json:"end"`
	Minutes int              `json:"minutes"` // minutos com os dois até agora
	Need    int              `json:"need"`
	CurK    float64          `json:"curK"`
	K0      float64          `json:"k0"`
	Result  *intercal.Result `json:"result,omitempty"`
	Pct     float64          `json:"pct"` // quanto o GMC lia acima (+) ou abaixo (−) do RC102, %
	Applied bool             `json:"applied"`
	Skipped bool             `json:"skipped"`
}

func (a *App) intercalPath() string { return filepath.Join(a.dataDir, intercalFile) }

func (a *App) loadIntercal() intercalData {
	var d intercalData
	if a.dataDir == "" {
		return d
	}
	a.readStateJSON(a.intercalPath(), "intercalibração", &d)
	return d
}

func (a *App) saveIntercal(d intercalData) error {
	if a.dataDir == "" {
		return nil
	}
	b, err := json.MarshalIndent(d, "", "  ")
	if err != nil {
		return err
	}
	return writeFileAtomic(a.intercalPath(), b, 0o644)
}

// gmcCounts são as contagens do minuto (sem a correção de tempo morto) e se
// o minuto serve para a conta: sem saturação nem interferência, e minuto de
// verdade (ao vivo ou das gravações por segundo da memória).
func gmcCounts(p store.Point) (float64, bool) {
	if p.HasObs(store.ObsSaturation) || p.HasObs(store.ObsInterference) {
		return 0, false
	}
	if p.Src != store.SrcLive && !p.HasObs(store.ObsSeconds) {
		return 0, false
	}
	if p.Has {
		return p.Raw, true
	}
	return p.CPM, true
}

// intercalPairs junta os minutos com os dois aparelhos em [from, to). Com
// homeOnly, só os minutos com os dois em casa e fora das medições.
func (a *App) intercalPairs(from, to time.Time, homeOnly bool) []intercal.Pair {
	if a.store == nil || !to.After(from) {
		return nil
	}
	rcs := a.rcRange(from, to)
	if len(rcs) == 0 {
		return nil
	}
	pts, err := a.store.Range(from, to)
	if err != nil {
		return nil
	}
	gm := make(map[int64]float64, len(pts))
	for _, p := range pts {
		if c, ok := gmcCounts(p); ok {
			gm[p.T.Truncate(time.Minute).Unix()] = c
		}
	}
	var where func(time.Time) (string, string)
	var measuring func(time.Time) bool
	if homeOnly {
		where = a.whereChecker()
		measuring = a.measuringChecker()
	}
	out := make([]intercal.Pair, 0, len(rcs))
	for _, r := range rcs {
		if r.N < rcNMin {
			continue
		}
		c, ok := gm[r.T.Truncate(time.Minute).Unix()]
		if !ok {
			continue
		}
		if homeOnly {
			if g, rc := where(r.T); g != WhereHome || rc != WhereHome || measuring(r.T) {
				continue
			}
		}
		out = append(out, intercal.Pair{T: r.T, Counts: c, USvh: r.USvh, CPS: r.CPS})
	}
	return out
}

func (a *App) curK() float64 {
	a.mu.Lock()
	defer a.mu.Unlock()
	return a.convCalLocked().USv(100) / 100
}

// GetIntercal devolve o estado da intercalibração (e termina a conta quando
// as 24 h passaram).
func (a *App) GetIntercal() IntercalView {
	d := a.loadIntercal()
	v := IntercalView{Need: int(intercalDur / time.Minute), CurK: a.curK(), Start: d.Start, End: d.End, K0: d.K0,
		Applied: d.Applied, Skipped: d.Skipped}
	if d.Start == 0 {
		v.State = "parado"
		return v
	}
	if d.Result == nil {
		start, end := time.UnixMilli(d.Start), time.UnixMilli(d.End)
		now := time.Now()
		if now.Before(end) {
			v.State = "coletando"
			v.Minutes = len(a.intercalPairs(start, now, false))
			return v
		}
		r, err := intercal.Compute(a.intercalPairs(start, end, false))
		if err != nil {
			v.State = "poucos"
			v.Minutes = r.Minutes
			return v
		}
		d.Result = &r
		_ = a.saveIntercal(d)
		a.logEvent(fmt.Sprintf("Intercalibração com o RC102 terminou: %s µSv/h por CPM (±%s%%, %d minutos)",
			fmtBR(r.K, 5), fmtBR(r.Rel95*100, 1), r.Minutes))
	}
	v.State = "pronta"
	v.Result = d.Result
	v.Minutes = d.Result.Minutes
	if d.Result.K > 0 && d.K0 > 0 {
		v.Pct = (d.K0/d.Result.K - 1) * 100
	}
	return v
}

// StartIntercal começa uma coleta de 24 h (os dois aparelhos encostados).
func (a *App) StartIntercal() (IntercalView, error) {
	now := time.Now()
	d := intercalData{Start: now.UnixMilli(), End: now.Add(intercalDur).UnixMilli(), K0: a.curK()}
	if err := a.saveIntercal(d); err != nil {
		return IntercalView{}, err
	}
	a.logEvent("Intercalibração com o RC102: coleta de 24 h começou (os dois aparelhos encostados)")
	return a.GetIntercal(), nil
}

// CancelIntercal para a coleta ou esquece o resultado.
func (a *App) CancelIntercal() error {
	d := a.loadIntercal()
	if d.Start != 0 && d.Result == nil {
		a.logEvent("Intercalibração com o RC102: coleta cancelada")
	}
	return a.saveIntercal(intercalData{})
}

// ApplyIntercal usa o fator da intercalibração a partir de agora (ou, com
// skip, deixa para depois: "Agora não").
func (a *App) ApplyIntercal(skip bool) (IntercalView, error) {
	d := a.loadIntercal()
	if d.Result == nil {
		return a.GetIntercal(), errors.New("a intercalibração ainda não terminou")
	}
	if skip {
		d.Skipped = true
		_ = a.saveIntercal(d)
		return a.GetIntercal(), nil
	}
	note := "com o RC102, coleta de " + time.UnixMilli(d.Start).Format("02/01")
	if err := a.setGMCFactor(d.Result.K, srcIntercal, note); err != nil {
		return a.GetIntercal(), err
	}
	d.Applied, d.Skipped = true, false
	_ = a.saveIntercal(d)
	return a.GetIntercal(), nil
}

// ConcordView é a vigia para a tela.
type ConcordView struct {
	intercal.Watch
	Text string `json:"text"`
}

// GetConcord diz se os dois aparelhos continuam concordando.
func (a *App) GetConcord() ConcordView {
	a.mu.Lock()
	var since int64
	if last := a.calLastLocked(); last != nil {
		since = last.Since
	}
	a.mu.Unlock()
	now := time.Now()
	s := time.UnixMilli(since)
	if since == 0 {
		// calibração desde o começo: o aprendizado começa no primeiro minuto do Radiacode
		s = now.Add(-365 * 24 * time.Hour)
		if f, ok := a.rcFirstMinute(); ok {
			s = f
		}
	}
	learnEnd := s.Add(intercal.LearnDays * 24 * time.Hour)
	var ps []intercal.Pair
	if now.Before(learnEnd) {
		ps = a.intercalPairs(s, now, true)
	} else {
		ps = a.intercalPairs(s, learnEnd, true)
		from := now.Add(-14 * 24 * time.Hour)
		if from.Before(learnEnd) {
			from = learnEnd
		}
		ps = append(ps, a.intercalPairs(from, now, true)...)
	}
	w := intercal.WatchState(ps, s, now)
	return ConcordView{Watch: w, Text: concordText(w)}
}

func concordText(w intercal.Watch) string {
	switch w.State {
	case intercal.WatchLearning:
		return fmt.Sprintf("Aprendendo o normal dos dois em casa: %s h juntos até agora; a vigia começa em %s.",
			fmtBR(float64(w.LearnedMin)/60, 0), w.LearnUntil.Format("02/01"))
	case intercal.WatchAgree:
		return fmt.Sprintf("Os dois concordam: nos últimos 7 dias a razão ficou %s%% do normal.", signedPct(w.Last.Pct))
	case intercal.WatchOneWeek:
		return fmt.Sprintf("Na última semana a razão ficou %s%% do normal. Se continuar assim mais uma semana, vira aviso.", signedPct(w.Last.Pct))
	case intercal.WatchDrift:
		return fmt.Sprintf("Os dois estão se afastando: %s%% do normal há duas semanas. Pode ser o tubo do GMC, um dos aparelhos mudado de lugar ou o RC102 guardado em outro canto. Vale intercalibrar de novo.", signedPct(w.Last.Pct))
	}
	return "Poucos minutos com os dois em casa para comparar (precisa de 12 h juntos na semana)."
}

func signedPct(p float64) string {
	if p >= 0 {
		return "+" + fmtBR(p, 1)
	}
	return "−" + fmtBR(-p, 1)
}

// rcFirstMinute é o primeiro minuto gravado do Radiacode.
func (a *App) rcFirstMinute() (time.Time, bool) {
	ents, err := os.ReadDir(a.rcDir())
	if err != nil {
		return time.Time{}, false
	}
	for _, e := range ents { // AAAA-MM.csv, em ordem
		n := e.Name()
		if len(n) != 11 || n[4] != '-' || filepath.Ext(n) != ".csv" {
			continue
		}
		m, err := time.ParseInLocation("2006-01", n[:7], time.Local)
		if err != nil {
			continue
		}
		if rs := a.rcRange(m, m.AddDate(0, 1, 0)); len(rs) > 0 {
			return rs[0].T, true
		}
	}
	return time.Time{}, false
}
