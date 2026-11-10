package app

import (
	"fmt"
	"math"
	"sort"
	"strings"
	"time"

	"radlog/internal/pdf"
)

// ---------- relatório em PDF ----------

var (
	cInk    = pdf.Hex("#15191C")
	cMuted  = pdf.Hex("#5B666E")
	cLine   = pdf.Hex("#DDE3E7")
	cSoft   = pdf.Hex("#F4F6F7")
	cAccent = pdf.Hex("#1F9D5B")
	cAtt    = pdf.Hex("#C98A12")
	cAlarm  = pdf.Hex("#D06A1E")
	cWarn   = pdf.Hex("#C8383E")
	cWhite  = pdf.Hex("#FFFFFF")
	cBand   = pdf.Hex("#A9DDBF") // faixa mín–máx do gráfico
	cGap    = pdf.Hex("#ECEFF1") // sem dado
	cAttBg  = pdf.Hex("#FCEFD3")
	cAlmBg  = pdf.Hex("#FBE1CC")
	cDngBg  = pdf.Hex("#F8D5D6")
	cExp    = pdf.Hex("#8A6BBE") // curva de Poisson
)

const (
	pm   = 40.0 // margem
	pBot = pdf.H - 50
)

type reportPDF struct {
	d    *pdf.Doc
	y    float64
	page int
	r    reportData
}

func (p *reportPDF) footer() {
	p.d.Line(pm, pdf.H-36, pdf.W-pm, pdf.H-36, 0.5, cLine, false)
	p.d.Text(pm, pdf.H-24, pdf.Sans, 7.5, cMuted, pdf.Fit("RadLog "+p.r.Version+" · "+p.r.Period, pdf.Sans, 7.5, pdf.W-2*pm-60))
	p.d.TextRight(pdf.W-pm, pdf.H-24, pdf.Sans, 7.5, cMuted, fmt.Sprintf("página %d", p.page))
}

func (p *reportPDF) need(h float64) {
	if p.y+h <= pBot {
		return
	}
	p.footer()
	p.d.AddPage()
	p.page++
	p.y = 50
}

func (p *reportPDF) section(title string) {
	p.need(85) // título + cabeçalho de tabela + uma linha, para o título não ficar sozinho
	p.y += 18
	p.d.Text(pm, p.y, pdf.SansBold, 9, cMuted, title)
	p.y += 8
}

func (p *reportPDF) tile(x, y, w, h float64, k, v, sub string, vf pdf.Font) {
	p.d.Rect(x, y, w, h, nil, &cLine)
	p.d.Text(x+8, y+13, pdf.Sans, 7.5, cMuted, k)
	size := 14.0
	if vf == pdf.SansBold {
		size = 11
	}
	for size > 8 && pdf.TextWidth(v, vf, size) > w-16 {
		size -= 0.5
	}
	p.d.Text(x+8, y+31, vf, size, cInk, pdf.Fit(v, vf, size, w-16))
	for i, ln := range wrapText(sub, pdf.Sans, 7, w-16, 2) {
		p.d.Text(x+8, y+43+float64(i)*9, pdf.Sans, 7, cMuted, ln)
	}
}

// wrapText quebra em linhas que cabem na largura (no máximo max linhas).
func wrapText(s string, f pdf.Font, size, w float64, max int) []string {
	s = pdf.Tr(s) // traduz a frase inteira antes de quebrar
	words := strings.Fields(s)
	for _, wd := range words {
		pdf.MarkTranslated(wd)
	}
	var lines []string
	cur := ""
	for _, wd := range words {
		try := strings.TrimSpace(cur + " " + wd)
		if pdf.TextWidth(try, f, size) <= w || cur == "" {
			cur = try
			continue
		}
		lines = append(lines, cur)
		cur = wd
	}
	if cur != "" {
		lines = append(lines, cur)
	}
	for _, ln := range lines {
		pdf.MarkTranslated(ln)
	}
	if len(lines) > max {
		rest := strings.Join(lines[max-1:], " ")
		pdf.MarkTranslated(rest)
		lines = append(lines[:max-1], pdf.Fit(rest, f, size, w))
	}
	return lines
}

// table desenha cabeçalho e linhas, quebrando página quando precisa.
// cols: largura de cada coluna; right: alinhada à direita; mono: fonte mono.
func (p *reportPDF) table(head []string, rows [][]string, cols []float64, right []bool, mono []bool, rowColor func(i int) *pdf.RGB) {
	drawHead := func() {
		x := pm
		for i, h := range head {
			if right[i] {
				p.d.TextRight(x+cols[i]-4, p.y+10, pdf.SansBold, 7.5, cMuted, h)
			} else {
				p.d.Text(x+4, p.y+10, pdf.SansBold, 7.5, cMuted, h)
			}
			x += cols[i]
		}
		p.y += 15
		p.d.Line(pm, p.y, pdf.W-pm, p.y, 0.6, cLine, false)
	}
	p.need(40)
	drawHead()
	for ri, row := range rows {
		if p.y+15 > pBot {
			p.need(100)
			drawHead()
		}
		x := pm
		for i, c := range row {
			f := pdf.Sans
			if mono[i] {
				f = pdf.Mono
			}
			col := cInk
			if i == 1 && rowColor != nil {
				if rc := rowColor(ri); rc != nil {
					p.levelShape(x+3.5, p.y+4.5, 7, *rc)
					p.d.Text(x+13, p.y+11, f, 8, col, pdf.Fit(c, f, 8, cols[i]-17))
					x += cols[i]
					continue
				}
			}
			if right[i] {
				p.d.TextRight(x+cols[i]-4, p.y+11, f, 8, col, c)
			} else {
				p.d.Text(x+4, p.y+11, f, 8, col, pdf.Fit(c, f, 8, cols[i]-8))
			}
			x += cols[i]
		}
		p.y += 15
		p.d.Line(pm, p.y, pdf.W-pm, p.y, 0.3, cLine, false)
	}
}

// levelShape desenha a forma do nível (ideia 12) numa caixa size × size em
// (x, y): triângulo (atenção), losango (alarme) e octógono com faixa
// (perigo). Imprime bem em preto e branco. A forma sai da cor do nível.
func (p *reportPDF) levelShape(x, y, size float64, c pdf.RGB) {
	s := size / 16
	pt := func(xs ...float64) [][2]float64 {
		var out [][2]float64
		for i := 0; i+1 < len(xs); i += 2 {
			out = append(out, [2]float64{x + xs[i]*s, y + xs[i+1]*s})
		}
		return out
	}
	switch c {
	case cWarn:
		p.d.Polygon(pt(5.1, 1, 10.9, 1, 15, 5.1, 15, 10.9, 10.9, 15, 5.1, 15, 1, 10.9, 1, 5.1), c)
		p.d.Rect(x+4*s, y+6.9*s, 8*s, 2.2*s, &cWhite, nil)
	case cAlarm:
		p.d.Polygon(pt(8, .6, 15.4, 8, 8, 15.4, .6, 8), c)
	default:
		p.d.Polygon(pt(8, 1.2, 15.2, 14.4, .8, 14.4), c)
	}
}

