package app

import (
	"encoding/json"
	"fmt"
	"hash/fnv"
	"os"
	"path/filepath"
	"time"

	"radlog/internal/core/stats"
)

// ---------- calendário do ano (2.53) ----------
//
// Um resumo por dia: o fundo de casa (mediana dos minutos em casa do
// GMC-800, em µSv/h pela calibração de cada minuto), a dose do RC102 no
// dia, se ele saiu de casa, o alarme mais grave com o pico, e a chuva.
// O pesado (ler os minutos) fica guardado por mês e só refaz quando o mês
// muda: arquivo de leituras novo, trecho fora de casa, calibração ou
// medição de objeto. O mês de hoje refaz a cada minuto gravado.

// YearDay é o resumo de um dia (data local).
type YearDay struct {
	Day    int64   `json:"day"`              // meia-noite local (ms)
	N      int     `json:"n"`                // minutos em casa do GMC-800
	Bg     float64 `json:"bg,omitempty"`     // µSv/h, mediana do dia
	DoseN  int     `json:"doseN,omitempty"`  // minutos medidos do RC102
	Dose   float64 `json:"dose,omitempty"`   // µSv no dia (RC102)
	Away   bool    `json:"away,omitempty"`   // RC102 fora de casa em parte do dia
	Level  string  `json:"lvl,omitempty"`    // atencao, alarme, perigo
	Peak   float64 `json:"peak,omitempty"`   // µSv/h
	PeakAt int64   `json:"peakAt,omitempty"` // ms
	PeakRC bool    `json:"peakRC,omitempty"` // o alerta foi do RC102
	MM     float64 `json:"mm,omitempty"`     // chuva do dia
	Rain   bool    `json:"rain,omitempty"`   // choveu de verdade (≥ 0,5 mm)
}

// YearCal é a resposta de GetYearCal.
type YearCal struct {
	Year  int       `json:"year"`
	First int       `json:"first"` // primeiro ano com dado
	Today int64     `json:"today"` // meia-noite de hoje (ms)
	Days  []YearDay `json:"days"`  // 1º de janeiro até hoje (ou 31/12)
}

const yearRainMM = 0.5 // mm no dia: abaixo disso foi garoa

type yearMonth struct {
	sig  string
	days []YearDay
}

var yearLevelRank = map[string]int{"atencao": 1, "alarme": 2, "perigo": 3}

// GetYearCal monta o calendário de um ano.
func (a *App) GetYearCal(year int) (YearCal, error) {
	now := time.Now()
	out := YearCal{Year: year, First: now.Year(), Days: []YearDay{}}
	if f := a.gmcFirst(); !f.IsZero() && f.Year() < out.First {
		out.First = f.Year()
	}
	if f := a.rcFirstYear(); f > 0 && f < out.First {
		out.First = f
	}
	if year < out.First || year > now.Year() {
		out.Year = now.Year()
	}
	y, m, d := now.Date()
	today := time.Date(y, m, d, 0, 0, 0, 0, time.Local)
	out.Today = today.UnixMilli()
	for mo := time.January; mo <= time.December; mo++ {
		m0 := time.Date(out.Year, mo, 1, 0, 0, 0, 0, time.Local)
		if m0.After(today) {
			break
		}
		days, err := a.yearMonth(m0, today)
		if err != nil {
			return out, err
		}
		out.Days = append(out.Days, days...)
	}
	a.yearMarks(out.Days)
	return out, nil
}

// yearMonth devolve os dias de um mês (até hoje), do guardado se nada mudou.
func (a *App) yearMonth(m0, today time.Time) ([]YearDay, error) {
	key := m0.Format("2006-01")
	sig := a.yearSig(m0)
	a.yearMu.Lock()
	if c, ok := a.yearCache[key]; ok && c.sig == sig {
		a.yearMu.Unlock()
		return append([]YearDay(nil), c.days...), nil
	}
	a.yearMu.Unlock()
	days, err := a.yearMonthCalc(m0, today)
	if err != nil {
		return nil, err
	}
	a.yearMu.Lock()
	if a.yearCache == nil {
		a.yearCache = map[string]yearMonth{}
	}
	a.yearCache[key] = yearMonth{sig, days}
	a.yearMu.Unlock()
	return append([]YearDay(nil), days...), nil
}

// yearSig muda quando algo que entra na conta do mês muda.
func (a *App) yearSig(m0 time.Time) string {
	h := fnv.New64a()
	file := func(p string) {
		if fi, err := os.Stat(p); err == nil {
			fmt.Fprintf(h, "%s|%d|%d;", fi.Name(), fi.Size(), fi.ModTime().UnixNano())
		} else {
			h.Write([]byte("-;"))
		}
	}
	if a.store != nil {
		file(filepath.Join(a.store.Dir(), "leituras", m0.Format("2006-01")+".csv"))
	}
	if a.dataDir != "" {
		file(filepath.Join(a.rcDir(), m0.Format("2006-01")+".csv"))
	}
	a.mu.Lock()
	b, _ := json.Marshal([]any{a.periods, a.calHist, a.settings.Levels.DeadTimeOn, len(a.measurements)})
	a.mu.Unlock()
	h.Write(b)
	if now := time.Now(); now.Year() == m0.Year() && now.Month() == m0.Month() {
		fmt.Fprintf(h, "|%s", now.Format("2006-01-02")) // o dia de hoje entra no mês
	}
	return fmt.Sprintf("%x", h.Sum64())
}

