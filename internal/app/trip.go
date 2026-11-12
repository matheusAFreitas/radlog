package app

import (
	"errors"
	"math"
	"sort"
	"time"
)

// ---------- resumo da saída (2.67.0) ----------
//
// Quando você volta para casa, o resumo junta o que o RC102 viu enquanto
// você estava fora: dose, média, pico, o gráfico, o espectro da saída
// comparado com o de casa e como o aparelho estava. Tudo é calculado na hora
// com o que já está gravado; só o período (Fora de casa) diz o intervalo.
// Nunca GPS: o lugar é o nome que você dá para a saída.

// TripLine é uma linha gama da saída comparada com a de casa.
type TripLine struct {
	Nuclide string  `json:"nuclide"`
	Group   string  `json:"group"`
	KeV     float64 `json:"kev"`
	Pct     float64 `json:"pct"`  // saída ÷ casa − 1, em %, na janela do pico
	More    bool    `json:"more"` // passou de 3σ acima de casa
	HasHome bool    `json:"hasHome"`
}

// TripEvent é um aviso do aparelho no intervalo.
type TripEvent struct {
	T     int64  `json:"t"`
	Event string `json:"event"`
}

// TripSummary é o resumo de uma saída.
type TripSummary struct {
	Period      Period  `json:"period"`
	From        int64   `json:"from"`
	To          int64   `json:"to"`
	Open        bool    `json:"open"`
	DefaultName bool    `json:"defaultName"` // nome padrão: a tela mostra "Saída de ter 10/11"
	RC          *RcSum  `json:"rc"`
	StepMs      int64   `json:"stepMs"`
	HomeUSv     float64 `json:"homeUSv"` // fundo de casa no dia (µSv/h)
	HomeSrc     string  `json:"homeSrc"` // "gmc" | "rc" | ""
	// espectro: Obj = saída, Bg = casa (24 h antes), Peaks = linhas da saída
	Spec      *MeasureSpectrum `json:"spec"`
	SpecRows  int              `json:"specRows"`
	Lines     []TripLine       `json:"lines"`
	HasBat    bool             `json:"hasBat"`
	Bat0      float64          `json:"bat0"`
	Bat1      float64          `json:"bat1"`
	HasTemp   bool             `json:"hasTemp"`
	TempMin   float64          `json:"tempMin"`
	TempMax   float64          `json:"tempMax"`
	TempMaxAt int64            `json:"tempMaxAt"`
	Events    []TripEvent      `json:"events"`
}

// isTrip: o RC102 foi junto (é ele que mede a saída).
func isTrip(p Period) bool {
	_, rc := periodWhere(p)
	return rc == WhereMe
}

// isDefaultPeriodName: o nome ainda é o que o app deu.
func isDefaultPeriodName(name string) bool {
	if name == "" || name == periodNameOut || name == "Fora de casa" {
		return true
	}
	ws := []string{WhereHome, WhereMe, WhereOff}
	for _, g := range ws {
		for _, r := range ws {
			if name == periodName(g, r) {
				return true
			}
		}
	}
	return false
}

// tripStep é o passo do gráfico: minuto a minuto até 6 h, depois até ~360 pontos.
func tripStep(span time.Duration) time.Duration {
	st := time.Minute
	if span > 6*time.Hour {
		st = (span / 360).Round(time.Minute)
		if st < time.Minute {
			st = time.Minute
		}
	}
	return st
}

// GetTripSummary monta o resumo da saída id.
func (a *App) GetTripSummary(id int64) (TripSummary, error) {
	a.mu.Lock()
	var p Period
	found := false
	for _, q := range a.periods {
		if q.ID == id {
			p, found = q, true
		}
	}
	a.mu.Unlock()
	if !found {
		return TripSummary{}, errors.New("período não encontrado")
	}
	return a.tripSummary(p, time.Now()), nil
}