// kv desenha linhas "rótulo: valor" em duas colunas de texto.
func (p *reportPDF) kv(rows [][2]string, keyW float64) {
	for _, r := range rows {
		lines := wrapText(r[1], pdf.Sans, 8, pdf.W-2*pm-keyW, 3)
		p.need(float64(len(lines))*10 + 6)
		p.d.Text(pm, p.y+10, pdf.SansBold, 8, cMuted, r[0])
		for i, ln := range lines {
			p.d.Text(pm+keyW, p.y+10+float64(i)*10, pdf.Sans, 8, cInk, ln)
		}
		p.y += float64(len(lines))*10 + 4
	}
}

// timeAxis escreve as marcas de tempo do eixo x: uma por dia (ou por hora
// em períodos curtos), sem encavalar.
func (p *reportPDF) timeAxis(x0, w, y float64, from, to time.Time) {
	span := to.Sub(from)
	if span <= 0 {
		return
	}
	xOf := func(t time.Time) float64 { return x0 + w*float64(t.Sub(from))/float64(span) }
	var step time.Duration
	format := "02/01"
	switch {
	case span <= 3*time.Hour:
		step, format = 30*time.Minute, "15:04"
	case span <= 26*time.Hour:
		step, format = 3*time.Hour, "15:04"
	case span <= 10*24*time.Hour:
		step = 24 * time.Hour
	default:
		step = 24 * time.Hour * time.Duration(math.Ceil(float64(span)/float64(10*24*time.Hour)))
	}
	t := from.Truncate(time.Hour)
	if step >= 24*time.Hour {
		y0, m0, d0 := from.Date()
		t = time.Date(y0, m0, d0, 0, 0, 0, 0, from.Location())
	}
	for ; !t.After(to); t = t.Add(step) {
		if t.Before(from) {
			continue
		}
		x := xOf(t)
		p.d.Line(x, y-3, x, y, 0.5, cMuted, false)
		lbl := t.Format(format)
		lw := pdf.TextWidth(lbl, pdf.Mono, 6.5)
		if x+lw/2 > x0+w {
			continue
		}
		p.d.Text(x-lw/2, y+8, pdf.Mono, 6.5, cMuted, lbl)
	}
}

// chart: todas as leituras por minuto. Com muitos minutos para a largura,
// cada coluna mostra a faixa do menor ao maior minuto dela (verde-claro) e a
// média (linha verde), para nenhum pico sumir. Trechos sem dado ficam
// cinza; episódios de alerta, com o fundo da cor do nível; tracejados no
// fundo da casa e no limite de atenção.
func (p *reportPDF) chart(r reportData) {
	const h = 190.0
	p.need(h + 40)
	x0, y0 := pm+30, p.y+6
	w := pdf.W - pm - x0
	from, to := r.From, r.To
	span := to.Sub(from)
	if span <= 0 {
		return
	}
	attn := 0.0
	for _, a := range r.Above {
		if a.Label == "Atenção" {
			attn = a.Limit
		}
	}
	// em µSv/h (2.36), cada minuto pela calibração que valia nele
	us := make([]float64, len(r.Pts))
	for i, pt := range r.Pts {
		us[i] = r.uPt(pt)
	}
	bgU := r.BgU
	top := math.Max(r.USt.Max*1.08, bgU*1.6)
	if attn > 0 && r.USt.Max >= attn*0.6 {
		top = math.Max(top, attn*1.1)
	}
	// um pico isolado muito alto achataria o resto: a escala para pouco
	// acima do P99,9 (ou do limite de atenção) e o pico aparece cortado,
	// com o valor escrito em cima
	clipped := false
	if len(r.Pts) > 0 {
		v := append([]float64(nil), us...)
		sort.Float64s(v)
		lim := math.Max(percentile(v, .999)*1.25, bgU*1.8)
		if attn > 0 {
			lim = math.Max(lim, attn*1.25)
		}
		if r.USt.Max > lim*1.15 {
			top, clipped = lim, true
		}
	}
	if top <= 0 {
		top = 0.1
	}
	stepY := niceStep(top / 5)
	top = math.Ceil(top/stepY-1e-9) * stepY
	stepDec := int(math.Max(0, -math.Floor(math.Log10(stepY)+1e-9)))
	yv := func(v float64) float64 { return y0 + h*(1-math.Min(v, top)/top) }
	xOf := func(t time.Time) float64 { return x0 + w*float64(t.Sub(from))/float64(span) }

	// fundo: episódios e lacunas
	for _, ep := range r.Episodes {
		c := &cAttBg
		switch ep.LevelCls {
		case "alarme":
			c = &cAlmBg
		case "perigo":
			c = &cDngBg
		}
		x1, x2 := xOf(ep.Start), xOf(ep.End.Add(time.Minute))
		if x2-x1 < 1.2 {
			x2 = x1 + 1.2
		}
		p.d.Rect(x1, y0, x2-x1, h, c, nil)
	}
	for _, g := range r.Gaps {
		x1, x2 := xOf(g.From), xOf(g.To)
		if x2-x1 >= 0.4 {
			p.d.Rect(x1, y0, x2-x1, h, &cGap, nil)
		}
	}
	// grade
	for v := 0.0; v <= top+1e-9; v += stepY {
		p.d.Line(x0, yv(v), x0+w, yv(v), 0.3, cLine, false)
		p.d.TextRight(x0-4, yv(v)+2.5, pdf.Mono, 6.5, cMuted, fmtBR(v, stepDec))
	}
	if bgU > 0 {
		p.d.Line(x0, yv(bgU), x0+w, yv(bgU), 0.7, cMuted, true)
	}
	if attn > 0 && attn <= top {
		p.d.Line(x0, yv(attn), x0+w, yv(attn), 0.7, cAtt, true)
	}

	// dados: colunas de ~0,8 pt
	cols := int(w / 0.8)
	perCol := span / time.Duration(cols)
	pts := r.Pts
	if perCol <= time.Minute {
		// cabe um ponto por minuto: linha direta, quebrando nas lacunas
		var seg [][2]float64
		var prev time.Time
		for i, pt := range pts {
			if len(seg) > 0 && pt.T.Sub(prev) >= 90*time.Second {
				p.d.Polyline(seg, 0.8, cAccent)
				seg = nil
			}
			seg = append(seg, [2]float64{xOf(pt.T), yv(us[i])})
			prev = pt.T
		}
		p.d.Polyline(seg, 0.8, cAccent)
	} else {
		type col struct {
			min, max, sum float64
			n             int
		}
		cs := make([]col, cols)
		for j, pt := range pts {
			i := int(float64(pt.T.Sub(from)) / float64(span) * float64(cols))
			if i < 0 || i >= cols {
				continue
			}
			c := &cs[i]
			u := us[j]
			if c.n == 0 || u < c.min {
				c.min = u
			}
			if c.n == 0 || u > c.max {
				c.max = u
			}
			c.sum += u
			c.n++
		}
		cw := w / float64(cols)
		for i, c := range cs {
			if c.n == 0 {
				continue
			}
			x := x0 + (float64(i)+0.5)*cw
			y1, y2 := yv(c.max), yv(c.min)
			if y2-y1 < 0.6 {
				y2 = y1 + 0.6
			}
			p.d.Line(x, y1, x, y2, cw+0.05, cBand, false)
		}
		var seg [][2]float64
		for i, c := range cs {
			if c.n == 0 {
				p.d.Polyline(seg, 0.7, cAccent)
				seg = nil
				continue
			}
			seg = append(seg, [2]float64{x0 + (float64(i)+0.5)*cw, yv(c.sum / float64(c.n))})
		}
		p.d.Polyline(seg, 0.7, cAccent)
	}
	if clipped {
		// marca os picos cortados (os 4 maiores, sem encavalar)
		type spike struct {
			t time.Time
			v float64
		}
		var sp []spike
		for i, pt := range pts {
			if us[i] > top {
				sp = append(sp, spike{pt.T, us[i]})
			}
		}
		sort.Slice(sp, func(i, j int) bool { return sp[i].v > sp[j].v })
		var used []float64
		shown := 0
		for _, k := range sp {
			if shown == 4 {
				break
			}
			x := xOf(k.t)
			near := false
			for _, ux := range used {
				if math.Abs(ux-x) < 30 {
					near = true
				}
			}
			if near {
				continue
			}
			used = append(used, x)
			shown++
			p.d.Line(x-2.5, y0+4, x, y0+0.5, 0.8, cWarn, false)
			p.d.Line(x, y0+0.5, x+2.5, y0+4, 0.8, cWarn, false)
			lbl := fmtU(k.v)
			p.d.Text(x+4, y0+8, pdf.Mono, 6.5, cWarn, lbl)
		}
	}
	p.d.Rect(x0, y0, w, h, nil, &cLine)
	p.timeAxis(x0, w, y0+h, from, to)
	p.d.Text(pm, y0-4+0, pdf.Sans, 6.5, cMuted, "µSv/h")
	p.y = y0 + h + 22

	// legenda
	lx := pm
	item := func(c pdf.RGB, txt string, dashed bool) {
		if lx+15+pdf.TextWidth(txt, pdf.Sans, 7) > pdf.W-pm {
			lx = pm
			p.y += 11
		}
		if dashed {
			p.d.Line(lx, p.y-2.5, lx+12, p.y-2.5, 0.8, c, true)
		} else {
			p.d.Rect(lx, p.y-6, 12, 6, &c, nil)
		}
		p.d.Text(lx+15, p.y, pdf.Sans, 7, cMuted, txt)
		lx += 15 + pdf.TextWidth(txt, pdf.Sans, 7) + 12
	}
	if perCol > time.Minute {
		item(cBand, fmt.Sprintf("faixa mín–máx (cada coluna ≈ %s)", fmtMinutes(int(math.Round(perCol.Minutes())))), false)
		item(cAccent, "média da coluna", false)
	} else {
		item(cAccent, "µSv/h de cada minuto", false)
	}
	item(cMuted, "fundo", true)
	if attn > 0 && attn <= top {
		item(cAtt, "atenção", true)
	}
	item(cAttBg, "episódio de alerta", false)
	if clipped {
		item(cWarn, "pico cortado (valor em cima)", false)
	}
	item(cGap, "sem dado", false)
	p.y += 6
}

