package app

import (
	"fmt"

	"math"
	"radlog/internal/pdf"
	"strings"
	"time"
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
	p.need(60)
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

func (p *reportPDF) chart(s Summary) {
	const h = 150.0
	p.need(h + 30)
	x0, y0, w := pm+26, p.y+6, pdf.W-pm-(pm+26)
	top := s.BgCPM * 1.6
	for _, c := range s.Curve {
		if c.CPM != nil && *c.CPM > top {
			top = *c.CPM
		}
	}
	if top <= 0 {
		top = 10
	}
	top = math.Ceil(top/10) * 10
	yv := func(v float64) float64 { return y0 + h*(1-v/top) }
	for i := 0; i <= 4; i++ {
		v := top * float64(i) / 4
		p.d.Line(x0, yv(v), x0+w, yv(v), 0.4, cLine, false)
		p.d.TextRight(x0-4, yv(v)+3, pdf.Mono, 7, cMuted, fmtBR(v, 0))
	}
	if s.BgCPM > 0 {
		p.d.Line(x0, yv(s.BgCPM), x0+w, yv(s.BgCPM), 0.8, cMuted, true)
	}
	n := len(s.Curve)
	if n >= 2 {
		xv := func(i int) float64 { return x0 + w*float64(i)/float64(n-1) }
		var seg [][2]float64
		for i, c := range s.Curve {
			if c.CPM == nil {
				p.d.Polyline(seg, 1.4, cAccent)
				seg = nil
				continue
			}
			seg = append(seg, [2]float64{xv(i), yv(*c.CPM)})
		}
		p.d.Polyline(seg, 1.4, cAccent)
		lbl := func(ms int64) string {
			t := time.UnixMilli(ms)
			if s.To-s.From <= 26*3600*1000 {
				return t.Format("15:04")
			}
			return t.Format("02/01")
		}
		p.d.Text(x0, y0+h+12, pdf.Mono, 7, cMuted, lbl(s.Curve[0].T))
		mid := lbl(s.Curve[n/2].T)
		p.d.Text(x0+w/2-pdf.TextWidth(mid, pdf.Mono, 7)/2, y0+h+12, pdf.Mono, 7, cMuted, mid)
		p.d.TextRight(x0+w, y0+h+12, pdf.Mono, 7, cMuted, lbl(s.Curve[n-1].T))
	}
	p.y = y0 + h + 24
	p.d.Text(pm, p.y, pdf.Sans, 7.5, cMuted, "Linha verde: média do CPM em cada trecho. Tracejado: fundo da casa (mediana de 30 dias).")
	p.y += 4
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

	if !r.HasData {
		d.Text(pm, p.y+12, pdf.Sans, 10, cMuted, "Sem leituras guardadas neste período.")
		p.footer()
		return d.Bytes(r.Title, "GMC Monitor "+AppVersion)
	}

	// cartões 4 x 2
	gap := 8.0
	tw := (pdf.W - 2*pm - 3*gap) / 4
	th := 60.0
	bg, bgSub := "—", "ainda sem fundo"
	if r.Bg != "" {
		bg, bgSub = r.Bg+" CPM", "mediana de 30 dias · média "+r.VsBg
	}
	rain := r.Rain
	if rain == "" {
		rain = "—"
	}
	away := "do período"
	if r.Away != "" {
		away += " · " + r.Away + " fora de casa"
	}
	tiles := [][4]string{
		{"Média", r.Avg + " CPM", "≈ " + r.AvgUSv + " µSv/h", "m"},
		{"Dose no período", r.Dose + " µSv", r.XRay + r.DoseYear + " mSv/ano nesse ritmo", "m"},
		{"Fundo da casa", bg, bgSub, "m"},
		{"Mínimo · máximo", r.Min + " · " + r.Max + " CPM", r.MinAt + " · " + r.MaxAt, "m"},
		{"Alertas", r.Alerts, "contados por episódio", "s"},
		{"Chuva", rain, "radar/modelo do app", "s"},
		{"Medido", r.Coverage, away, "m"},
		{"Medições de objeto", fmt.Sprint(len(r.Meas)), "no período", "m"},
	}
	for i, t := range tiles {
		x := pm + float64(i%4)*(tw+gap)
		y := p.y + float64(i/4)*(th+gap)
		f := pdf.MonoBold
		if t[3] == "s" {
			f = pdf.SansBold
		}
		p.tile(x, y, tw, th, t[0], t[1], t[2], f)
	}
	p.y += 2*th + gap

	p.section("CPM MÉDIO")
	p.chart(r.S)

	p.section("POR DIA")
	var rows [][]string
	for _, dd := range r.Days {
		rows = append(rows, []string{dd.Label, fmtBR(dd.Avg, 1), fmtBR(dd.USv, 3), fmtBR(dd.Min, 0), fmtBR(dd.Max, 0), fmtBR(dd.Dose, 2), fmt.Sprintf("%d min", dd.Minutes)})
	}
	cw := pdf.W - 2*pm
	p.table([]string{"Dia", "Média CPM", "µSv/h", "Mín", "Máx", "Dose µSv", "Medido"}, rows,
		[]float64{cw * .2, cw * .14, cw * .13, cw * .1, cw * .1, cw * .15, cw * .18},
		[]bool{false, true, true, true, true, true, true}, []bool{false, true, true, true, true, true, true}, nil)

	p.section("ALERTAS")
	if len(r.AlertList) == 0 {
		p.need(20)
		d.Text(pm, p.y+12, pdf.Sans, 8.5, cMuted, "Nenhum alerta no período.")
		p.y += 18
	} else {
		rows = nil
		for _, a := range r.AlertList {
			rows = append(rows, []string{a.When, a.Level, a.Span, a.Peak})
		}
		al := r.AlertList
		p.table([]string{"Dia", "Nível", "Horário", "Pico"}, rows,
			[]float64{cw * .14, cw * .2, cw * .4, cw * .26},
			[]bool{false, false, false, true}, []bool{false, false, true, true},
			func(i int) *pdf.RGB {
				switch al[i].LevelCls {
				case "perigo":
					return &cWarn
				case "alarme":
					return &cAlarm
				}
				return &cAtt
			})
	}

	if len(r.Meas) > 0 {
		p.section("MEDIÇÕES DE OBJETO")
		rows = nil
		for _, m := range r.Meas {
			rows = append(rows, []string{m.When, m.Name, m.Result, m.Verdict})
		}
		p.table([]string{"Quando", "Objeto", "Resultado", "Conclusão"}, rows,
			[]float64{cw * .14, cw * .3, cw * .34, cw * .22},
			[]bool{false, false, false, false}, []bool{true, false, true, false}, nil)
	}

	// nota final
	p.need(40)
	p.y += 16
	d.Rect(pm, p.y, pdf.W-2*pm, 30, &cSoft, nil)
	d.Text(pm+8, p.y+12, pdf.Sans, 7.5, cMuted, "Calibração: "+r.Cal+" · correção de tempo morto: "+r.DeadTime+".")
	d.Text(pm+8, p.y+23, pdf.Sans, 7.5, cMuted, "A dose é a do contador (monitor de área), não a de quem está perto dele. Referências são aproximadas.")
	p.y += 30
	p.footer()
	return d.Bytes(r.Title+" · "+r.Period, "GMC Monitor "+AppVersion)
}