func (a *App) tripSummary(p Period, now time.Time) TripSummary {
	out := TripSummary{Period: p, From: p.From, To: p.To, Open: p.To == 0, DefaultName: isDefaultPeriodName(p.Name), Lines: []TripLine{}, Events: []TripEvent{}}
	if out.Open || out.To > now.UnixMilli() {
		out.To = now.UnixMilli()
	}
	from, to := time.UnixMilli(out.From), time.UnixMilli(out.To)
	if !to.After(from) {
		return out
	}
	step := tripStep(to.Sub(from))
	n := int((to.Sub(from) + step - 1) / step)
	out.StepMs = step.Milliseconds()
	out.RC = a.rcSummary(from, to, step, n)

	// fundo de casa no dia: mediana do GMC-800 em casa, pela calibração de agora
	d0 := time.Date(from.Year(), from.Month(), from.Day(), 0, 0, 0, 0, from.Location())
	if med, nm := a.homeMedian(d0, d0.AddDate(0, 0, 1)); nm >= 60 && med > 0 {
		a.mu.Lock()
		out.HomeUSv = a.convCalLocked().USv(med)
		a.mu.Unlock()
		out.HomeSrc = "gmc"
	} else if out.RC != nil && out.RC.BgUSv > 0 {
		out.HomeUSv, out.HomeSrc = out.RC.BgUSv, "rc"
	}

	a.tripSpectrum(&out)
	a.tripDevice(&out)
	return out
}

// rcSpecEach passa por cada linha gravada do espectrograma que toca
// [fromMs, toMs], sem guardar todas (um mês são dezenas de milhares).
func (a *App) rcSpecEach(fromMs, toMs int64, fn func(r rcSpecRow)) {
	if a.dataDir == "" {
		return
	}
	a.rcSpecFlush()
	names, days := a.rcSpecFiles()
	for i, name := range names {
		// uma linha que cobre a saída inteira pode ser gravada no dia seguinte
		if days[i].UnixMilli() > toMs+24*3600e3 || days[i].AddDate(0, 0, 1).UnixMilli() < fromMs {
			continue
		}
		for _, r := range a.rcSpecRead(name) {
			if r.T > fromMs && float64(r.T)-r.S*1000 < float64(toMs) {
				fn(r)
			}
		}
	}
}

// tripMinHome: com menos que isso de casa nas 24 h antes, procura em 7 dias.
const tripMinHome = 3600.0

// tripSpectrum soma as linhas do espectrograma que caem na saída (com o
// RC102 longe do PC, a primeira linha depois que ele volta cobre a saída
// inteira) e compara com o espectro de casa antes da saída.
func (a *App) tripSpectrum(out *TripSummary) { a.spectrumVsHome(out, false) }