// yearMonthCalc lê os minutos do mês e resume cada dia.
func (a *App) yearMonthCalc(m0, today time.Time) ([]YearDay, error) {
	m1 := m0.AddDate(0, 1, 0)
	end := m1
	if t1 := today.AddDate(0, 0, 1); t1.Before(end) {
		end = t1
	}
	var days []YearDay
	idx := map[int64]int{}
	for d := m0; d.Before(end); d = d.AddDate(0, 0, 1) {
		idx[d.UnixMilli()] = len(days)
		days = append(days, YearDay{Day: d.UnixMilli()})
	}
	dayOf := func(t time.Time) (int, bool) {
		y, m, d := t.In(time.Local).Date()
		i, ok := idx[time.Date(y, m, d, 0, 0, 0, 0, time.Local).UnixMilli()]
		return i, ok
	}
	// fundo de casa: mediana dos minutos em casa do GMC-800, em µSv/h
	if a.store != nil {
		raw, err := a.store.Range(m0, end)
		if err != nil {
			return nil, err
		}
		a.mu.Lock()
		tl := a.calTLLocked()
		a.mu.Unlock()
		per := make([][]float64, len(days))
		cpm := make([][]float64, len(days))
		for _, p := range bgClean(a.homeOnly(raw)) {
			if i, ok := dayOf(p.T); ok {
				per[i] = append(per[i], tl.USv(p))
				cpm[i] = append(cpm[i], p.CPM)
			}
		}
		for i, v := range per {
			if len(v) > 0 {
				days[i].N = len(v)
				// mediana interpolada: a comum anda em degraus de ~5% com ~20 CPM
				days[i].Bg = stats.MedianCountsScaled(cpm[i], v)
			}
		}
	}
	// dose do RC102: soma dos minutos medidos; encostado num objeto, o
	// minuto vale o fundo de casa do mês (como na Dose)
	if a.dataDir != "" {
		pts := a.rcRange(m0, end)
		if len(pts) > 0 {
			onObj := a.onObjectChecker(devRC)
			away := a.rcAwayChecker()
			var home []float64
			for _, p := range pts {
				if !away(p.T) && !onObj(p.T) {
					home = append(home, p.USvh)
				}
			}
			bg := 0.0
			if len(home) > 0 {
				bg = stats.Median(home)
			}
			for _, p := range pts {
				i, ok := dayOf(p.T)
				if !ok {
					continue
				}
				u := p.USvh
				if bg > 0 && u > bg && onObj(p.T) {
					u = bg
				}
				days[i].Dose += u / 60
				days[i].DoseN++
			}
		}
		// fora de casa: algum trecho de 10 min do dia com o RC102 fora
		away := a.rcAwayChecker()
		for i := range days {
			d0 := time.UnixMilli(days[i].Day)
			for t := d0; t.Before(d0.AddDate(0, 0, 1)); t = t.Add(10 * time.Minute) {
				if away(t) {
					days[i].Away = true
					break
				}
			}
		}
	}
	return days, nil
}

// yearMarks põe os alarmes e a chuva (baratos: refaz sempre).
func (a *App) yearMarks(days []YearDay) {
	if len(days) == 0 {
		return
	}
	idx := map[string]int{}
	for i, d := range days {
		idx[time.UnixMilli(d.Day).Format("2006-01-02")] = i
	}
	for _, e := range a.GetEvents(kindAlert) {
		at := e.PeakAt
		if at == 0 {
			at = e.Time
		}
		i, ok := idx[time.UnixMilli(at).Format("2006-01-02")]
		if !ok || e.Demo {
			continue
		}
		peak := e.PeakUSv
		if e.Device == devRC {
			peak = e.Peak
		}
		if peak == 0 {
			peak = e.USv
		}
		d := &days[i]
		r, cur := yearLevelRank[e.Level], yearLevelRank[d.Level]
		if r == 0 || r < cur || (r == cur && peak <= d.Peak) {
			continue
		}
		d.Level, d.Peak, d.PeakAt, d.PeakRC = e.Level, peak, at, e.Device == devRC
	}
	a.mu.Lock()
	if a.rainDays != nil {
		for k, r := range a.rainDays.Days {
			if i, ok := idx[k]; ok {
				days[i].MM = r.MM
				days[i].Rain = r.MM >= yearRainMM
			}
		}
	}
	a.mu.Unlock()
}

// rcFirstYear é o ano do primeiro arquivo de minutos do RC102 (0 = nenhum).
func (a *App) rcFirstYear() int {
	if a.dataDir == "" {
		return 0
	}
	ents, err := os.ReadDir(a.rcDir())
	if err != nil {
		return 0
	}
	first := 0
	for _, e := range ents {
		var y, m int
		if n, _ := fmt.Sscanf(e.Name(), "%4d-%2d.csv", &y, &m); n == 2 && (first == 0 || y < first) {
			first = y
		}
	}
	return first
}
