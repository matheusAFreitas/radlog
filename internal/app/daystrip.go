package app

import (
	"sort"
	"time"
)

// ---------- fita do dia (2.42) ----------
//
// O dia inteiro numa faixa: cada pedacinho é 10 minutos, pintado pelo nível
// da média do aparelho que estava em cada papel (roles.go): a linha "casa" e a
// linha "com você". Embaixo, os trechos de Fora de casa, a chuva e as marcas
// (alertas e anotações). Sem dado fica vazio (a tela pinta de cinza escuro,
// nunca de verde). A semana é a mesma coisa, um dia por linha.

const stripStep = 10 * time.Minute

// StripSlot é um pedaço de 10 min.
type StripSlot struct {
	T       int64   `json:"t"`
	Home    string  `json:"home,omitempty"` // seguro, atencao, alarme, perigo; "" = sem dado
	HomeUSv float64 `json:"homeUSv,omitempty"`
	HomeDev string  `json:"homeDev,omitempty"`
	Pers    string  `json:"pers,omitempty"`
	PersUSv float64 `json:"persUSv,omitempty"`
	PersDev string  `json:"persDev,omitempty"`
	Away    bool    `json:"away,omitempty"`
	Rain    float64 `json:"rain,omitempty"` // mm/h, só quando chove
	// para a dose por origem (alara.go): minutos medidos e fundo do aparelho
	persN  int
	persBg float64
}

// StripMark é um alerta ou uma anotação na fita.
type StripMark struct {
	T      int64  `json:"t"`
	Kind   string `json:"kind"` // alerta, nota
	Level  string `json:"level,omitempty"`
	Device string `json:"device,omitempty"` // "" = GMC
	Text   string `json:"text"`
}

// DayStrip é a resposta de GetDayStrip.
type DayStrip struct {
	From   int64       `json:"from"`
	To     int64       `json:"to"`
	StepMs int64       `json:"stepMs"`
	Slots  []StripSlot `json:"slots"`
	Marks  []StripMark `json:"marks"`
}

// stripBands são os limites (µSv/h) de cada aparelho: o GMC pelas faixas do
// app, o Radiacode pelos alarmes dele (rcLimits).
type stripBands struct{ attn, alarm, danger float64 }

func (b stripBands) level(usv float64) string {
	switch {
	case b.danger > 0 && usv >= b.danger:
		return "perigo"
	case b.alarm > 0 && usv >= b.alarm:
		return "alarme"
	case b.attn > 0 && usv >= b.attn:
		return "atencao"
	}
	return "seguro"
}

func (a *App) gmcBands() stripBands {
	c := a.withUSv(Chart{})
	return stripBands{c.AttentionUSv, c.AlarmUSv, c.DangerUSv}
}

// GetDayStrip monta a fita de um trecho (normalmente um dia, de meia-noite a
// meia-noite), em pedaços de 10 min.
func (a *App) GetDayStrip(fromMs, toMs int64) (DayStrip, error) {
	from := time.UnixMilli(fromMs).Truncate(stripStep)
	to := time.UnixMilli(toMs)
	if max := from.Add(8 * 24 * time.Hour); to.After(max) {
		to = max
	}
	out := DayStrip{From: from.UnixMilli(), To: to.UnixMilli(), StepMs: stripStep.Milliseconds(), Slots: []StripSlot{}, Marks: []StripMark{}}
	if !to.After(from) {
		return out, nil
	}
	n := int((to.Sub(from) + stripStep - 1) / stripStep)
	out.Slots = make([]StripSlot, n)
	for i := range out.Slots {
		out.Slots[i].T = from.Add(time.Duration(i) * stripStep).UnixMilli()
	}
	bands := map[string]stripBands{"gmc": a.gmcBands()}
	if l := a.rcLimits(); l.Attn > 0 {
		bands[devRC] = stripBands{l.Attn, l.Alarm, l.Danger}
	}
	// minuto a minuto de cada aparelho
	type acc struct {
		sum float64
		n   int
	}
	type key struct {
		slot int
		dev  string
	}
	sums := map[key]*acc{}
	mins := map[string]map[int64]float64{}
	bgOf := map[string]float64{}
	for _, dev := range []string{"gmc", devRC} {
		pts, bg, _, err := a.doseSeries(dev, from, to)
		bgOf[dev] = bg
		if err != nil {
			return out, err
		}
		m := map[int64]float64{}
		for _, p := range pts {
			m[p.T.Truncate(time.Minute).UnixMilli()] = p.USv
		}
		mins[dev] = m
	}
	roles := a.rolesChecker()
	for dev, m := range mins {
		for t, v := range m {
			i := int((t - out.From) / out.StepMs)
			if i < 0 || i >= n {
				continue
			}
			pers, persFill, home, homeFill := roles(time.UnixMilli(t))
			for _, r := range [][3]string{{"h", home, homeFill}, {"p", pers, persFill}} {
				// o principal vale; o que preenche só entra se o principal não mediu
				if dev != r[1] && !(dev == r[2] && !hasMin(mins[r[1]], t)) {
					continue
				}
				k := key{i, r[0] + dev}
				if sums[k] == nil {
					sums[k] = &acc{}
				}
				sums[k].sum += v
				sums[k].n++
			}
		}
	}
	for i := range out.Slots {
		s := &out.Slots[i]
		for _, role := range []string{"h", "p"} {
			best, bn, bdev := 0.0, 0, ""
			for _, dev := range []string{"gmc", devRC} {
				if c := sums[key{i, role + dev}]; c != nil && c.n > bn {
					best, bn, bdev = c.sum/float64(c.n), c.n, dev
				}
			}
			if bn == 0 {
				continue
			}
			lv := bands[bdev].level(best)
			if role == "h" {
				s.Home, s.HomeUSv, s.HomeDev = lv, best, bdev
			} else {
				s.Pers, s.PersUSv, s.PersDev = lv, best, bdev
				s.persN, s.persBg = bn, bgOf[bdev]
			}
		}
	}
	// fora de casa e chuva
	a.mu.Lock()
	periods := append([]Period{}, a.periods...)
	a.mu.Unlock()
	for i := range out.Slots {
		t0, t1 := out.Slots[i].T, out.Slots[i].T+out.StepMs
		for _, p := range periods {
			if p.From < t1 && periodEnd(p) > t0 {
				out.Slots[i].Away = true
				break
			}
		}
	}
	for k, r := range a.rainSlots(from, to) {
		if !r.wet() {
			continue
		}
		if i := int((k*1000 - out.From) / out.StepMs); i >= 0 && i < n {
			out.Slots[i].Rain = r.mmh
		}
	}
	// alertas e anotações
	for _, e := range a.GetEvents(kindAlert) {
		if e.Time < out.From || e.Time >= out.To {
			continue
		}
		out.Marks = append(out.Marks, StripMark{T: e.Time, Kind: "alerta", Level: e.Level, Device: e.Device, Text: e.Text})
	}
	for _, nt := range a.GetNotes(out.From, out.To) {
		out.Marks = append(out.Marks, StripMark{T: nt.Time, Kind: "nota", Text: nt.Text})
	}
	sort.Slice(out.Marks, func(i, j int) bool { return out.Marks[i].T < out.Marks[j].T })
	return out, nil
}

func hasMin(m map[int64]float64, t int64) bool { _, ok := m[t]; return ok }