// spectrumVsHome: com inside (o relatório de um período), "em casa" são as
// linhas do próprio período fora das saídas; sem casa suficiente dentro,
// vale o de antes, como na saída.
func (a *App) spectrumVsHome(out *TripSummary, inside bool) {
	inTrip := func(r rcSpecRow) bool {
		r0, r1 := float64(r.T)-r.S*1000, float64(r.T)
		ov := math.Min(r1, float64(out.To)) - math.Max(r0, float64(out.From))
		// conta se a maior parte da linha cai na saída (ou se ela cobre quase toda a saída)
		return ov > 0 && (ov >= 0.5*(r1-r0) || ov >= 0.8*float64(out.To-out.From))
	}
	away := a.rcAwayChecker()
	var (
		nch           int
		ref           [3]float64
		obj, bgIn     []uint32
		objSec, bgIns float64
		bg24, bg7     []uint32
		bg24s, bg7sec float64
	)
	addTo := func(dst []uint32, c []uint32) {
		for i, v := range c {
			dst[i] += v
		}
	}
	// a linha da volta só é gravada quando o RC102 reconecta: procura até 1 dia depois
	a.rcSpecEach(out.From-7*24*3600e3, out.To+24*3600e3, func(r rcSpecRow) {
		if len(r.C) == 0 {
			return
		}
		if nch == 0 {
			nch = len(r.C)
			obj, bg24, bg7, bgIn = make([]uint32, nch), make([]uint32, nch), make([]uint32, nch), make([]uint32, nch)
		}
		if len(r.C) != nch {
			return
		}
		if inTrip(r) {
			addTo(obj, r.C)
			objSec += r.S
			ref = r.A
			out.SpecRows++
			if inside && !away(time.UnixMilli(r.T-int64(r.S*500))) {
				addTo(bgIn, r.C)
				bgIns += r.S
			}
			return
		}
		r0 := r.T - int64(r.S*1000)
		if r.T > out.From || away(time.UnixMilli(r.T-int64(r.S*500))) {
			return
		}
		if r0 >= out.From-7*24*3600e3 {
			addTo(bg7, r.C)
			bg7sec += r.S
		}
		if r0 >= out.From-24*3600e3 {
			addTo(bg24, r.C)
			bg24s += r.S
		}
	})
	if objSec <= 0 || ref[1] <= 0 {
		out.SpecRows = 0
		return
	}
	bg, bgSec := bg24, bg24s
	if inside && bgIns >= tripMinHome {
		bg, bgSec = bgIn, bgIns
	} else if bgSec < tripMinHome {
		bg, bgSec = bg7, bg7sec
	}
	sp := &MeasureSpectrum{A: ref, Obj: obj, ObjSec: objSec}
	// as linhas da saída: o SNIP no espectro dela sozinho, como no Acumulado
	alone := &MeasureSpectrum{A: ref, Obj: obj, ObjSec: objSec, Bg: make([]uint32, nch), BgSec: 1}
	sp.Peaks = alone.findPeaks()
	if bgSec > 0 {
		sp.Bg, sp.BgSec = bg, bgSec
		// "mais que em casa": as linhas que passam de 3σ acima de casa
		vs := &MeasureSpectrum{A: ref, Obj: obj, ObjSec: objSec, Bg: bg, BgSec: bgSec}
		more := vs.findPeaks()
		seen := map[float64]bool{}
		add := func(pk SpecPeak) {
			if seen[pk.KeV] {
				return
			}
			seen[pk.KeV] = true
			l := TripLine{Nuclide: pk.Nuclide, Group: pk.Group, KeV: pk.KeV}
			for _, m := range more {
				l.More = l.More || m.KeV == pk.KeV
			}
			if o, h := tripWindow(sp, pk.KeV); h > 0 {
				l.Pct, l.HasHome = (o/h-1)*100, true
			}
			out.Lines = append(out.Lines, l)
		}
		for _, m := range more {
			add(m)
		}
		for _, pk := range sp.Peaks {
			add(pk)
		}
		sort.SliceStable(out.Lines, func(i, j int) bool {
			if out.Lines[i].More != out.Lines[j].More {
				return out.Lines[i].More
			}
			return out.Lines[i].KeV < out.Lines[j].KeV
		})
	} else {
		for _, pk := range sp.Peaks {
			out.Lines = append(out.Lines, TripLine{Nuclide: pk.Nuclide, Group: pk.Group, KeV: pk.KeV})
		}
	}
	out.Spec = sp
}

// tripWindow são as contagens por segundo da saída e de casa em volta de um pico.
func tripWindow(s *MeasureSpectrum, kev float64) (obj, bg float64) {
	h := 0.6 * specFWHM(kev)
	for i := range s.Obj {
		if e := s.energy(i); e >= kev-h && e < kev+h {
			obj += float64(s.Obj[i])
			if i < len(s.Bg) {
				bg += float64(s.Bg[i])
			}
		}
	}
	if s.ObjSec > 0 {
		obj /= s.ObjSec
	}
	if s.BgSec > 0 {
		bg /= s.BgSec
	}
	return
}

// tripWarn são os avisos do aparelho que entram no resumo: alarmes 1 e 2,
// fora de escala, temperatura e bateria (ligar, carregar etc. não entram).
var tripWarn = map[int]bool{2: true, 6: true, 9: true, 10: true, 11: true, 12: true, 13: true, 14: true,
	15: true, 16: true, 20: true, 21: true, 22: true}

// tripDevice: bateria, temperatura e avisos do RC102 no intervalo (rclog).
func (a *App) tripDevice(out *TripSummary) {
	pg := a.GetRcLog(out.From, out.To, []string{"raro", "evento"}, 2000, 0)
	items := pg.Items
	sort.SliceStable(items, func(i, j int) bool { return items[i].T < items[j].T })
	for _, it := range items {
		switch it.Kind {
		case "raro":
			if it.HasBat {
				if !out.HasBat {
					out.Bat0 = it.Battery
				}
				out.HasBat, out.Bat1 = true, it.Battery
			}
			if it.HasTemp {
				if !out.HasTemp || it.Temp < out.TempMin {
					out.TempMin = it.Temp
				}
				if !out.HasTemp || it.Temp > out.TempMax {
					out.TempMax, out.TempMaxAt = it.Temp, it.T
				}
				out.HasTemp = true
			}
		case "evento":
			if tripWarn[it.Code] {
				out.Events = append(out.Events, TripEvent{T: it.T, Event: it.Event})
			}
		}
	}
}
