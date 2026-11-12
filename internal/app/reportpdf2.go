package app

import (
	"fmt"
	"math"
	"strings"
	"time"

	"radlog/internal/pdf"
)

// ---------- as páginas do relatório novo (2.68.0, spec ux/pdf.html) ----------

// renderReportPDF desenha duas vezes: a primeira só conta as páginas, para o
// rodapé dizer "página N de M".
func renderReportPDF(r reportData) []byte {
	if r.pages == 0 {
		p, _ := renderReport(r)
		r.pages = p.page
	}
	_, b := renderReport(r)
	return b
}

func renderReport(r reportData) (*reportPDF, []byte) {
	p := &reportPDF{d: pdf.New(), y: 50, page: 1, r: r}
	d := p.d
	p.header()
	if !r.HasData || r.St.N == 0 {
		d.Text(pm, p.y+12, pdf.Sans, 10, cMuted, "Sem leituras guardadas neste período.")
		p.footer()
		return p, d.Bytes(pdf.Tr(r.Title), "RadLog "+AppVersion)
	}
	p.pageSummary()
	if !r.Opts.Summary {
		p.newPage()
		p.pageDays()
		if r.Spec != nil || len(r.Trips) > 0 && p.anyTripSpec() {
			p.newPage()
			p.pageSpectrum()
		}
		p.newPage()
		p.annex()
	}
	p.footer()
	return p, d.Bytes(pdf.Tr(r.Title+" · "+r.Period), "RadLog "+AppVersion)
}

func (p *reportPDF) newPage() {
	p.footer()
	p.d.AddPage()
	p.page++
	p.y = 50
}

func (p *reportPDF) anyTripSpec() bool {
	for _, t := range p.r.Trips {
		if t.Spec != nil {
			return true
		}
	}
	return false
}

// levelOf é a cor da forma de nível de cada veredito.
func levelColor(lv string) pdf.RGB {
	switch lv {
	case "perigo":
		return cWarn
	case "alarme", "alto":
		return cAlarm
	case "atencao", "acima":
		return cAtt
	}
	return cAccent
}

// shape desenha a forma do nível (círculo verde, triângulo, losango, octógono).
func (p *reportPDF) shape(x, y, size float64, c pdf.RGB) {
	if c == cAccent {
		var pts [][2]float64
		r := size / 2
		for i := 0; i < 16; i++ {
			a := float64(i) / 16 * 2 * math.Pi
			pts = append(pts, [2]float64{x + r + r*math.Cos(a), y + r + r*math.Sin(a)})
		}
		p.d.Polygon(pts, c)
		return
	}
	p.levelShape(x, y, size, c)
}

// ---------- página 1: o resumo ----------

// sumSay é a frase do topo (a mesma regra do Resumo do app).
func (r reportData) sumSay() (lv, title string, parts []string) {
	s, rc := r.S, r.S.RC
	if rc != nil && rc.Minutes == 0 {
		rc = nil
	}
	n := func(g, c int) int { return g + c }
	var rd, ra, rt int
	if rc != nil {
		rd, ra, rt = rc.Danger, rc.Alarm, rc.Attention
	}
	danger, alarm, attn := n(s.Danger, rd), n(s.Alarm, ra), n(s.Attention, rt)
	gAbove := s.Minutes > 0 && s.BgOK && s.VsBgPct >= 20
	rAbove := rc != nil && rc.BgUSv > 0 && rc.VsBgPct >= 20
	switch {
	case danger > 0:
		lv, title = "perigo", "Teve perigo no período."
	case alarm > 0:
		lv, title = "alarme", "Teve alarme no período."
	case gAbove || rAbove:
		lv, title = "atencao", "Fundo acima do normal no período."
	case attn > 0:
		lv, title = "atencao", "Período normal, com alertas de atenção."
	default:
		lv, title = "seguro", "Período normal."
	}
	if s.Minutes > 0 && s.BgOK {
		parts = append(parts, pctNormal("Casa", s.VsBgPct))
	}
	if rc != nil && rc.BgUSv > 0 {
		parts = append(parts, pctNormal("RC102", rc.VsBgPct))
	}
	cov := s.Coverage
	if rc != nil {
		cov = rc.Coverage
	}
	parts = append(parts, fmtBR(cov*100, 0)+"% do tempo medido")
	if rc != nil && rc.AwayMinutes > 0 {
		parts = append(parts, "fora de casa: "+fmtMinutes(rc.AwayMinutes))
	}
	return
}