// niceStep arredonda o passo da grade para 1, 2, 5 × 10^n.
func niceStep(x float64) float64 {
	if x <= 0 {
		return 1
	}
	e := math.Pow(10, math.Floor(math.Log10(x)))
	for _, m := range []float64{1, 2, 5, 10} {
		if x <= m*e {
			return m * e
		}
	}
	return 10 * e
}

// histChart: quantos minutos caíram em cada faixa de CPM (barras) e o que o
// acaso puro (Poisson com a mesma média) daria (linha roxa).
func (p *reportPDF) histChart(bins []histBin, x0, y0, w, h float64) {
	if len(bins) == 0 {
		return
	}
	top := 0.0
	for _, b := range bins {
		top = math.Max(top, math.Max(float64(b.Count), b.Expected))
	}
	if top <= 0 {
		return
	}
	bw := w / float64(len(bins))
	for i, b := range bins {
		bh := h * float64(b.Count) / top
		p.d.Rect(x0+float64(i)*bw+0.3, y0+h-bh, bw-0.6, bh, &cBand, nil)
	}
	var curve [][2]float64
	for i, b := range bins {
		curve = append(curve, [2]float64{x0 + (float64(i)+0.5)*bw, y0 + h - h*b.Expected/top})
	}
	p.d.Polyline(curve, 1.1, cExp)
	p.d.Line(x0, y0+h, x0+w, y0+h, 0.5, cMuted, false)
	step := int(math.Max(1, math.Ceil(float64(len(bins))/8)))
	for i := 0; i < len(bins); i += step {
		lbl := fmtBR(bins[i].Lo, 0)
		p.d.Text(x0+float64(i)*bw, y0+h+8, pdf.Mono, 6.5, cMuted, lbl)
	}
}

