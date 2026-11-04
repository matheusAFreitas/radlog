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
	p.d.Text(pm, pdf.H-24, pdf.Sans, 7.5, cMuted, "GMC Monitor "+p.r.Version+" · "+p.r.Period)
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
	words := strings.Fields(s)
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
	if len(lines) > max {
		lines = append(lines[:max-1], pdf.Fit(strings.Join(lines[max-1:], " "), f, size, w))
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
					p.d.Rect(x+4, p.y+5, 5, 5, rc, nil)
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
			attn = float64(a.Limit)
		}
	}
	top := math.Max(r.St.Max*1.08, r.S.BgCPM*1.6)
	if attn > 0 && r.St.Max >= attn*0.6 {
		top = math.Max(top, attn*1.1)
	}
	// um pico isolado muito alto achataria o resto: a escala para pouco
	// acima do P99,9 (ou do limite de atenção) e o pico aparece cortado,
	// com o valor escrito em cima
	clipped := false
	if len(r.Pts) > 0 {
		v := make([]float64, len(r.Pts))
		for i, pt := range r.Pts {
			v[i] = pt.CPM
		}
		sort.Float64s(v)
		lim := math.Max(percentile(v, .999)*1.25, r.S.BgCPM*1.8)
		if attn > 0 {
			lim = math.Max(lim, attn*1.25)
		}
		if r.St.Max > lim*1.15 {
			top, clipped = lim, true
		}
	}
	if top <= 0 {
		top = 10
	}
	stepY := math.Max(1, niceStep(top/5))
	top = math.Ceil(top/stepY) * stepY
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
		p.d.TextRight(x0-4, yv(v)+2.5, pdf.Mono, 6.5, cMuted, fmtBR(v, 0))
	}
	if r.S.BgCPM > 0 {
		p.d.Line(x0, yv(r.S.BgCPM), x0+w, yv(r.S.BgCPM), 0.7, cMuted, true)
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
		for _, pt := range pts {
			if len(seg) > 0 && pt.T.Sub(prev) >= 90*time.Second {
				p.d.Polyline(seg, 0.8, cAccent)
				seg = nil
			}
			seg = append(seg, [2]float64{xOf(pt.T), yv(pt.CPM)})
			prev = pt.T
		}
		p.d.Polyline(seg, 0.8, cAccent)
	} else {
		type col struct {
			min, max, sum float64
			n             int
		}
		cs := make([]col, cols)
		for _, pt := range pts {
			i := int(float64(pt.T.Sub(from)) / float64(span) * float64(cols))
			if i < 0 || i >= cols {
				continue
			}
			c := &cs[i]
			if c.n == 0 || pt.CPM < c.min {
				c.min = pt.CPM
			}
			if c.n == 0 || pt.CPM > c.max {
				c.max = pt.CPM
			}
			c.sum += pt.CPM
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
		for _, pt := range pts {
			if pt.CPM > top {
				sp = append(sp, spike{pt.T, pt.CPM})
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
			lbl := fmtBR(k.v, 0)
			p.d.Text(x+4, y0+8, pdf.Mono, 6.5, cWarn, lbl)
		}
	}
	p.d.Rect(x0, y0, w, h, nil, &cLine)
	p.timeAxis(x0, w, y0+h, from, to)
	p.d.Text(pm, y0-4+0, pdf.Sans, 6.5, cMuted, "CPM")
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
		item(cAccent, "CPM de cada minuto", false)
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
	d.Text(pm, p.y+20, pdf.Sans, 9, cMuted, r.Period)
	d.TextRight(pdf.W-pm, p.y, pdf.Sans, 8, cMuted, "GMC Monitor "+r.Version)
	ry := p.y + 11
	if r.Device != "" {
		d.TextRight(pdf.W-pm, ry, pdf.Sans, 8, cMuted, r.Device)
		ry += 11
	}
	d.TextRight(pdf.W-pm, ry, pdf.Sans, 8, cMuted, "gerado em "+r.Generated)
	p.y += 32
	d.Line(pm, p.y, pdf.W-pm, p.y, 1.5, cInk, false)
	p.y += 12

	if !r.HasData || r.St.N == 0 {
		d.Text(pm, p.y+12, pdf.Sans, 10, cMuted, "Sem leituras guardadas neste período.")
		p.footer()
		return d.Bytes(r.Title, "GMC Monitor "+AppVersion)
	}
	st := r.St

	// cartões 4 x 2
	tg := 8.0
	tw := (pdf.W - 2*pm - 3*tg) / 4
	th := 60.0
	bg, bgSub := "—", "ainda sem fundo"
	if r.Bg != "" {
		bg, bgSub = r.Bg+" CPM", "mediana de 30 dias · média do período "+r.VsBg
	}
	rain := r.Rain
	if rain == "" {
		rain = "—"
	}
	cover := fmt.Sprintf("%d dia(s) com dado", r.DaysData)
	if r.Away != "" {
		cover += " · " + r.Away + " fora de casa"
	}
	maxSub := r.MinAt + " · " + r.MaxAt
	if r.MaxLive {
		maxSub += " · máx. = pico ao vivo (maior minuto: " + fmtBR(st.Max, 0) + ")"
	}
	alertSub := "contados por episódio"
	if r.AlertPeakMax > 0 {
		alertSub += " · pico ao vivo " + fmtBR(r.AlertPeakMax, 0) + " CPM"
	}
	tiles := [][4]string{
		{"Média (por minuto)", r.Avg + " CPM", fmt.Sprintf("≈ %s µSv/h · n = %d min", r.AvgUSv, st.N), "m"},
		{"Dose no período", r.Dose + " µSv", "calculada pelo app · ≈ " + r.DoseYear + " mSv/ano (extrapolação)", "m"},
		{"Fundo da casa", bg, bgSub, "m"},
		{"Mínimo · máximo", r.Min + " · " + r.Max + " CPM", maxSub, "m"},
		{"Alertas", r.Alerts, alertSub, "s"},
		{"Chuva", rain, "radar/modelo do app", "s"},
		{"Cobertura", fmtBR(math.Min(100, float64(st.N)/math.Max(1, float64(r.SpanMin))*100), 0) + "%", fmt.Sprintf("%d de %d min · ", st.N, r.SpanMin) + cover, "s"},
		{"Medições de objeto", fmt.Sprint(len(r.Meas)), "no período", "m"},
	}
	for i, t := range tiles {
		x := pm + float64(i%4)*(tw+tg)
		y := p.y + float64(i/4)*(th+tg)
		f := pdf.MonoBold
		if t[3] == "s" {
			f = pdf.SansBold
		}
		p.tile(x, y, tw, th, t[0], t[1], t[2], f)
	}
	p.y += 2*th + tg

	p.section("CPM AO LONGO DO PERÍODO (TODAS AS LEITURAS POR MINUTO)")
	p.chart(r)

	// estatística + histograma lado a lado
	p.section("ESTATÍSTICA DAS LEITURAS POR MINUTO")
	p.need(165)
	top := p.y
	livePeak := "— (nenhum alerta ao vivo)"
	if r.AlertPeakMax > 0 {
		livePeak = fmtBR(r.AlertPeakMax, 0) + " CPM às " + r.alertPeakAt.Format("02/01 15:04")
	}
	rows := [][2]string{
		{"Minutos com leitura (n)", fmt.Sprintf("%d", st.N)},
		{"Média · mediana", fmtBR(st.Mean, 2) + " · " + fmtBR(st.Median, 1) + " CPM"},
		{"Desvio-padrão real", fmtBR(st.SD, 2) + " CPM"},
		{"σ esperado só pelo acaso (√média)", fmtBR(st.PoissonSD, 2) + " CPM"},
		{"Dispersão (real ÷ acaso)", fmtBR(st.Dispersion, 2)},
		{"P5 · P25 · P50", fmtBR(st.P5, 1) + " · " + fmtBR(st.P25, 1) + " · " + fmtBR(st.Median, 1)},
		{"P75 · P95", fmtBR(st.P75, 1) + " · " + fmtBR(st.P95, 1)},
		{"Mínimo · máximo (minutos gravados)", fmtBR(st.Min, 0) + " · " + fmtBR(st.Max, 0) + " CPM"},
		{"Pico ao vivo (alertas, janela de 60 s)", livePeak},
	}
	colW := (pdf.W - 2*pm) * 0.46
	for _, kv := range rows {
		d.Text(pm, p.y+10, pdf.Sans, 8, cMuted, kv[0])
		d.TextRight(pm+colW, p.y+10, pdf.Mono, 8, cInk, kv[1])
		p.y += 13
		d.Line(pm, p.y, pm+colW, p.y, 0.3, cLine, false)
	}
	disp := "Dispersão perto de 1: as leituras variam como o acaso explica."
	if st.Dispersion >= 1.2 {
		disp = "Dispersão acima de ~1,2: há variação além do acaso (chuva, radônio, objetos perto, mudanças no ambiente)."
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
		mx := fmtBR(dd.Max, 0)
		if dd.LivePeak > dd.Max {
			mx = fmtBR(dd.LivePeak, 0) + "*"
			starred = true
		}
		trows = append(trows, []string{dd.Label, fmtBR(dd.Avg, 1), fmtBR(dd.USv, 3), fmtBR(dd.Min, 0), mx, fmtBR(dd.Dose, 2), fmt.Sprintf("%d / 1440", dd.Minutes)})
	}
	cw := pdf.W - 2*pm
	p.table([]string{"Dia", "Média CPM", "µSv/h", "Mín", "Máx", "Dose µSv", "Minutos"}, trows,
		[]float64{cw * .2, cw * .14, cw * .13, cw * .1, cw * .1, cw * .15, cw * .18},
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
		trows = append(trows, []string{b.Label, fmtBR(b.CPM, 1), fmtBR(b.USv, 3), fmt.Sprint(b.N), vs})
	}
	p.table([]string{"Janela", "Mediana CPM", "µSv/h", "Minutos em casa", "Média do período vs."}, trows,
		[]float64{cw * .32, cw * .15, cw * .13, cw * .18, cw * .22},
		[]bool{false, true, true, true, true}, []bool{false, true, true, true, true}, nil)

	p.section("ALERTAS (EPISÓDIOS)")
	if len(r.Episodes) == 0 {
		p.need(20)
		d.Text(pm, p.y+12, pdf.Sans, 8.5, cMuted, "Nenhum alerta no período.")
		p.y += 18
	} else {
		trows = nil
		eps := r.Episodes
		for i := len(eps) - 1; i >= 0; i-- {
			e := eps[i]
			span := e.Start.Format("02/01 15:04") + "–" + e.End.Format("15:04")
			mm := "—"
			if e.N > 0 {
				mm = fmtBR(e.MinC, 0) + " / " + fmtBR(e.MeanC, 1) + " / " + fmtBR(e.MaxC, 0)
			}
			exc := fmt.Sprintf("+%s (×%s)", fmtBR(e.Peak-float64(e.Limit), 0), fmtBR(e.Peak/float64(e.Limit), 2))
			dur := fmtSeconds(e.DurSec)
			if e.Retro {
				dur = "≈ " + fmtMinutes(e.Minutes) // histórico: só minuto a minuto
			}
			lvl := e.Level
			if e.Retro {
				lvl += " (c)"
			}
			trows = append(trows, []string{span, lvl, dur, mm, fmtBR(e.Peak, 0), fmt.Sprint(e.Limit), exc})
		}
		rev := make([]reportEpisode, len(eps))
		for i := range eps {
			rev[i] = eps[len(eps)-1-i]
		}
		p.table([]string{"Quando", "Nível", "Duração", "Mín/méd/máx (min)", "Pico", "Limite", "Acima do limite"}, trows,
			[]float64{cw * .2, cw * .15, cw * .1, cw * .17, cw * .09, cw * .09, cw * .2},
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
		p.need(14)
		d.Text(pm, p.y+10, pdf.Sans, 6.8, cMuted, "Pico: maior leitura ao vivo do alerta (janela de 60 s, lida por segundo). Mín/méd/máx: minutos gravados no episódio. (c): achado no histórico do contador.")
		p.y += 14
		p.episodeCharts()
	}

	p.section("TEMPO ACIMA DOS LIMITES OPERACIONAIS")
	trows = nil
	for _, a := range r.Above {
		trows = append(trows, []string{a.Label, fmt.Sprintf("≥ %d CPM", a.Limit), fmtMinutes(a.Minutes), fmtBR(float64(a.Minutes)/float64(st.N)*100, 2) + "%"})
	}
	p.table([]string{"Limite", "A partir de", "Tempo (minutos gravados)", "Do tempo medido"}, trows,
		[]float64{cw * .2, cw * .2, cw * .35, cw * .25}, []bool{false, true, true, true}, []bool{false, true, true, true}, nil)

	p.section("LACUNAS (SEM LEITURA)")
	missing := r.SpanMin - st.N
	if missing < 0 {
		missing = 0
	}
	p.need(16)
	d.Text(pm, p.y+11, pdf.Sans, 8, cInk, fmt.Sprintf("%s sem leitura no período, em %d trecho(s). Lacuna não conta como zero em nenhum cálculo.", fmtMinutes(missing), len(r.Gaps)))
	p.y += 16
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

	if len(r.Meas) > 0 {
		p.section("MEDIÇÕES DE OBJETO")
		for _, m := range r.Meas {
			p.measBlock(m)
		}
	}

	p.section("CONFIGURAÇÕES E MÉTODO")
	p.kv(r.Config, 150)

	p.section("LIMITAÇÕES")
	for _, l := range []string{
		"A dose é a do contador (monitor de área) no lugar onde ele está, não a dose de quem está perto. Não é dosimetria pessoal.",
		"A conversão CPM → µSv/h depende da calibração do tubo e da energia da radiação; com outras fontes ela pode errar bastante.",
		"Os limites do app são operacionais, configurados pelo usuário; não são limites regulamentares nem limiares de segurança.",
		"CPM sozinho não identifica o radionuclídeo nem o tipo de radiação.",
		"Projeções (mSv/ano) são extrapolações do período, não medições.",
	} {
		lines := wrapText("• "+l, pdf.Sans, 7.5, pdf.W-2*pm, 3)
		p.need(float64(len(lines))*9.5 + 3)
		for i, ln := range lines {
			d.Text(pm, p.y+10+float64(i)*9.5, pdf.Sans, 7.5, cMuted, ln)
		}
		p.y += float64(len(lines))*9.5 + 3
	}
	p.footer()
	return d.Bytes(r.Title+" · "+r.Period, "GMC Monitor "+AppVersion)
}

// episodeCharts desenha, para os episódios escolhidos, o CPM de 30 min antes
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
		title := e.Start.Format("02/01 15:04") + " · " + e.Level + " · pico " + fmtBR(e.Peak, 0)
		d.Text(x0, p.y+9, pdf.SansBold, 7, cInk, pdf.Fit(title, pdf.SansBold, 7, cw))

		w0 := e.Start.Add(-30 * time.Minute).Truncate(time.Minute)
		w1 := e.End.Add(31 * time.Minute).Truncate(time.Minute)
		span := w1.Sub(w0)
		top := math.Max(e.Peak, float64(e.Limit))
		for _, pt := range e.Around {
			top = math.Max(top, pt.CPM)
		}
		for _, sp := range e.Secs {
			top = math.Max(top, sp.CPM)
		}
		top = math.Max(1, top*1.12)
		xOf := func(t time.Time) float64 { return x0 + cw*float64(t.Sub(w0))/float64(span) }
		yv := func(v float64) float64 { return y0 + ch*(1-math.Min(math.Max(v, 0), top)/top) }

		ex1, ex2 := xOf(e.Start), xOf(e.End.Add(time.Minute))
		if ex2-ex1 < 1 {
			ex2 = ex1 + 1
		}
		d.Rect(ex1, y0, ex2-ex1, ch, &bgc, nil)
		if p.r.S.BgCPM > 0 && p.r.S.BgCPM < top {
			d.Line(x0, yv(p.r.S.BgCPM), x0+cw, yv(p.r.S.BgCPM), 0.5, cMuted, true)
		}
		if float64(e.Limit) < top {
			d.Line(x0, yv(float64(e.Limit)), x0+cw, yv(float64(e.Limit)), 0.6, lc, true)
		}
		// por segundo
		var seg [][2]float64
		var prev int64
		for _, sp := range e.Secs {
			t := time.UnixMilli(sp.T)
			if t.Before(w0) || t.After(w1) {
				continue
			}
			if len(seg) > 0 && sp.T-prev > 30000 {
				d.Polyline(seg, 0.4, cSec)
				seg = nil
			}
			seg = append(seg, [2]float64{xOf(t), yv(sp.CPM)})
			prev = sp.T
		}
		d.Polyline(seg, 0.4, cSec)
		// por minuto
		seg = nil
		var pt0 time.Time
		for _, pt := range e.Around {
			if pt.T.Before(w0) || !pt.T.Before(w1) {
				continue
			}
			if len(seg) > 0 && pt.T.Sub(pt0) >= 90*time.Second {
				d.Polyline(seg, 0.9, cAccent)
				seg = nil
			}
			seg = append(seg, [2]float64{xOf(pt.T.Add(30 * time.Second)), yv(pt.CPM)})
			pt0 = pt.T
		}
		d.Polyline(seg, 0.9, cAccent)
		// pico
		if !e.PeakAt.IsZero() && !e.PeakAt.Before(w0) && e.PeakAt.Before(w1) {
			px, py := xOf(e.PeakAt), yv(e.Peak)
			d.Rect(px-1.3, py-1.3, 2.6, 2.6, &lc, nil)
		}
		d.Rect(x0, y0, cw, ch, nil, &cLine)
		d.Text(x0+2, y0+7, pdf.Mono, 5.8, cMuted, fmtBR(top, 0))
		d.Text(x0, y0+ch+8, pdf.Mono, 6, cMuted, w0.Format("15:04"))
		d.TextRight(x0+cw, y0+ch+8, pdf.Mono, 6, cMuted, w1.Format("15:04"))
		dur := fmtSeconds(e.DurSec)
		if e.Retro {
			dur = "≈ " + fmtMinutes(e.Minutes)
		}
		mid := dur
		if e.N > 0 {
			mid += " · méd. " + fmtBR(e.MeanC, 1)
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