func pctNormal(lead string, p float64) string {
	switch {
	case math.Abs(p) < 5:
		return lead + " igual ao normal"
	case p > 0:
		return lead + " " + fmtBR(p, 0) + "% acima do normal"
	}
	return lead + " " + fmtBR(-p, 0) + "% abaixo do normal"
}

func alertsCountTxt(danger, alarm, attn int) string {
	var parts []string
	if danger > 0 {
		parts = append(parts, fmt.Sprintf("%d perigo", danger))
	}
	if alarm == 1 {
		parts = append(parts, "1 alarme")
	} else if alarm > 1 {
		parts = append(parts, fmt.Sprintf("%d alarmes", alarm))
	}
	if attn > 0 {
		parts = append(parts, fmt.Sprintf("%d atenção", attn))
	}
	if len(parts) == 0 {
		return "nenhum"
	}
	return strings.Join(parts, " · ")
}

// homeMean é o fundo da casa: a média do GMC-800 só com ele em casa.
func (r *reportData) homeMean() float64 {
	if r.HomeN > 0 {
		return r.HomeU
	}
	return r.USt.Mean
}

func (p *reportPDF) pageSummary() {
	d, r := p.d, p.r
	s, rc := r.S, r.S.RC
	if rc != nil && rc.Minutes == 0 {
		rc = nil
	}
	// a frase
	lv, title, parts := r.sumSay()
	c := levelColor(lv)
	box := cSoft
	p.d.Rect(pm, p.y, pdf.W-2*pm, 42, &box, nil)
	p.d.Rect(pm, p.y, 3, 42, &c, nil)
	p.shape(pm+12, p.y+9, 11, c)
	d.Text(pm+29, p.y+18, pdf.SansBold, 12, cInk, title)
	d.Text(pm+12, p.y+33, pdf.Sans, 8, cMuted, pdf.Fit(strings.Join(parts, " · "), pdf.Sans, 8, pdf.W-2*pm-24))
	p.y += 52

	// os quatro números
	tg := 8.0
	tw := (pdf.W - 2*pm - 3*tg) / 4
	th := 74.0
	x := func(i int) float64 { return pm + float64(i)*(tw+tg) }
	y := p.y
	// 1. sua dose, com o veredito
	if v := r.Verdict; v.OK {
		p.tile(x(0), y, tw, th, "Sua dose (RC102)", fmtBR(v.Dose, 2)+" µSv", "", pdf.MonoBold)
		vc := levelColor(v.Level)
		vt := map[string]string{"normal": "Normal", "acima": "Acima do normal", "alto": "Alto"}[v.Level] + " para " + fmtMinutes(int(r.To.Sub(r.From).Minutes()))
		p.shape(x(0)+8, y+38, 7, vc)
		d.Text(x(0)+18, y+44, pdf.SansBold, 7.5, vc, pdf.Fit(vt, pdf.SansBold, 7.5, tw-26))
		d.Text(x(0)+8, y+55, pdf.Sans, 7, cMuted, pdf.Fit("ficando em casa: "+fmtBR(v.Expected, 2)+" µSv ("+fmtSigned((v.Ratio-1)*100, 0)+"%)", pdf.Sans, 7, tw-16))
		d.Text(x(0)+8, y+65, pdf.Sans, 7, cMuted, pdf.Fit("no ritmo: "+fmtBR(v.YearMSv, 2)+" mSv/ano", pdf.Sans, 7, tw-16))
	} else if rc != nil {
		dose := rc.Dose
		if rc.DevOK {
			dose = rc.DevDose
		}
		p.tile(x(0), y, tw, th, "Sua dose (RC102)", fmtBR(dose, 2)+" µSv", "fundo de casa do RC102 ainda não calculado", pdf.MonoBold)
	} else {
		p.tile(x(0), y, tw, th, "Dose da casa (GMC-800)", r.Dose+" µSv", "≈ "+r.DoseYear+" mSv/ano no ritmo do período", pdf.MonoBold)
	}
	// 2. fundo da casa
	bgSub := "normal ainda não calculado"
	if s.BgOK {
		bgSub = pctNormal("GMC-800 ·", s.VsBgPct)
	}
	p.tile(x(1), y, tw, th, "Fundo da casa", fmtU(r.homeMean())+" µSv/h", bgSub, pdf.MonoBold)
	// 3. pico
	if rc != nil && rc.Max >= r.USt.Max {
		p.tile(x(2), y, tw, th, "Pico", fmtU(rc.Max)+" µSv/h", "RC102 · "+time.UnixMilli(rc.MaxAt).Format("02/01 15:04"), pdf.MonoBold)
	} else {
		mx, at := r.USt.Max, r.USt.MaxAt
		if r.AlertPeakMaxU > mx {
			mx, at = r.AlertPeakMaxU, r.alertPeakAt
		}
		p.tile(x(2), y, tw, th, "Pico", fmtU(mx)+" µSv/h", "GMC-800 · "+at.Format("02/01 15:04"), pdf.MonoBold)
	}
	// 4. alertas
	var rd, ra, rt int
	if rc != nil {
		rd, ra, rt = rc.Danger, rc.Alarm, rc.Attention
	}
	alSub := "sempre no nível seguro"
	if s.Danger+s.Alarm+s.Attention+rd+ra+rt > 0 {
		alSub = fmt.Sprintf("GMC-800: %s · RC102: %s", alertsCountTxt(s.Danger, s.Alarm, s.Attention), alertsCountTxt(rd, ra, rt))
	}
	p.tile(x(3), y, tw, th, "Alertas", alertsCountTxt(s.Danger+rd, s.Alarm+ra, s.Attention+rt), alSub, pdf.SansBold)
	p.y += th + 4

	if rc != nil && rc.Minutes > 0 && len(rc.Curve) > 0 {
		p.section("µSv/h AO LONGO DO PERÍODO: GMC-800 (CASA) E RC102 (VOCÊ)")
	} else {
		p.section("µSv/h AO LONGO DO PERÍODO: GMC-800 (CASA)")
	}
	p.chart(r)
	p.stripDays(r)

	// em resumo
	p.section("EM RESUMO")
	var lines []string
	if v := r.Verdict; v.OK {
		lines = append(lines, fmt.Sprintf("Sua dose no período foi %s µSv, %s do que você teria recebido ficando em casa (%s µSv).",
			fmtBR(v.Dose, 2), map[string]string{"normal": "perto", "acima": "acima", "alto": "bem acima"}[v.Level], fmtBR(v.Expected, 2)))
	}
	if s.BgOK {
		lines = append(lines, "O fundo da casa (GMC-800) ficou em "+fmtU(r.homeMean())+" µSv/h: "+strings.ToLower(pctNormal("", s.VsBgPct)[1:])+".")
	}
	if len(r.Trips) > 0 {
		lines = append(lines, fmt.Sprintf("Saídas de casa: %d (página 2).", len(r.Trips)))
	}
	if len(r.Episodes) > 0 || (rc != nil && rc.Danger+rc.Alarm+rc.Attention > 0) {
		lines = append(lines, "Os alertas estão listados na página 2 e detalhados no anexo (A4).")
	}
	for _, l := range lines {
		p.paragraph("• "+l, 8, cInk)
	}
	// os números são estimativas (auditoria de 12/11, R1)
	p.paragraph(pdfEstimate, 7, cMuted)
}