func renderReportPDF(r reportData) []byte {
	p := &reportPDF{d: pdf.New(), y: 50, page: 1, r: r}
	d := p.d
	// cabeçalho
	d.Text(pm, p.y+4, pdf.SansBold, 18, cInk, r.Title)
	// período com nome comprido (ex.: "Fora de casa: …"): quebra, não corta
	pl := wrapText(r.Period, pdf.Sans, 9, pdf.W-2*pm-150, 3)
	for i, ln := range pl {
		d.Text(pm, p.y+20+float64(i)*11, pdf.Sans, 9, cMuted, ln)
	}
	d.TextRight(pdf.W-pm, p.y, pdf.Sans, 8, cMuted, "RadLog "+r.Version)
	ry := p.y + 11
	if r.Device != "" {
		d.TextRight(pdf.W-pm, ry, pdf.Sans, 8, cMuted, r.Device)
		ry += 11
	}
	d.TextRight(pdf.W-pm, ry, pdf.Sans, 8, cMuted, "gerado em "+r.Generated)
	p.y += 32 + float64(len(pl)-1)*11
	d.Line(pm, p.y, pdf.W-pm, p.y, 1.5, cInk, false)
	p.y += 12

	if !r.HasData || r.St.N == 0 {
		d.Text(pm, p.y+12, pdf.Sans, 10, cMuted, "Sem leituras guardadas neste período.")
		p.footer()
		return d.Bytes(pdf.Tr(r.Title), "RadLog "+AppVersion)
	}
	st := r.St

	// cartões 4 x 2
	tg := 8.0
	tw := (pdf.W - 2*pm - 3*tg) / 4
	th := 60.0
	bg, bgSub := "—", "ainda sem fundo"
	if r.Bg != "" {
		bg, bgSub = r.BgUSv+" µSv/h", "mediana de 30 dias ("+r.Bg+" CPM) · média do período "+r.VsBg
	}
	rain := r.Rain
	if rain == "" {
		rain = "—"
	}
	cover := fmt.Sprintf("%d dia(s) com dado", r.DaysData)
	covSub := fmt.Sprintf("%d de %d min · ", st.N, r.HistMin)
	if r.HistMin < r.SpanMin {
		covSub = fmt.Sprintf("%d de %d min desde %s · ", st.N, r.HistMin, r.HistFrom.Format("02/01 15:04"))
	}
	if r.Away != "" {
		cover += " · " + r.Away + " fora de casa"
	}
	ust := r.USt
	maxU, maxAt := ust.Max, ust.MaxAt
	maxSub := ust.MinAt.Format("02/01 15:04") + " · " + ust.MaxAt.Format("02/01 15:04")
	if r.AlertPeakMaxU > maxU {
		maxU, maxAt = r.AlertPeakMaxU, r.alertPeakAt
		maxSub = ust.MinAt.Format("02/01 15:04") + " · " + maxAt.Format("02/01 15:04") + " · máx. = pico ao vivo (maior minuto: " + fmtU(ust.Max) + ")"
	}
	alertSub := "contados por episódio"
	if r.AlertPeakMaxU > 0 {
		alertSub += " · pico ao vivo " + fmtU(r.AlertPeakMaxU) + " µSv/h"
	}
	tiles := [][5]string{
		{"Média (por minuto)", fmtU(ust.Mean) + " µSv/h", fmt.Sprintf("contagem %s CPM (M) · n = %d min", r.Avg, st.N), "m", "C"},
		{"Dose no período", r.Dose + " µSv", "calculada pelo app · ≈ " + r.DoseYear + " mSv/ano (E)", "m", "C"},
		{"Fundo da casa", bg, bgSub, "m", "C"},
		{"Mínimo · máximo", fmtU(ust.Min) + " · " + fmtU(maxU) + " µSv/h", maxSub, "m", "C"},
		{"Alertas", r.Alerts, alertSub, "s", "C"},
		{"Chuva", rain, "radar/modelo meteorológico", "s", "E"},
		{"Cobertura", fmtBR(ratio(st.N, r.HistMin)*100, 0) + "%", covSub + cover, "s", "C"},
		{"Medições de objeto", fmt.Sprint(len(r.Meas)), "no período", "m", "C"},
	}
	for i, t := range tiles {
		x := pm + float64(i%4)*(tw+tg)
		y := p.y + float64(i/4)*(th+tg)
		f := pdf.MonoBold
		if t[3] == "s" {
			f = pdf.SansBold
		}
		p.tile(x, y, tw, th, t[0], t[1], t[2], f)
		d.TextRight(x+tw-8, y+13, pdf.SansBold, 6.5, cMuted, t[4])
	}
	p.y += 2*th + tg
	for i, ln := range wrapText(originLegend, pdf.Sans, 6.8, pdf.W-2*pm, 2) {
		d.Text(pm, p.y+10+float64(i)*8.5, pdf.Sans, 6.8, cMuted, ln)
	}
	p.y += 22

	if rc := r.S.RC; rc != nil && rc.Minutes > 0 {
		p.section("RADIACODE 102 (DOSÍMETRO PESSOAL)")
		u := func(v float64) string { return fmtBR(v, usvDec(v)) + " µSv/h" }
		at := func(ms int64) string { return time.UnixMilli(ms).Format("02/01 15:04") }
		vs := "fundo em casa ainda não calculado"
		if rc.BgUSv > 0 {
			vs = fmt.Sprintf("%s%% em relação ao fundo em casa (%s, mediana de 7 dias)", fmtSigned(rc.VsBgPct, 0), u(rc.BgUSv))
		}
		away := "nenhum minuto fora de casa"
		if rc.AwayMinutes > 0 {
			away = fmt.Sprintf("%s µSv em %s · média %s · máximo %s (%s)", fmtBR(rc.AwayDose, 2), fmtMinutes(rc.AwayMinutes), u(rc.AwayAvg), u(rc.AwayMax), at(rc.AwayMaxAt))
		}
		dev := "não disponível: o registro dos totais do aparelho não cobre o início do período"
		if rc.DevOK {
			dev = fmt.Sprintf("%s µSv (diferença entre os totais informados pelo aparelho; inclui o tempo fora do alcance do PC)", fmtBR(rc.DevDose, 2))
		}
		p.kv([][2]string{
			{"Dose acumulada pelo aparelho (R)", dev},
			{"Dose calculada pelo app (C)", fmt.Sprintf("%s µSv (soma dos minutos registrados; média %s)", fmtBR(rc.Dose, 2), u(rc.Avg))},
			{"Média (R)", vs},
			{"Mínimo · máximo (R)", fmt.Sprintf("%s (%s) · %s (%s)", u(rc.Min), at(rc.MinAt), u(rc.Max), at(rc.MaxAt))},
			{"Em casa", fmt.Sprintf("%s µSv em %s", fmtBR(rc.HomeDose, 2), fmtMinutes(rc.HomeMinutes))},
			{"Fora de casa", away},
			{"Cobertura", rcCoverageTxt(rc)},
			{"Alertas", rcAlertsTxt(rc)},
		}, 150)
	}

	p.section("µSv/h AO LONGO DO PERÍODO (TODAS AS LEITURAS POR MINUTO)")
	p.chart(r)

	// estatística + histograma lado a lado
	p.section("ESTATÍSTICA DA CONTAGEM POR MINUTO (CPM, O QUE O CONTADOR MEDE)")
	p.need(165)
	top := p.y
	livePeak := "— (nenhum alerta ao vivo)"
	if r.AlertPeakMax > 0 {
		livePeak = fmtBR(r.AlertPeakMax, 0) + " CPM às " + r.alertPeakAt.Format("02/01 15:04")
	}
	rows := [][3]string{
		{"Minutos com leitura (n)", fmt.Sprintf("%d", st.N), "M"},
		{"Média · mediana", fmtBR(st.Mean, 2) + " · " + fmtBR(st.Median, 1) + " CPM", "M"},
		{"Desvio-padrão real", fmtBR(st.SD, 2) + " CPM", "M"},
		{"σ esperado só pelo acaso (√média)", fmtBR(st.PoissonSD, 2) + " CPM", "C"},
		{"Dispersão (real ÷ acaso)", fmtBR(st.Dispersion, 2), "C"},
		{"P5 · P25 · P50", fmtBR(st.P5, 1) + " · " + fmtBR(st.P25, 1) + " · " + fmtBR(st.Median, 1), "M"},
		{"P75 · P95", fmtBR(st.P75, 1) + " · " + fmtBR(st.P95, 1), "M"},
		{"Mínimo · máximo (minutos gravados)", fmtBR(st.Min, 0) + " · " + fmtBR(st.Max, 0) + " CPM", "M"},
		{"Pico ao vivo (alertas, janela de 60 s)", livePeak, "M"},
	}
	colW := (pdf.W - 2*pm) * 0.46
	for _, kv := range rows {
		d.Text(pm, p.y+10, pdf.Sans, 8, cMuted, kv[0])
		d.Text(pm+colW+5, p.y+10, pdf.SansBold, 6.5, cMuted, kv[2])
		d.TextRight(pm+colW, p.y+10, pdf.Mono, 8, cInk, kv[1])
		p.y += 13
		d.Line(pm, p.y, pm+colW, p.y, 0.3, cLine, false)
	}
	disp := "Dispersão próxima de 1: a variação observada é compatível com a flutuação estatística da contagem (Poisson)."
	if st.Dispersion >= 1.2 {
		disp = "Dispersão acima de ~1,2: há variação além da flutuação estatística (precipitação, radônio, objetos próximos ou mudanças no ambiente)."
	}
	for i, ln := range wrapText(disp, pdf.Sans, 7, colW, 3) {
		d.Text(pm, p.y+11+float64(i)*9, pdf.Sans, 7, cMuted, ln)
	}
	hx := pm + colW + 24
	hw := pdf.W - pm - hx
	d.Text(hx, top+10, pdf.SansBold, 7.5, cMuted, "Distribuição: minutos por faixa de CPM (eixo x em CPM)")
	p.histChart(r.Bins, hx, top+18, hw, 92)
	ly := top + 128
	d.Rect(hx, ly-6, 10, 6, &cBand, nil)
	d.Text(hx+13, ly, pdf.Sans, 7, cMuted, "medido")
	d.Line(hx+52, ly-3, hx+64, ly-3, 1.1, cExp, false)
	d.Text(hx+67, ly, pdf.Sans, 7, cMuted, "só acaso (Poisson, mesma média)")
	p.y = math.Max(p.y+30, ly+8)

	p.section("POR DIA")
	var trows [][]string
	starred := false
	for _, dd := range r.Days {
		mx := fmtU(dd.MaxU)
		if dd.LivePeakU > dd.MaxU {
			mx = fmtU(dd.LivePeakU) + "*"
			starred = true
		}
		trows = append(trows, []string{dd.Label, fmtU(dd.USv), fmtU(dd.MinU), mx, fmtBR(dd.Dose, 2), fmtBR(dd.Avg, 1), fmt.Sprintf("%d / 1440", dd.Minutes)})
	}
	cw := pdf.W - 2*pm
	p.table([]string{"Dia", "Média µSv/h (C)", "Mín µSv/h", "Máx µSv/h", "Dose µSv (C)", "Média CPM (M)", "Minutos"}, trows,
		[]float64{cw * .18, cw * .15, cw * .12, cw * .12, cw * .14, cw * .13, cw * .16},
		[]bool{false, true, true, true, true, true, true}, []bool{false, true, true, true, true, true, true}, nil)
	if starred {
		p.need(14)
		d.Text(pm, p.y+10, pdf.Sans, 6.8, cMuted, "* pico ao vivo de um alerta (janela de 60 s), maior que o maior minuto gravado do dia. Média, mínimo e dose usam os minutos gravados.")
		p.y += 14
	}

	p.section("FUNDO DA CASA: AGORA × 7 DIAS × 30 DIAS")
	trows = nil
	for _, b := range r.BgRows {
		if !b.OK {
			trows = append(trows, []string{b.Label, "—", "—", fmt.Sprint(b.N), "dados insuficientes"})
			continue
		}
		vs := "—"
		if st.N > 0 && b.CPM > 0 {
			pc := (st.Mean - b.CPM) / b.CPM * 100
			sign := "+"
			if pc < 0 {
				sign = ""
			}
			vs = sign + fmtBR(pc, 1) + "%"
		}
		trows = append(trows, []string{b.Label, fmtU(b.USv), fmtBR(b.CPM, 1), fmt.Sprint(b.N), vs})
	}
	p.table([]string{"Janela", "µSv/h (C)", "Mediana CPM (C)", "Minutos em casa", "Média do período vs."}, trows,
		[]float64{cw * .32, cw * .15, cw * .13, cw * .18, cw * .22},
		[]bool{false, true, true, true, true}, []bool{false, true, true, true, true}, nil)

	p.section("ALERTAS (EPISÓDIOS)")
	if len(r.Episodes) == 0 {
		p.need(20)
		d.Text(pm, p.y+12, pdf.Sans, 8.5, cMuted, "Nenhum alerta no período.")
		p.y += 18
	} else {
		trows = nil
		limNow := false
		eps := r.Episodes
		for i := len(eps) - 1; i >= 0; i-- {
			e := eps[i]
			span := e.Start.Format("02/01 15:04") + "–" + e.End.Format("15:04")
			mm := "—"
			if e.N > 0 {
				mm = fmtU(e.MinU) + " / " + fmtU(e.MeanU) + " / " + fmtU(e.MaxU)
			}
			exc := "—"
			if e.LimitU > 0 {
				exc = fmt.Sprintf("+%s (×%s)", fmtU(math.Max(0, e.PeakU-e.LimitU)), fmtBR(e.PeakU/e.LimitU, 2))
			}
			dur := fmtSeconds(e.DurSec)
			if e.Retro {
				dur = "≈ " + fmtMinutes(e.Minutes) // histórico: só minuto a minuto
			}
			lvl := e.Level
			if e.Retro {
				lvl += " (c)"
			}
			if e.Breaks > 0 || e.EndReason != "" {
				lvl += " (d)"
			}
			lim := fmtU(e.LimitU)
			if e.LimitNow {
				lim += "*"
				limNow = true
			}
			trows = append(trows, []string{span, lvl, dur, mm, fmtU(e.PeakU), lim, exc})
		}
		rev := make([]reportEpisode, len(eps))
		for i := range eps {
			rev[i] = eps[len(eps)-1-i]
		}
		p.table([]string{"Quando", "Nível", "Duração", "Mín/méd/máx µSv/h", "Pico µSv/h", "Limite µSv/h", "Acima do limite"}, trows,
			[]float64{cw * .2, cw * .13, cw * .1, cw * .22, cw * .1, cw * .1, cw * .15},
			[]bool{false, false, true, true, true, true, true}, []bool{true, false, true, true, true, true, true},
			func(i int) *pdf.RGB {
				switch rev[i].LevelCls {
				case "perigo":
					return &cWarn
				case "alarme":
					return &cAlarm
				}
				return &cAtt
			})
		note := "Pico: maior leitura ao vivo do alerta (janela de 60 s, consultada a cada segundo). Mín/méd/máx: minutos gravados no episódio. (c): episódio encontrado no histórico do contador. (d): episódio com interrupção de leitura (desconexão ou app fechado)."
		if limNow {
			note += " *: limite pelo valor atual, sem registro do valor vigente na época."
		}
		p.paragraph(note, 6.8, cMuted)
		p.paragraph(episodeRule, 6.8, cMuted)
		p.episodeCharts()
	}

	p.section("TEMPO ACIMA DOS LIMITES OPERACIONAIS")
	trows = nil
	aboveNow := false
	for _, a := range r.Above {
		lim := "≥ " + fmtU(a.Limit) + " µSv/h"
		if len(a.Limits) > 1 {
			var ls []string
			for _, l := range a.Limits {
				ls = append(ls, fmtU(l))
			}
			lim = "≥ " + strings.Join(ls, " → ") + " µSv/h"
		} else if len(a.Limits) == 1 {
			lim = "≥ " + fmtU(a.Limits[0]) + " µSv/h"
		}
		if a.UsedNow {
			lim += "*"
			aboveNow = true
		}
		trows = append(trows, []string{a.Label, lim, fmtMinutes(a.Minutes), fmtBR(float64(a.Minutes)/float64(st.N)*100, 2) + "%"})
	}
	p.table([]string{"Limite", "A partir de", "Tempo (minutos gravados)", "Do tempo medido"}, trows,
		[]float64{cw * .2, cw * .2, cw * .35, cw * .25}, []bool{false, true, true, true}, []bool{false, true, true, true}, nil)
	if aboveNow {
		p.paragraph("Cada minuto foi comparado com o limite vigente naquele momento (histórico de configurações). *: há minutos sem registro do limite da época, comparados com o valor atual.", 6.8, cMuted)
	} else {
		p.paragraph("Cada minuto foi comparado com o limite vigente naquele momento (histórico de configurações).", 6.8, cMuted)
	}

	p.section("LACUNAS (SEM LEITURA)")
	missing := r.HistMin - st.N
	if missing < 0 {
		missing = 0
	}
	p.need(16)
	d.Text(pm, p.y+11, pdf.Sans, 8, cInk, fmt.Sprintf("%s sem leitura no período, em %d trecho(s). Lacunas não são tratadas como zero em nenhum cálculo.", fmtMinutes(missing), len(r.Gaps)))
	p.y += 16
	if pre := r.SpanMin - r.HistMin; pre > 0 {
		p.need(14)
		d.Text(pm, p.y+9, pdf.Sans, 8, cMuted, fmt.Sprintf("O período começa %s antes do primeiro dado do contador (%s); esse trecho não é lacuna.", fmtMinutes(pre), r.HistFrom.Format("02/01/2006 15:04")))
		p.y += 14
	}
	var big []gap
	for _, g := range r.Gaps {
		if g.Minutes() >= 10 {
			big = append(big, g)
		}
	}
	if len(big) > 0 {
		trows = nil
		for i, g := range big {
			if i == 20 {
				trows = append(trows, []string{fmt.Sprintf("e mais %d", len(big)-20), "", ""})
				break
			}
			trows = append(trows, []string{g.From.Format("02/01 15:04"), g.To.Format("02/01 15:04"), fmtMinutes(g.Minutes())})
		}
		p.table([]string{"De", "Até", "Duração (trechos de 10 min ou mais)"}, trows,
			[]float64{cw * .3, cw * .3, cw * .4}, []bool{false, false, true}, []bool{true, true, true}, nil)
	}

	p.maturity()

	if len(r.Meas) > 0 {
		p.section("MEDIÇÕES DE OBJETO")
		for _, m := range r.Meas {
			p.measBlock(m)
		}
	}

	p.cfgPeriod()

	p.section("CONFIGURAÇÕES E MÉTODO")
	p.kv(r.Config, 150)

	p.section("LIMITAÇÕES")
	for _, l := range []string{
		"A dose refere-se ao ponto onde o contador está instalado (monitor de área), não à dose de pessoas próximas. Não constitui dosimetria pessoal.",
		"A conversão CPM → µSv/h depende da calibração do tubo e da energia da radiação; para fontes com espectro diferente do de calibração, o erro pode ser grande.",
		"Os limites do app são operacionais, definidos pelo usuário; não são limites regulamentares nem limiares de segurança.",
		"A contagem (CPM) não identifica o radionuclídeo nem o tipo de radiação.",
		"Projeções (mSv/ano) são extrapolações do período, não medições.",
		"Médias de janelas \"em formação\" (histórico menor que a janela) não representam a janela inteira.",
	} {
		lines := wrapText("• "+l, pdf.Sans, 7.5, pdf.W-2*pm, 3)
		p.need(float64(len(lines))*9.5 + 3)
		for i, ln := range lines {
			d.Text(pm, p.y+10+float64(i)*9.5, pdf.Sans, 7.5, cMuted, ln)
		}
		p.y += float64(len(lines))*9.5 + 3
	}
	p.footer()
	return d.Bytes(pdf.Tr(r.Title+" · "+r.Period), "RadLog "+AppVersion)
}

