package app

import (
	"fmt"
	"math"
	"sort"
	"time"
)

// ---------- fundo e "quão incomum" é a leitura ----------
//
// Contagens de um tubo Geiger seguem a estatística de Poisson: num fundo de N
// contagens por minuto, a variação natural de um minuto para outro é de cerca
// de √N. Então (CPM − fundo) / √fundo diz quantos "desvios" a leitura está
// acima do normal: até ~2 é ruído, 3 ou mais quase certamente é real.

const (
	bgWindow   = 30 * 24 * time.Hour
	bgRefresh  = 10 * time.Minute
	bgMinCount = 30 // minutos de dado para confiar na mediana
)

// backgroundCPM devolve a mediana do CPM real dos últimos 30 dias (o demo não
// entra). Recalcula no máximo a cada 10 minutos.
func (a *App) backgroundCPM() (float64, bool) {
	a.mu.Lock()
	if !a.bgAt.IsZero() && time.Since(a.bgAt) < bgRefresh {
		v, ok := a.bgCPM, a.bgOK
		a.mu.Unlock()
		return v, ok
	}
	a.mu.Unlock()

	var v float64
	ok := false
	if a.store != nil {
		now := time.Now()
		pts, err := a.store.Range(now.Add(-bgWindow), now.Add(time.Minute))
		pts = a.homeOnly(pts) // fora de casa não é o fundo da casa
		if err == nil && len(pts) >= bgMinCount {
			cpms := make([]float64, len(pts))
			for i, p := range pts {
				cpms[i] = p.CPM
			}
			v, ok = median(cpms), true
		}
	}
	a.mu.Lock()
	a.bgCPM, a.bgOK, a.bgAt = v, ok, time.Now()
	a.mu.Unlock()
	return v, ok
}

func median(v []float64) float64 {
	s := append([]float64(nil), v...)
	sort.Float64s(s)
	n := len(s)
	if n == 0 {
		return 0
	}
	if n%2 == 1 {
		return s[n/2]
	}
	return (s[n/2-1] + s[n/2]) / 2
}

// sigmaOf diz quantos desvios de Poisson o CPM está acima (ou abaixo) do fundo.
func sigmaOf(cpm, bg float64) *float64 {
	if bg <= 0 {
		return nil
	}
	z := (cpm - bg) / math.Sqrt(bg)
	return &z
}

// fillSigma completa a leitura com o fundo e o desvio.
func (a *App) fillSigma(r *Reading) {
	if bg, ok := a.backgroundCPM(); ok && bg > 0 {
		r.BgCPM = bg
		r.Sigma = sigmaOf(float64(r.CPM), bg)
	}
}

// ---------- padrão por hora do dia ----------

// Heatmap é a média de CPM por dia (linhas) e hora (colunas).
type Heatmap struct {
	Days   []int64      `json:"days"` // início de cada dia (ms), do mais antigo ao mais novo
	Values [][]*float64 `json:"values"`
	USv    [][]*float64 `json:"usv"`    // mesma média em µSv/h
	Counts [][]int      `json:"counts"` // minutos medidos em cada hora (em casa)
	// fora de casa: minutos e média dessas horas (não entram no padrão)
	AwayCounts [][]int      `json:"awayCounts"`
	AwayValues [][]*float64 `json:"awayValues"`
	// alertas que pegaram cada hora; chave "dia-hora" (índices de Days e 0–23)
	Alerts   map[string][]HeatAlert `json:"alerts"`
	HourAvg  []*float64             `json:"hourAvg"` // média de cada hora em todos os dias
	Min      *float64               `json:"min"`
	Max      *float64               `json:"max"`
	PeakHour int                    `json:"peakHour"` // -1 se não houver dados
	LowHour  int                    `json:"lowHour"`
	Minutes  int                    `json:"minutes"`
}

// HeatAlert é um alerta que aconteceu (ao menos em parte) numa hora do mapa.
type HeatAlert struct {
	Level string  `json:"level"` // "atencao" ou "alarme"
	Time  int64   `json:"time"`
	Peak  float64 `json:"peak"`
	Retro bool    `json:"retro"`
}

// heatAlerts marca as horas em que houve alerta do GMC.
func (a *App) heatAlerts(first time.Time, days int) map[string][]HeatAlert {
	out := map[string][]HeatAlert{}
	end := first.AddDate(0, 0, days)
	now := time.Now()
	a.mu.Lock()
	var list []Event
	for _, e := range a.events {
		if e.Kind == kindAlert && e.Device != devRC { // o mapa é do GMC: alertas do RadiaCode ficam de fora
			c := e
			c.Active = a.isActiveLocked(e.ID)
			list = append(list, c)
		}
	}
	a.mu.Unlock()
	for _, e := range list {
		start := time.UnixMilli(e.Time)
		stop := time.UnixMilli(e.End)
		switch {
		case e.Active:
			stop = now
		case e.End == 0:
			stop = start.Add(time.Minute) // fim não registrado
		}
		if !stop.After(start) {
			stop = start.Add(time.Second)
		}
		if !stop.After(first) || !start.Before(end) {
			continue
		}
		peak := e.Peak
		if peak == 0 {
			peak = e.CPM
		}
		ha := HeatAlert{Level: e.Level, Time: e.Time, Peak: peak, Retro: e.Retro}
		if ha.Level == "" {
			ha.Level = "atencao"
		}
		// percorre as horas que o alerta tocou
		h := time.Date(start.Year(), start.Month(), start.Day(), start.Hour(), 0, 0, 0, start.Location())
		if h.Before(first) {
			h = first
		}
		for ; h.Before(stop) && h.Before(end); h = h.Add(time.Hour) {
			y, m, d := h.Date()
			day := time.Date(y, m, d, 0, 0, 0, 0, h.Location())
			i := int(math.Round(day.Sub(first).Hours() / 24))
			if i < 0 || i >= days {
				continue
			}
			k := fmt.Sprintf("%d-%d", i, h.Hour())
			out[k] = append(out[k], ha)
		}
	}
	return out
}