// pdfEstimate: o µSv/h dos aparelhos é estimativa, não dose oficial (R1).
const pdfEstimate = "Os µSv/h e as doses deste relatório são estimativas dos próprios aparelhos: eles medem a radiação no ambiente e são calibrados para o césio-137. Servem para comparar dias e lugares; não são uma dose oficial, que só vem de um dosímetro de trabalho lido por um laboratório."

// ---------- página 2: por dia, alertas, saídas, medições, chuva, anotações ----------

func (p *reportPDF) pageDays() {
	d, r := p.d, p.r
	cw := pdf.W - 2*pm
	p.section("POR DIA")
	var rows [][]string
	for _, dd := range r.Days2 {
		home, dose, pk, al := "—", "—", "—", "—"
		if dd.HasHome {
			home = fmtU(dd.HomeU)
		}
		if dd.HasDose {
			dose = fmtBR(dd.Dose, 2)
		}
		if dd.Peak > 0 {
			pk = fmtU(dd.Peak) + " · " + dd.PeakDev
		}
		if dd.Alerts > 0 {
			al = fmt.Sprint(dd.Alerts)
		}
		rows = append(rows, []string{dd.Label, home, dose, pk, al})
	}
	p.table([]string{"Dia", "Casa µSv/h", "Sua dose µSv", "Pico µSv/h", "Alertas"}, rows,
		[]float64{cw * .2, cw * .18, cw * .18, cw * .3, cw * .14},
		[]bool{false, true, true, true, true}, []bool{false, true, true, true, true}, nil)

	p.section("ALERTAS")
	if len(r.Alerts2) == 0 {
		p.need(20)
		d.Text(pm, p.y+12, pdf.Sans, 8.5, cMuted, "Nenhum alerta no período.")
		p.y += 18
	} else {
		rows = nil
		lvTxt := map[string]string{"atencao": "atenção", "alarme": "alarme", "perigo": "perigo"}
		for _, a := range r.Alerts2 {
			cf := a.Confirmed
			if cf == "" {
				cf = "—"
			}
			rows = append(rows, []string{a.Start.Format("02/01 15:04"), a.Dev, lvTxt[a.Level], fmtU(a.Peak), fmtSeconds(a.DurSec), cf})
		}
		al := r.Alerts2
		p.table([]string{"Quando", "Aparelho", "Nível", "Pico µSv/h", "Duração", "O outro aparelho"}, rows,
			[]float64{cw * .19, cw * .14, cw * .13, cw * .14, cw * .14, cw * .26},
			[]bool{false, false, false, true, true, false}, []bool{true, false, false, true, true, false},
			func(i int) *pdf.RGB {
				c := levelColor(al[i].Level)
				return &c
			})
	}

	if len(r.Trips) > 0 {
		p.section("SAÍDAS DE CASA")
		rows = nil
		for _, t := range r.Trips {
			dev, dose, pk := "RC102", "—", "—"
			if t.HasDose {
				dose = fmtBR(t.Dose, 2)
			}
			if t.GMC {
				dev = "GMC-800"
				if t.AvgU > 0 {
					dose = "média " + fmtU(t.AvgU) + " µSv/h"
				}
			}
			if t.Peak > 0 {
				pk = fmtU(t.Peak) + " às " + t.PeakAt.Format("15:04")
			}
			rows = append(rows, []string{t.Title, dev, t.From.Format("02/01 15:04") + " → " + t.To.Format("15:04"), fmtMinutes(int(t.To.Sub(t.From).Minutes())), dose, pk})
		}
		p.table([]string{"Saída", "Levou", "Horário", "Duração", "Dose µSv", "Pico µSv/h"}, rows,
			[]float64{cw * .22, cw * .1, cw * .2, cw * .1, cw * .18, cw * .2},
			[]bool{false, false, false, true, true, true}, []bool{false, false, true, true, true, true}, nil)
	}

	p.measures()

	if r.Rain != "" {
		p.section("CHUVA")
		p.paragraph(r.Rain+" no período (radar/modelo meteorológico).", 8, cInk)
	}

	if r.Opts.Notes && len(r.Notes) > 0 {
		p.section("ANOTAÇÕES")
		for _, n := range r.Notes {
			when := time.UnixMilli(n.Time).Format("02/01 15:04")
			if n.End > n.Time {
				when += "–" + time.UnixMilli(n.End).Format("15:04")
			}
			p.paragraph(when+" · "+n.Text, 8, cInk)
		}
	}
}