// episodeCharts desenha, para os episódios escolhidos, o µSv/h de 30 min antes
// até 30 min depois: minutos gravados (verde) e, se houver, a leitura ao vivo
// por segundo (cinza), com o limite e o fundo tracejados.
func (p *reportPDF) episodeCharts() {
	var eps []reportEpisode
	for _, e := range p.r.Episodes {
		if e.Chart {
			eps = append(eps, e)
		}
	}
	if len(eps) == 0 {
		return
	}
	d := p.d
	p.section("EPISÓDIOS: 30 MIN ANTES E DEPOIS")
	if len(eps) < len(p.r.Episodes) {
		p.need(14)
		d.Text(pm, p.y+10, pdf.Sans, 7, cMuted, fmt.Sprintf("Os %d episódios mais graves e mais altos de %d.", len(eps), len(p.r.Episodes)))
		p.y += 14
	}
	cSec := pdf.Hex("#8C99A3")
	const ncol, gapX, ch = 3, 14.0, 66.0
	cw := (pdf.W - 2*pm - gapX*(ncol-1)) / ncol
	for i, e := range eps {
		if i%ncol == 0 {
			if i > 0 {
				p.y += ch + 34
			}
			p.need(ch + 36)
		}
		x0 := pm + float64(i%ncol)*(cw+gapX)
		y0 := p.y + 14
		lc := cAtt
		bgc := cAttBg
		switch e.LevelCls {
		case "alarme":
			lc, bgc = cAlarm, cAlmBg
		case "perigo":
			lc, bgc = cWarn, cDngBg
		}
		title := e.Start.Format("02/01 15:04") + " · " + e.Level + " · pico " + fmtU(e.PeakU) + " µSv/h"
		d.Text(x0, p.y+9, pdf.SansBold, 7, cInk, pdf.Fit(title, pdf.SansBold, 7, cw))

		w0 := e.Start.Add(-30 * time.Minute).Truncate(time.Minute)
		w1 := e.End.Add(31 * time.Minute).Truncate(time.Minute)
		span := w1.Sub(w0)
		// em µSv/h (2.36): minutos pela calibração de cada um, segundos pela da hora
		au := make([]float64, len(e.Around))
		for k, pt := range e.Around {
			au[k] = p.r.uPt(pt)
		}
		su := make([]float64, len(e.Secs))
		for k, sp := range e.Secs {
			su[k] = p.r.uAt(time.UnixMilli(sp.T), sp.CPM)
		}
		top := math.Max(e.PeakU, e.LimitU)
		for _, v := range au {
			top = math.Max(top, v)
		}
		for _, v := range su {
			top = math.Max(top, v)
		}
		top = math.Max(0.01, top*1.12)
		xOf := func(t time.Time) float64 { return x0 + cw*float64(t.Sub(w0))/float64(span) }
		yv := func(v float64) float64 { return y0 + ch*(1-math.Min(math.Max(v, 0), top)/top) }

		ex1, ex2 := xOf(e.Start), xOf(e.End.Add(time.Minute))
		if ex2-ex1 < 1 {
			ex2 = ex1 + 1
		}
		d.Rect(ex1, y0, ex2-ex1, ch, &bgc, nil)
		if bgU := p.r.BgU; bgU > 0 && bgU < top {
			d.Line(x0, yv(bgU), x0+cw, yv(bgU), 0.5, cMuted, true)
		}
		if e.LimitU > 0 && e.LimitU < top {
			d.Line(x0, yv(e.LimitU), x0+cw, yv(e.LimitU), 0.6, lc, true)
		}
		// por segundo
		var seg [][2]float64
		var prev int64
		for k, sp := range e.Secs {
			t := time.UnixMilli(sp.T)
			if t.Before(w0) || t.After(w1) {
				continue
			}
			if len(seg) > 0 && sp.T-prev > 30000 {
				d.Polyline(seg, 0.4, cSec)
				seg = nil
			}
			seg = append(seg, [2]float64{xOf(t), yv(su[k])})
			prev = sp.T
		}
		d.Polyline(seg, 0.4, cSec)
		// por minuto
		seg = nil
		var pt0 time.Time
		for k, pt := range e.Around {
			if pt.T.Before(w0) || !pt.T.Before(w1) {
				continue
			}
			if len(seg) > 0 && pt.T.Sub(pt0) >= 90*time.Second {
				d.Polyline(seg, 0.9, cAccent)
				seg = nil
			}
			seg = append(seg, [2]float64{xOf(pt.T.Add(30 * time.Second)), yv(au[k])})
			pt0 = pt.T
		}
		d.Polyline(seg, 0.9, cAccent)
		// pico
		if !e.PeakAt.IsZero() && !e.PeakAt.Before(w0) && e.PeakAt.Before(w1) {
			px, py := xOf(e.PeakAt), yv(e.PeakU)
			d.Rect(px-1.3, py-1.3, 2.6, 2.6, &lc, nil)
		}
		d.Rect(x0, y0, cw, ch, nil, &cLine)
		d.Text(x0+2, y0+7, pdf.Mono, 5.8, cMuted, fmtU(top)+" µSv/h")
		d.Text(x0, y0+ch+8, pdf.Mono, 6, cMuted, w0.Format("15:04"))
		d.TextRight(x0+cw, y0+ch+8, pdf.Mono, 6, cMuted, w1.Format("15:04"))
		dur := fmtSeconds(e.DurSec)
		if e.Retro {
			dur = "≈ " + fmtMinutes(e.Minutes)
		}
		mid := dur
		if e.N > 0 {
			mid += " · méd. " + fmtU(e.MeanU)
		}
		mw := pdf.TextWidth(mid, pdf.Sans, 6.2)
		d.Text(x0+cw/2-mw/2, y0+ch+8, pdf.Sans, 6.2, cMuted, mid)
	}
	p.y += ch + 34
	p.need(14)
	lx := pm
	leg := func(c pdf.RGB, txt string, dashed bool, thin bool) {
		switch {
		case dashed:
			d.Line(lx, p.y-2.5, lx+12, p.y-2.5, 0.7, c, true)
		case thin:
			d.Line(lx, p.y-2.5, lx+12, p.y-2.5, 0.5, c, false)
		default:
			d.Line(lx, p.y-2.5, lx+12, p.y-2.5, 1.1, c, false)
		}
		d.Text(lx+15, p.y, pdf.Sans, 7, cMuted, txt)
		lx += 15 + pdf.TextWidth(txt, pdf.Sans, 7) + 12
	}
	leg(cAccent, "minuto gravado", false, false)
	leg(cSec, "ao vivo, por segundo (janela de 60 s)", false, true)
	leg(cAlarm, "limite do nível", true, false)
	leg(cMuted, "fundo", true, false)
	d.Rect(lx, p.y-5, 4, 4, &cAlarm, nil)
	d.Text(lx+7, p.y, pdf.Sans, 7, cMuted, "pico")
	p.y += 8
}