// GetHeatmap monta o mapa de calor dos últimos dias (1 a 60).
func (a *App) GetHeatmap(days int) (Heatmap, error) {
	if days < 1 || days > 60 {
		days = 14
	}
	now := time.Now()
	y, m, d := now.Date()
	today := time.Date(y, m, d, 0, 0, 0, 0, now.Location())
	return a.heatmapFrom(today.AddDate(0, 0, -(days-1)), days)
}

// GetHeatmapMonth monta o mapa de calor de um mês (month 1–12). O mês atual
// vai só até hoje; mês futuro volta vazio.
func (a *App) GetHeatmapMonth(year, month int) (Heatmap, error) {
	if month < 1 || month > 12 {
		return Heatmap{}, fmt.Errorf("mês inválido: %d", month)
	}
	now := time.Now()
	first := time.Date(year, time.Month(month), 1, 0, 0, 0, 0, now.Location())
	end := first.AddDate(0, 1, 0)
	y, m, d := now.Date()
	tomorrow := time.Date(y, m, d+1, 0, 0, 0, 0, now.Location())
	if end.After(tomorrow) {
		end = tomorrow
	}
	days := int(math.Round(end.Sub(first).Hours() / 24))
	if days < 1 {
		return Heatmap{Days: []int64{}, Values: [][]*float64{}, USv: [][]*float64{}, Counts: [][]int{},
			AwayCounts: [][]int{}, AwayValues: [][]*float64{}, Alerts: map[string][]HeatAlert{},
			HourAvg: make([]*float64, 24), PeakHour: -1, LowHour: -1}, nil
	}
	return a.heatmapFrom(first, days)
}

// GetDataStart devolve o início (ms) do mês da leitura mais antiga guardada,
// ou 0 se ainda não há nada. Serve para a tela não deixar voltar além disso.
func (a *App) GetDataStart() int64 {
	t, ok := a.store.FirstTime()
	if !ok {
		return 0
	}
	return time.Date(t.Year(), t.Month(), 1, 0, 0, 0, 0, t.Location()).UnixMilli()
}

// heatmapFrom monta o mapa de calor de days dias a partir de first (meia-noite).
func (a *App) heatmapFrom(first time.Time, days int) (Heatmap, error) {
	pts, err := a.points(first, first.AddDate(0, 0, days))
	if err != nil {
		return Heatmap{}, err
	}
	a.mu.Lock()
	tl := a.calTLLocked()
	a.mu.Unlock()
	sum := make([][24]float64, days)
	usum := make([][24]float64, days)
	cnt := make([][24]int, days)
	asum := make([][24]float64, days)
	acnt := make([][24]int, days)
	away := a.awayChecker()
	measuring := a.measuringChecker()
	var hsum [24]float64
	var hcnt [24]int
	h := Heatmap{PeakHour: -1, LowHour: -1, Minutes: len(pts)}
	for _, p := range pts {
		py, pm, pd := p.T.Date()
		day := time.Date(py, pm, pd, 0, 0, 0, 0, p.T.Location())
		i := int(math.Round(day.Sub(first).Hours() / 24)) // arredonda por causa de horário de verão
		if i < 0 || i >= days {
			continue
		}
		hr := p.T.Hour()
		if measuring(p.T) {
			continue // objeto encostado no contador: não é o padrão da casa
		}
		if away(p.T) {
			asum[i][hr] += p.CPM
			acnt[i][hr]++
			continue
		}
		sum[i][hr] += p.CPM
		usum[i][hr] += tl.USv(p) // pela calibração que valia nesse minuto
		cnt[i][hr]++
		hsum[hr] += p.CPM
		hcnt[hr]++
	}
	for i := 0; i < days; i++ {
		h.Days = append(h.Days, first.AddDate(0, 0, i).UnixMilli())
		row := make([]*float64, 24)
		urow := make([]*float64, 24)
		crow := make([]int, 24)
		arow := make([]int, 24)
		avrow := make([]*float64, 24)
		for hr := 0; hr < 24; hr++ {
			crow[hr] = cnt[i][hr]
			arow[hr] = acnt[i][hr]
			if acnt[i][hr] > 0 {
				av := asum[i][hr] / float64(acnt[i][hr])
				avrow[hr] = &av
			}
			if cnt[i][hr] == 0 {
				continue
			}
			v := sum[i][hr] / float64(cnt[i][hr])
			row[hr] = &v
			u := usum[i][hr] / float64(cnt[i][hr])
			urow[hr] = &u
			if h.Min == nil || v < *h.Min {
				mn := v
				h.Min = &mn
			}
			if h.Max == nil || v > *h.Max {
				mx := v
				h.Max = &mx
			}
		}
		h.Values = append(h.Values, row)
		h.USv = append(h.USv, urow)
		h.Counts = append(h.Counts, crow)
		h.AwayCounts = append(h.AwayCounts, arow)
		h.AwayValues = append(h.AwayValues, avrow)
	}
	h.Alerts = a.heatAlerts(first, days)
	h.HourAvg = make([]*float64, 24)
	for hr := 0; hr < 24; hr++ {
		if hcnt[hr] == 0 {
			continue
		}
		v := hsum[hr] / float64(hcnt[hr])
		h.HourAvg[hr] = &v
		if h.PeakHour < 0 || v > *h.HourAvg[h.PeakHour] {
			h.PeakHour = hr
		}
		if h.LowHour < 0 || v < *h.HourAvg[h.LowHour] {
			h.LowHour = hr
		}
	}
	return h, nil
}