// ---------- página 3: espectro do RC102 ----------

func (p *reportPDF) pageSpectrum() {
	r := p.r
	if r.Spec != nil {
		p.section("ESPECTRO DO RC102 NO PERÍODO × EM CASA (MESMO TEMPO)")
		p.specChart(r.Spec, pm, pdf.W-2*pm, 200, "no período", "em casa (mesmo tempo)")
		p.specLines(r.SpecLines, r.Spec.BgSec > 0)
	}
	var rcTrips []reportTrip
	for _, t := range r.Trips {
		if !t.GMC {
			rcTrips = append(rcTrips, t)
		}
	}
	if len(rcTrips) > 0 {
		p.section("ESPECTRO DE CADA SAÍDA DE CASA")
		gap := 14.0
		w := (pdf.W - 2*pm - gap) / 2
		for i, t := range rcTrips {
			if i%2 == 0 {
				p.need(150)
			}
			x := pm
			if i%2 == 1 {
				x = pm + w + gap
				p.y = p.tripY
			} else {
				p.tripY = p.y
			}
			sub := t.From.Format("02/01 15:04") + " → " + t.To.Format("15:04")
			if t.HasDose {
				sub += " · " + fmtBR(t.Dose, 2) + " µSv"
			}
			p.d.Text(x, p.y+10, pdf.SansBold, 8, cInk, pdf.Fit(t.Title, pdf.SansBold, 8, w))
			p.d.Text(x, p.y+20, pdf.Sans, 7, cMuted, sub)
			p.y += 24
			if t.Spec == nil {
				p.d.Text(x, p.y+12, pdf.Sans, 8, cMuted, "sem espectro desta saída")
				p.y += 24
			} else {
				p.specChart(t.Spec, x, w, 100, "na saída", "em casa")
			}
			if i%2 == 0 {
				p.tripY2 = p.y
			} else {
				p.y = math.Max(p.y, p.tripY2)
			}
		}
		if len(rcTrips)%2 == 1 {
			p.y = math.Max(p.y, p.tripY2)
		}
	}
	p.paragraph("Espectro somado das leituras que o app guarda a cada minuto (e, com o RC102 longe do PC, da leitura que cobre o tempo todo fora). Em casa: no período, o tempo fora das saídas; numa saída, as 24 h antes dela. As curvas estão por segundo, para comparar tempos diferentes. Linhas de isótopo são uma pista, não uma identificação garantida.", 6.8, cMuted)
}