// measBlock escreve uma medição de objeto com tudo o que ela usou.
func (p *reportPDF) measBlock(m reportMeas) {
	d := p.d
	x := m.M
	phase := func(ph MeasurePhase) string {
		return fmt.Sprintf("%d contagens em %d s = %s ± %s CPM", ph.Counts, ph.Seconds, fmtBR(ph.CPM, 1), fmtBR(ph.ErrCPM, 1))
	}
	net := m.Result
	if x.Result.Ratio > 0 {
		net += " · objeto ÷ fundo = " + fmtBR(x.Result.Ratio, 2) + "×"
	}
	if x.Result.Sigma != 0 {
		net += " · " + fmtBR(x.Result.Sigma, 1) + "σ"
	}
	note := strings.TrimSpace(x.Note)
	if note == "" {
		note = "não anotada (dá para anotar na aba Medir: distância, lado encostado no tubo, com ou sem embalagem)"
	}
	lines := [][2]string{
		{"Fundo usado", phase(x.Bg) + " (medido logo antes, com o objeto longe)"},
		{"Com o objeto", phase(x.Obj)},
		{"Líquido", net},
		{"Geometria / anotação", note},
	}
	p.need(14 + float64(len(lines))*10 + 8)
	d.Text(pm, p.y+11, pdf.SansBold, 8.5, cInk, pdf.Fit(m.When+" · "+m.Name, pdf.SansBold, 8.5, (pdf.W-2*pm)*0.65))
	d.TextRight(pdf.W-pm, p.y+11, pdf.SansBold, 8, cMuted, pdf.Fit(m.Verdict, pdf.SansBold, 8, (pdf.W-2*pm)*0.33))
	p.y += 15
	for _, l := range lines {
		ls := wrapText(l[1], pdf.Sans, 7.8, pdf.W-2*pm-95, 2)
		d.Text(pm+8, p.y+9, pdf.Sans, 7.8, cMuted, l[0])
		for i, ln := range ls {
			d.Text(pm+95, p.y+9+float64(i)*9.5, pdf.Sans, 7.8, cInk, ln)
		}
		p.y += float64(len(ls))*9.5 + 1.5
	}
	p.y += 4
	d.Line(pm, p.y, pdf.W-pm, p.y, 0.3, cLine, false)
}