// specChart desenha o espectro (contagens/s em log) do período e o de casa.
func (p *reportPDF) specChart(sp *MeasureSpectrum, x0, w, h float64, objLbl, bgLbl string) {
	p.need(h + 24)
	bins := sp.bins()
	if len(bins) == 0 {
		return
	}
	pl := 26.0
	gx, gw := x0+pl, w-pl
	y0 := p.y + 4
	lo, hi := math.Inf(1), 0.0
	for _, b := range bins {
		for _, v := range []float64{b.Obj, b.Bg} {
			if v > 0 {
				lo, hi = math.Min(lo, v), math.Max(hi, v)
			}
		}
	}
	if hi <= 0 {
		return
	}
	dlo := math.Floor(math.Log10(lo))
	dhi := math.Max(dlo+1, math.Ceil(math.Log10(hi)))
	e0, e1 := bins[0].E0, bins[len(bins)-1].E1
	xe := func(e float64) float64 { return gx + (e-e0)/(e1-e0)*gw }
	yv := func(v float64) float64 {
		l := dlo
		if v > 0 {
			l = math.Max(dlo, math.Log10(v))
		}
		return y0 + h*(1-(l-dlo)/(dhi-dlo))
	}
	for dd := dlo; dd <= dhi; dd++ {
		yy := yv(math.Pow(10, dd))
		p.d.Line(gx, yy, gx+gw, yy, 0.3, cLine, false)
		lbl := fmtBR(math.Pow(10, dd), int(math.Max(0, -dd)))
		if dd >= 4 || dd < -3 {
			lbl = fmt.Sprintf("1e%d", int(dd))
		}
		p.d.TextRight(gx-3, yy+2.5, pdf.Mono, 6, cMuted, lbl)
	}
	for e := 500.0; e < e1; e += 500 {
		p.d.Line(xe(e), y0, xe(e), y0+h, 0.3, cLine, false)
		p.d.Text(xe(e)-8, y0+h+9, pdf.Mono, 6, cMuted, fmt.Sprint(int(e)))
	}
	p.d.TextRight(gx+gw, y0+h+9, pdf.Mono, 6, cMuted, "keV")
	// linhas achadas
	for _, pk := range sp.Peaks {
		if pk.KeV < e0 || pk.KeV > e1 {
			continue
		}
		xx := xe(pk.KeV)
		p.d.Line(xx, y0, xx, y0+h, 0.5, cAway, true)
		p.d.Text(xx+2, y0+7, pdf.Mono, 5.8, cAway, pk.Nuclide)
	}
	path := func(get func(b specBin) float64) [][][2]float64 {
		var segs [][][2]float64
		var seg [][2]float64
		for _, b := range bins {
			v := get(b)
			if !(v > 0) {
				if len(seg) > 0 {
					segs = append(segs, seg)
				}
				seg = nil
				continue
			}
			seg = append(seg, [2]float64{xe(b.E0), yv(v)}, [2]float64{xe(b.E1), yv(v)})
		}
		if len(seg) > 0 {
			segs = append(segs, seg)
		}
		return segs
	}
	if sp.BgSec > 0 {
		for _, sg := range path(func(b specBin) float64 { return b.Bg }) {
			p.d.Polyline(sg, 0.6, cMuted)
		}
	}
	for _, sg := range path(func(b specBin) float64 { return b.Obj }) {
		p.d.Polyline(sg, 0.9, cRC)
	}
	p.d.Rect(gx, y0, gw, h, nil, &cLine)
	ly := y0 + h + 20
	p.d.Line(gx, ly-2.5, gx+12, ly-2.5, 1, cRC, false)
	t1 := objLbl + " (" + fmtSecondsLong(sp.ObjSec) + ")"
	p.d.Text(gx+15, ly, pdf.Sans, 6.8, cMuted, t1)
	if sp.BgSec > 0 {
		lx := gx + 15 + pdf.TextWidth(t1, pdf.Sans, 6.8) + 12
		p.d.Line(lx, ly-2.5, lx+12, ly-2.5, 0.8, cMuted, false)
		p.d.Text(lx+15, ly, pdf.Sans, 6.8, cMuted, bgLbl)
	}
	p.y = ly + 6
}