// usvDec: casas decimais para µSv/h (3 abaixo de 1, 2 até 10, 1 acima).
func usvDec(v float64) int {
	switch {
	case v < 1:
		return 3
	case v < 10:
		return 2
	}
	return 1
}

// fmtSigned: número com sinal (+12, −3), vírgula decimal.
func fmtSigned(v float64, dec int) string {
	if v >= 0 {
		return "+" + fmtBR(v, dec)
	}
	return "−" + fmtBR(-v, dec)
}

// rcAlertsTxt resume os alertas do Radiacode no período.
func rcAlertsTxt(rc *RcSum) string {
	var parts []string
	if rc.Danger > 0 {
		parts = append(parts, fmt.Sprintf("%d perigo", rc.Danger))
	}
	if rc.Alarm > 0 {
		parts = append(parts, plural(rc.Alarm, "1 alarme", fmt.Sprintf("%d alarmes", rc.Alarm)))
	}
	if rc.Attention > 0 {
		parts = append(parts, plural(rc.Attention, "1 atenção", fmt.Sprintf("%d atenções", rc.Attention)))
	}
	if len(parts) == 0 {
		return "nenhum"
	}
	return strings.Join(parts, ", ") + " · maior pico " + usvTxt(rc.PeakAlert) + " µSv/h"
}

func rcCoverageTxt(rc *RcSum) string {
	s := fmt.Sprintf("%s%% · %d de %d min com leitura do RC102", fmtBR(rc.Coverage*100, 0), rc.Minutes, rc.HistMinutes)
	if rc.HistMinutes < rc.SpanMinutes {
		s += fmt.Sprintf(" desde o primeiro dado (%s) · %s%% do período inteiro", time.UnixMilli(rc.HistFrom).Format("02/01/2006 15:04"), fmtBR(rc.CoverageAll*100, 0))
	}
	return s
}

// originLegend explica as marcas de origem dos números (2.6).
const originLegend = "Origem dos valores: M = medido pelo contador (contagem por minuto e estatística direta dessas contagens); C = calculado pelo app a partir das contagens (conversão para µSv/h, dose, fundo, σ esperado, alertas, cobertura); R = informado pelo RC102; E = estimativa externa ou extrapolação."

// episodeRule é a regra de agrupamento dos alertas em episódios.
const episodeRule = "Regra dos episódios: um alerta abre quando a leitura ao vivo atinge o limite do nível e fecha quando ela cai abaixo de 90% dele (histerese). Alertas que se encostam (o seguinte começa até 1 min após o fim do anterior) formam um episódio, com o nível mais alto, o início do primeiro e o fim do último. Se o contador desconecta com um alerta aberto e reconecta em até 15 min com a leitura ainda acima do limite, o episódio continua e o trecho sem leitura não entra na estatística; fechar o app encerra o episódio. Episódios encontrados no histórico do contador (c) usam as leituras por minuto e terminam após 70 min sem dado."

// paragraph escreve um texto corrido com quebra de linha.
func (p *reportPDF) paragraph(txt string, size float64, c pdf.RGB) {
	lines := wrapText(txt, pdf.Sans, size, pdf.W-2*pm, 8)
	lh := size + 2.2
	p.need(float64(len(lines))*lh + 6)
	for i, ln := range lines {
		p.d.Text(pm, p.y+size+3+float64(i)*lh, pdf.Sans, size, c, ln)
	}
	p.y += float64(len(lines))*lh + 6
}

// cfgPeriod é a tabela das configurações vigentes no período (2.6).
func (p *reportPDF) cfgPeriod() {
	r := p.r
	p.section("CONFIGURAÇÕES VIGENTES NO PERÍODO")
	if len(r.CfgRows) == 0 {
		msg := "Não há registro de configurações para este período (o histórico de configurações começa na versão 2.1)."
		if !r.CfgFirst.IsZero() {
			msg = "Não há registro de configurações para este período: o histórico de configurações começa em " + r.CfgFirst.Format("02/01/2006 15:04") + "."
		}
		p.paragraph(msg, 8, cMuted)
		return
	}
	cw := pdf.W - 2*pm
	var rows [][]string
	for _, c := range r.CfgRows {
		start := c.AtStart
		if !c.Known {
			start = "sem registro (histórico desde " + r.CfgFirst.Format("02/01/2006") + ")"
		}
		ch := "nenhuma"
		if len(c.Changes) > 0 {
			var parts []string
			for _, x := range c.Changes {
				parts = append(parts, cfgChangeTxt(x))
			}
			ch = strings.Join(parts, "; ")
		}
		rows = append(rows, []string{c.Field, start, ch})
	}
	p.wrapTable([]string{"Ajuste", "No início do período", "Mudanças no período"}, rows, []float64{cw * .2, cw * .35, cw * .45})
	p.paragraph("Valores tirados do histórico de configurações do app. Cada minuto e cada alerta foram avaliados com o valor vigente no momento; a calibração segue as versões descritas em \"Calibração no período\".", 6.8, cMuted)
}

// maturity é a tabela do histórico disponível em cada janela, terminando no
// fim do período (2.6).
func (p *reportPDF) maturity() {
	m := p.r.Mature
	if len(m.GMC) == 0 {
		return
	}
	p.section("HISTÓRICO DISPONÍVEL (MATURIDADE)")
	cw := pdf.W - 2*pm
	win := func(d int) string {
		switch d {
		case 1:
			return "24 h"
		case 365:
			return "1 ano"
		}
		return fmt.Sprintf("%d dias", d)
	}
	cell := func(w MatWin) string {
		switch w.State {
		case matWaiting:
			return "aguardando dados"
		case matForming:
			return fmt.Sprintf("em formação (%s de %d dias) · %s%% medido", fmtBR(w.HistDays, 1), w.Days, fmtBR(w.Coverage*100, 0))
		}
		return fmt.Sprintf("disponível · %s%% medido (%d de %d min)", fmtBR(w.Coverage*100, 0), w.Minutes, w.HistMinutes)
	}
	head := []string{"Janela (até o fim do período)", "GMC-800"}
	cols := []float64{cw * .32, cw * .68}
	if len(m.RC) > 0 {
		head = append(head, "RC102")
		cols = []float64{cw * .3, cw * .35, cw * .35}
	}
	var rows [][]string
	for i, w := range m.GMC {
		row := []string{win(w.Days), cell(w)}
		if len(m.RC) > i {
			row = append(row, cell(m.RC[i]))
		}
		rows = append(rows, row)
	}
	p.wrapTable(head, rows, cols)
	first := "Primeiro dado do GMC-800: " + time.UnixMilli(m.GMCFirst).Format("02/01/2006 15:04")
	if m.GMCFirst == 0 {
		first = "Sem dado do GMC-800"
	}
	if m.RCFirst != 0 {
		first += " · do RC102: " + time.UnixMilli(m.RCFirst).Format("02/01/2006 15:04")
	}
	p.paragraph(first+".", 6.8, cMuted)
	p.y -= 4
	p.paragraph("Disponível: o histórico cobre a janela inteira. Em formação: o primeiro dado está dentro da janela, e estatísticas dessa janela ainda não a representam por completo.", 6.8, cMuted)
}

// wrapTable é uma tabela de texto com quebra de linha nas células.
func (p *reportPDF) wrapTable(head []string, rows [][]string, cols []float64) {
	const fs, lh = 7.5, 9.5
	drawHead := func() {
		x := pm
		for i, h := range head {
			p.d.Text(x+4, p.y+10, pdf.SansBold, 7.5, cMuted, h)
			x += cols[i]
		}
		p.y += 15
		p.d.Line(pm, p.y, pdf.W-pm, p.y, 0.6, cLine, false)
	}
	p.need(40)
	drawHead()
	for _, row := range rows {
		cells := make([][]string, len(row))
		n := 1
		for i, c := range row {
			cells[i] = wrapText(c, pdf.Sans, fs, cols[i]-8, 8)
			if len(cells[i]) > n {
				n = len(cells[i])
			}
		}
		h := float64(n)*lh + 5
		if p.y+h > pBot {
			p.need(100)
			drawHead()
		}
		x := pm
		for i, ls := range cells {
			for j, ln := range ls {
				p.d.Text(x+4, p.y+10+float64(j)*lh, pdf.Sans, fs, cInk, ln)
			}
			x += cols[i]
		}
		p.y += h
		p.d.Line(pm, p.y, pdf.W-pm, p.y, 0.3, cLine, false)
	}
}