func fmtSecondsLong(sec float64) string {
	m := int(math.Round(sec / 60))
	if m < 1 {
		return fmt.Sprintf("%d s", int(math.Round(sec)))
	}
	return fmtMinutes(m)
}

// specLines é a lista "mais que em casa" debaixo do espectro do período.
func (p *reportPDF) specLines(ls []TripLine, hasHome bool) {
	if len(ls) == 0 {
		p.paragraph("Nenhuma linha gama conhecida se destacou no período.", 7.5, cMuted)
		return
	}
	var more, less, same []string
	for _, l := range ls {
		t := fmt.Sprintf("%s %s keV", l.Nuclide, fmtBR(l.KeV, 0))
		if l.More {
			t += fmt.Sprintf(" (%s%%)", fmtSigned(l.Pct, 0))
			more = append(more, t)
		} else if l.Less {
			t += fmt.Sprintf(" (%s%%)", fmtSigned(l.Pct, 0))
			less = append(less, t)
		} else {
			same = append(same, t)
		}
	}
	if !hasHome {
		p.paragraph("Linhas que apareceram: "+strings.Join(append(more, same...), " · ")+".", 7.5, cInk)
		return
	}
	if len(more) > 0 {
		p.paragraph("Mais que em casa (acima de 3σ): "+strings.Join(more, " · ")+".", 7.5, cInk)
	} else {
		p.paragraph("Nenhuma linha passou de 3σ acima de casa.", 7.5, cInk)
	}
	if len(less) > 0 {
		p.paragraph("Menos que em casa: "+strings.Join(less, " · ")+".", 7.5, cMuted)
	}
	if len(same) > 0 {
		p.paragraph("Iguais a casa: "+strings.Join(same, " · ")+".", 7.5, cMuted)
	}
}

// ---------- anexo técnico ----------

var annexTOC = []string{
	"A1 · Estatística da contagem (CPM), por dia na contagem e o RC102 em detalhe",
	"A2 · Fundo da casa: agora × 7 dias × 30 dias",
	"A3 · Tempo acima dos limites operacionais",
	"A4 · Episódios de alerta (30 min antes e depois)",
	"A5 · Lacunas (sem leitura)",
	"A6 · Histórico disponível (maturidade)",
	"A7 · Configurações e método",
	"A8 · Limitações",
}

func (p *reportPDF) annex() {
	d := p.d
	d.Text(pm, p.y+6, pdf.SansBold, 14, cInk, "Anexo técnico")
	p.y += 16
	for _, t := range annexTOC {
		d.Text(pm+6, p.y+9, pdf.Sans, 8, cMuted, t)
		p.y += 11
	}
	p.y += 4
	d.Line(pm, p.y, pdf.W-pm, p.y, 0.8, cInk, false)
	p.y += 4
	p.annexStats()
	p.annexDays()
	if rc := p.r.S.RC; rc != nil && rc.Minutes > 0 {
		p.annexRC()
	}
	p.annexBg()
	p.annexAbove()
	p.annexEpisodes()
	p.annexGaps()
	p.maturity()
	if p.r.Opts.Cfg {
		p.cfgPeriod()
	}
	p.annexConfig()
	p.annexLimits()
}
