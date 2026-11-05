package app

import (
	"errors"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"strings"
	"time"

	"radlog/internal/pdf"
)

// ---------- relatório de uma medição de objeto ----------
//
// Um PDF só da medição: as duas fases (fundo e objeto) com contagens,
// tempos e incertezas, o gráfico segundo a segundo, a distribuição das
// contagens por segundo comparada com Poisson, o limite de detecção para o
// tempo usado e quanto tempo seria preciso para uma precisão melhor.

// measPhaseStats é a estatística das contagens por segundo de uma fase.
type measPhaseStats struct {
	N          int     // segundos
	Mean, Var  float64 // contagens por segundo
	Dispersion float64 // variância ÷ média (1 = Poisson)
	Max        float64
	Hist       []int // quantos segundos com 0, 1, 2… contagens
}

func phaseSeconds(samples []MeasureSample, phase string) []float64 {
	var out []float64
	for _, s := range samples {
		if s.Phase == phase {
			out = append(out, float64(s.CPS))
		}
	}
	return out
}

func calcPhaseStats(v []float64) measPhaseStats {
	st := measPhaseStats{N: len(v)}
	if len(v) == 0 {
		return st
	}
	for _, x := range v {
		st.Mean += x
		st.Max = math.Max(st.Max, x)
	}
	st.Mean /= float64(len(v))
	if len(v) > 1 {
		for _, x := range v {
			st.Var += (x - st.Mean) * (x - st.Mean)
		}
		st.Var /= float64(len(v) - 1)
	}
	if st.Mean > 0 {
		st.Dispersion = st.Var / st.Mean
	}
	st.Hist = make([]int, int(st.Max)+1)
	for _, x := range v {
		st.Hist[int(x)]++
	}
	return st
}

// detectionLimitCPM é o limite de detecção de Currie para duas medições do
// mesmo tempo: L_D = 2,71 + 4,65·√B contagens, em CPM.
func detectionLimitCPM(bg MeasurePhase) float64 {
	if bg.Seconds <= 0 {
		return 0
	}
	ld := 2.71 + 4.65*math.Sqrt(float64(bg.Counts))
	return ld / (float64(bg.Seconds) / 60)
}

// secondsForPrecision diz quantos segundos de cada fase dariam o líquido
// com incerteza relativa rel (0,1 = ±10%).
func secondsForPrecision(bg, obj MeasurePhase, rel float64) float64 {
	rb, ro := bg.CPM/60, obj.CPM/60
	net := ro - rb
	if net <= 0 || rel <= 0 {
		return math.Inf(1)
	}
	return (rb + ro) / (rel * rel * net * net)
}

func fmtDurSec(s float64) string {
	switch {
	case math.IsInf(s, 1) || s > 30*24*3600:
		return "—"
	case s < 90:
		return fmt.Sprintf("%.0f s", math.Ceil(s))
	case s < 5400:
		return fmt.Sprintf("%.0f min", math.Ceil(s/60))
	}
	return fmtBR(s/3600, 1) + " h"
}

// ExportMeasureReport salva o PDF de uma medição salva e abre.
func (a *App) ExportMeasureReport(id int64) (string, error) {
	path, err := a.saveMeasureReport(id)
	if err == nil {
		_ = openFolder(path)
	}
	return path, err
}

// OpenReportFile abre um PDF que o app salvou na pasta de relatórios.
func (a *App) OpenReportFile(path string) error {
	dir := a.reportDir().Dir
	rel, err := filepath.Rel(dir, path)
	if dir == "" || err != nil || strings.HasPrefix(rel, "..") || !strings.HasSuffix(strings.ToLower(path), ".pdf") {
		return errors.New("arquivo fora da pasta de relatórios")
	}
	return openFolder(path)
}

func (a *App) saveMeasureReport(id int64) (string, error) {
	a.mu.Lock()
	var m *SavedMeasure
	for i := range a.measurements {
		if a.measurements[i].ID == id {
			c := a.measurements[i]
			m = &c
		}
	}
	cal := a.convCalLocked()
	lv := a.settings.Levels.norm()
	info, connected := a.info, a.state == "conectado"
	a.mu.Unlock()
	if m == nil {
		return "", errors.New("medição não encontrada")
	}
	dir := a.reportDir().Dir
	if dir == "" {
		return "", errors.New("não achei uma pasta para salvar (escolha uma nas Configurações → Relatórios)")
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", fmt.Errorf("não consegui criar a pasta %s: %w", dir, err)
	}
	slug := strings.Trim(badFileChars.ReplaceAllString(strings.ReplaceAll(strings.ToLower(asciiFold(m.Name)), " ", "-"), ""), "-")
	if slug == "" {
		slug = "objeto"
	}
	t := time.UnixMilli(m.Time)
	dst := uniquePath(filepath.Join(dir, fmt.Sprintf("gmc-medicao-%s-%s.pdf", slug, t.Format("2006-01-02_15h04"))))
	dev := ""
	if connected && info.Version != "" {
		dev = info.Version
	}
	calTxt := fmtBR(cal.USv(100)/100, 4) + " µSv/h por CPM"
	dt := "desligada"
	if lv.DeadTimeOn {
		dt = fmt.Sprintf("ligada (τ = %s µs)", fmtBR(lv.DeadTimeUs, 0))
	}
	if err := os.WriteFile(dst, renderMeasurePDF(*m, dev, calTxt, dt), 0o644); err != nil {
		return "", err
	}
	a.logf("relatório da medição salvo: %s", dst)
	return dst, nil
}

// asciiFold tira acentos para o nome do arquivo.
func asciiFold(s string) string {
	r := strings.NewReplacer("á", "a", "à", "a", "â", "a", "ã", "a", "é", "e", "ê", "e", "í", "i", "ó", "o", "ô", "o", "õ", "o", "ú", "u", "ç", "c",
		"Á", "A", "À", "A", "Â", "A", "Ã", "A", "É", "E", "Ê", "E", "Í", "I", "Ó", "O", "Ô", "O", "Õ", "O", "Ú", "U", "Ç", "C")
	return r.Replace(s)
}

func renderMeasurePDF(m SavedMeasure, device, calTxt, deadTime string) []byte {
	t0 := time.UnixMilli(m.Time)
	title := "Medição: " + m.Name
	period := t0.Format("02/01/2006 15:04") + " · fundo " + fmt.Sprint(m.Bg.Seconds) + " s + objeto " + fmt.Sprint(m.Obj.Seconds) + " s"
	p := &reportPDF{d: pdf.New(), y: 50, page: 1, r: reportData{Version: AppVersion, Period: period}}
	d := p.d
	// nome comprido: diminui a letra até 14 e, se ainda não couber, quebra em
	// até 3 linhas (nunca corta o nome)
	tmax := pdf.W - 2*pm - 150
	ts := 18.0
	for ts > 14 && pdf.TextWidth(title, pdf.SansBold, ts) > tmax {
		ts -= 0.5
	}
	tl := wrapText(title, pdf.SansBold, ts, tmax, 3)
	for i, ln := range tl {
		d.Text(pm, p.y+4+float64(i)*(ts+3), pdf.SansBold, ts, cInk, ln)
	}
	extra := float64(len(tl)-1) * (ts + 3)
	d.Text(pm, p.y+20+extra, pdf.Sans, 9, cMuted, period)
	d.TextRight(pdf.W-pm, p.y, pdf.Sans, 8, cMuted, "GMC Monitor "+AppVersion)
	ry := p.y + 11
	if device != "" {
		d.TextRight(pdf.W-pm, ry, pdf.Sans, 8, cMuted, device)
		ry += 11
	}
	d.TextRight(pdf.W-pm, ry, pdf.Sans, 8, cMuted, "gerado em "+time.Now().Format("02/01/2006 15:04"))
	p.y += 32 + extra
	d.Line(pm, p.y, pdf.W-pm, p.y, 1.5, cInk, false)
	p.y += 12

	r := m.Result
	sign := "+"
	if r.NetCPM < 0 {
		sign = ""
	}
	// conclusão em destaque
	vc := cMuted
	switch r.Level {
	case "sim":
		vc = cWarn
	case "talvez":
		vc = cAtt
	}
	verdict := r.Verdict
	if verdict != "" {
		verdict = strings.ToUpper(verdict[:1]) + verdict[1:]
	}
	d.Rect(pm, p.y, 4, 30, &vc, nil)
	d.Text(pm+12, p.y+12, pdf.SansBold, 11, cInk, pdf.Fit(verdict, pdf.SansBold, 11, pdf.W-2*pm-12))
	d.Text(pm+12, p.y+26, pdf.Sans, 8.5, cMuted, fmt.Sprintf("Diferença de %s σ: até ~2σ é a variação natural da contagem; de 3σ para cima a diferença é real.", fmtBR(r.Sigma, 1)))
	p.y += 42

	// cartões
	tg := 8.0
	tw := (pdf.W - 2*pm - 3*tg) / 4
	th := 60.0
	ld := detectionLimitCPM(m.Bg)
	tiles := [][4]string{
		{"Fundo (objeto longe)", fmtBR(m.Bg.CPM, 1) + " CPM", "± " + fmtBR(m.Bg.ErrCPM, 1) + " · " + fmt.Sprintf("%d contagens em %d s", m.Bg.Counts, m.Bg.Seconds), "m"},
		{"Com o objeto", fmtBR(m.Obj.CPM, 1) + " CPM", "± " + fmtBR(m.Obj.ErrCPM, 1) + " · " + fmt.Sprintf("%d contagens em %d s", m.Obj.Counts, m.Obj.Seconds), "m"},
		{"Líquido (objeto − fundo)", sign + fmtBR(r.NetCPM, 1) + " CPM", "± " + fmtBR(r.NetErr, 1) + " CPM · ≈ " + fmtBR(r.NetUSv, 3) + " µSv/h no tubo", "m"},
		{"Objeto ÷ fundo", fmtBR(r.Ratio, 2) + "×", fmtBR(r.Sigma, 1) + " σ acima do acaso", "m"},
	}
	for i, tl := range tiles {
		p.tile(pm+float64(i)*(tw+tg), p.y, tw, th, tl[0], tl[1], tl[2], pdf.MonoBold)
	}
	p.y += th + 6

	// gráfico segundo a segundo
	p.section("SEGUNDO A SEGUNDO")
	if len(m.Samples) == 0 {
		p.need(16)
		d.Text(pm, p.y+11, pdf.Sans, 8.5, cMuted, "Esta medição é de antes da versão 0.20.3 e não guardou os segundos.")
		p.y += 16
	} else {
		p.measChart(m)
	}

	// distribuição
	bgS, objS := calcPhaseStats(phaseSeconds(m.Samples, "fundo")), calcPhaseStats(phaseSeconds(m.Samples, "objeto"))
	if bgS.N > 0 && objS.N > 0 {
		p.section("CONTAGENS POR SEGUNDO × ACASO (POISSON)")
		p.need(130)
		top := p.y
		hw := (pdf.W - 2*pm - 24) / 2
		p.measHist("Fundo", bgS, pm, top+4, hw, 90, cMuted)
		p.measHist("Com o objeto", objS, pm+hw+24, top+4, hw, 90, cAccent)
		p.y = top + 118
		d.Text(pm, p.y, pdf.Sans, 7, cMuted, "Barras: quantos segundos tiveram 0, 1, 2… contagens. Linha roxa: o esperado só pelo acaso (Poisson) com a mesma média.")
		p.y += 6
	}

	// estatística por fase
	p.section("ESTATÍSTICA DAS FASES")
	row := func(name string, ph MeasurePhase, st measPhaseStats) []string {
		disp, mx, mean := "—", "—", "—"
		if st.N > 0 {
			disp, mx, mean = fmtBR(st.Dispersion, 2), fmtBR(st.Max, 0), fmtBR(st.Mean, 3)
		}
		when := "—"
		if ph.Start > 0 {
			when = time.UnixMilli(ph.Start).Format("15:04:05") + "–" + time.UnixMilli(ph.End).Format("15:04:05")
		}
		return []string{name, when, fmt.Sprint(ph.Counts), fmt.Sprint(ph.Seconds), fmtBR(ph.CPM, 1) + " ± " + fmtBR(ph.ErrCPM, 1), mean, disp, mx}
	}
	cw := pdf.W - 2*pm
	p.table([]string{"Fase", "Horário", "Contagens", "Segundos", "CPM ± σ", "Média/s", "Var ÷ média", "Máx/s"},
		[][]string{row("Fundo", m.Bg, bgS), row("Com o objeto", m.Obj, objS)},
		[]float64{cw * .14, cw * .2, cw * .1, cw * .09, cw * .16, cw * .1, cw * .11, cw * .1},
		[]bool{false, false, true, true, true, true, true, true}, []bool{false, true, true, true, true, true, true, true}, nil)
	p.need(22)
	note := "Var ÷ média perto de 1: a contagem variou como o acaso explica. Bem acima de 1: algo mudou durante a fase (o objeto mexeu, interferência, fonte variando)."
	for i, ln := range wrapText(note, pdf.Sans, 7, cw, 2) {
		d.Text(pm, p.y+10+float64(i)*9, pdf.Sans, 7, cMuted, ln)
	}
	p.y += 22

	// limite de detecção e precisão
	p.section("O QUE ESTA MEDIÇÃO CONSEGUE VER")
	rows := [][2]string{
		{"Limite de detecção", fmt.Sprintf("%s CPM acima do fundo (Currie, 95%%, com %d s de fundo). Abaixo disso, um objeto fraco pode passar despercebido.", fmtBR(ld, 1), m.Bg.Seconds)},
	}
	if r.NetCPM > 0 {
		rows = append(rows,
			[2]string{"Precisão do líquido", fmt.Sprintf("± %s%% (1σ)", fmtBR(r.NetErr/r.NetCPM*100, 0))},
			[2]string{"Tempo para ±10%", fmtDurSec(secondsForPrecision(m.Bg, m.Obj, 0.10)) + " em cada fase"},
			[2]string{"Tempo para ±5%", fmtDurSec(secondsForPrecision(m.Bg, m.Obj, 0.05)) + " em cada fase"},
		)
	}
	p.kv(rows, 120)

	// geometria e método
	p.section("GEOMETRIA, CONTADOR E MÉTODO")
	geo := strings.TrimSpace(m.Note)
	if geo == "" {
		geo = "não anotada (na aba Medir, a anotação da medição guarda distância, lado encostado no tubo e embalagem)"
	}
	if device == "" {
		device = "contador não conectado na hora do relatório"
	}
	p.kv([][2]string{
		{"Geometria / anotação", geo},
		{"Contador", device},
		{"Calibração", calTxt + " (para gama do Cs-137)"},
		{"Tempo morto", deadTime},
		{"Contagens", "soma do CPS do contador a cada segundo, em cada fase"},
		{"Incerteza", "√N de cada fase (Poisson); a do líquido junta as duas: raiz da soma dos quadrados"},
		{"Significância", "líquido ÷ incerteza do líquido (σ)"},
		{"Limite de detecção", "Currie: L_D = 2,71 + 4,65·√B contagens, com B = contagens do fundo, mesmo tempo nas duas fases"},
	}, 120)

	p.section("LIMITAÇÕES")
	for _, l := range []string{
		"O µSv/h é a taxa no tubo, na posição medida, com a calibração para Cs-137. Não é a dose de quem segura o objeto e muda muito com a distância.",
		"O tubo do GMC-800 vê gama e parte do beta. Objetos que emitem mais beta (ex.: tungstênio toriado, vidro de urânio) podem dar CPM alto com µSv/h pouco confiável.",
		"CPM não identifica o radionuclídeo. Para isso, é preciso espectrometria (ex.: RadiaCode).",
	} {
		lines := wrapText("• "+l, pdf.Sans, 7.5, cw, 3)
		p.need(float64(len(lines))*9.5 + 3)
		for i, ln := range lines {
			d.Text(pm, p.y+10+float64(i)*9.5, pdf.Sans, 7.5, cMuted, ln)
		}
		p.y += float64(len(lines))*9.5 + 3
	}
	p.footer()
	return d.Bytes(title, "GMC Monitor "+AppVersion)
}

// measChart: contagem por segundo (média de 10 s, em CPM) das duas fases,
// com a média e a margem de erro (±1σ) de cada uma.
func (p *reportPDF) measChart(m SavedMeasure) {
	d := p.d
	p.need(150)
	x0, y0, w, h := pm+28, p.y+8, pdf.W-2*pm-28, 110.0
	var tmax float64
	for _, s := range m.Samples {
		tmax = math.Max(tmax, float64(s.T)/1000)
	}
	if tmax <= 0 {
		tmax = 1
	}
	// média móvel de 10 s em CPM, por fase
	type pt struct{ t, v float64 }
	smooth := func(phase string) []pt {
		var out []pt
		var win []float64
		for _, s := range m.Samples {
			if s.Phase != phase {
				continue
			}
			win = append(win, float64(s.CPS))
			if len(win) > 10 {
				win = win[1:]
			}
			if len(win) < 10 {
				continue // começo da fase: espera ter 10 s para a média não pular
			}
			sum := 0.0
			for _, x := range win {
				sum += x
			}
			out = append(out, pt{float64(s.T) / 1000, sum / float64(len(win)) * 60})
		}
		return out
	}
	bgL, objL := smooth("fundo"), smooth("objeto")
	top := math.Max(m.Obj.CPM+m.Obj.ErrCPM, m.Bg.CPM+m.Bg.ErrCPM)
	for _, q := range append(append([]pt{}, bgL...), objL...) {
		top = math.Max(top, q.v)
	}
	top = math.Max(1, top*1.1)
	step := math.Max(1, niceStep(top/4))
	top = math.Ceil(top/step) * step
	xv := func(t float64) float64 { return x0 + w*t/tmax }
	yv := func(v float64) float64 { return y0 + h*(1-math.Min(math.Max(v, 0), top)/top) }

	// faixas das fases
	span := func(phase string) (float64, float64) {
		a, b := math.Inf(1), math.Inf(-1)
		for _, s := range m.Samples {
			if s.Phase == phase {
				t := float64(s.T) / 1000
				a, b = math.Min(a, t-1), math.Max(b, t)
			}
		}
		return a, b
	}
	soft := pdf.Hex("#EEF1F3")
	objBg := pdf.Hex("#E3F4EA")
	if a, b := span("fundo"); b > a {
		d.Rect(xv(math.Max(0, a)), y0, xv(b)-xv(math.Max(0, a)), h, &soft, nil)
	}
	oa, ob := span("objeto")
	if ob > oa {
		d.Rect(xv(oa), y0, xv(ob)-xv(oa), h, &objBg, nil)
	}
	for v := 0.0; v <= top+1e-9; v += step {
		d.Line(x0, yv(v), x0+w, yv(v), 0.3, cLine, false)
		d.TextRight(x0-4, yv(v)+2.5, pdf.Mono, 6.5, cMuted, fmtBR(v, 0))
	}
	// média ± σ de cada fase
	band := func(ph MeasurePhase, a, b float64, c pdf.RGB) {
		if b <= a {
			return
		}
		y1, y2 := yv(ph.CPM+ph.ErrCPM), yv(ph.CPM-ph.ErrCPM)
		bc := c.Tint(0.65)
		d.Rect(xv(math.Max(0, a)), y1, xv(b)-xv(math.Max(0, a)), y2-y1, &bc, nil)
		d.Line(xv(math.Max(0, a)), yv(ph.CPM), xv(b), yv(ph.CPM), 1.1, c, false)
	}
	ba, bb := span("fundo")
	band(m.Bg, ba, bb, cMuted)
	band(m.Obj, oa, ob, cAccent)
	line := func(l []pt, c pdf.RGB) {
		var seg [][2]float64
		for _, q := range l {
			seg = append(seg, [2]float64{xv(q.t), yv(q.v)})
		}
		d.Polyline(seg, 0.6, c)
	}
	line(bgL, pdf.Hex("#8C99A3"))
	line(objL, pdf.Hex("#5FBF8A"))
	d.Rect(x0, y0, w, h, nil, &cLine)
	d.Text(pm, y0-3, pdf.Sans, 6.5, cMuted, "CPM")
	// eixo de tempo em segundos
	ts := niceStep(tmax / 6)
	for t := 0.0; t <= tmax+1e-9; t += ts {
		x := xv(t)
		d.Line(x, y0+h, x, y0+h+3, 0.5, cMuted, false)
		lbl := fmtBR(t, 0) + " s"
		lw := pdf.TextWidth(lbl, pdf.Mono, 6.5)
		if x+lw/2 <= x0+w+2 {
			d.Text(x-lw/2, y0+h+11, pdf.Mono, 6.5, cMuted, lbl)
		}
	}
	p.y = y0 + h + 24
	lx := pm
	leg := func(c pdf.RGB, txt string, thick float64) {
		d.Line(lx, p.y-2.5, lx+12, p.y-2.5, thick, c, false)
		d.Text(lx+15, p.y, pdf.Sans, 7, cMuted, txt)
		lx += 15 + pdf.TextWidth(txt, pdf.Sans, 7) + 12
	}
	leg(pdf.Hex("#8C99A3"), "fundo (média de 10 s)", 0.6)
	leg(pdf.Hex("#5FBF8A"), "com o objeto (média de 10 s)", 0.6)
	leg(cMuted, "média do fundo ± σ", 1.1)
	leg(cAccent, "média com o objeto ± σ", 1.1)
	p.y += 6
}

// measHist: segundos por número de contagens, com a curva de Poisson.
func (p *reportPDF) measHist(title string, st measPhaseStats, x0, y0, w, h float64, c pdf.RGB) {
	d := p.d
	d.Text(x0, y0+6, pdf.SansBold, 7.5, cInk, fmt.Sprintf("%s · média %s/s · var ÷ média %s", title, fmtBR(st.Mean, 2), fmtBR(st.Dispersion, 2)))
	y0 += 14
	h -= 14
	kmax := len(st.Hist) - 1
	for k := kmax; k < 60; k++ { // a curva esperada também precisa caber
		if poissonPMF(k, st.Mean)*float64(st.N) < 0.5 {
			break
		}
		kmax = k
	}
	if kmax > 40 {
		kmax = 40
	}
	top := 1.0
	for k := 0; k <= kmax; k++ {
		if k < len(st.Hist) {
			top = math.Max(top, float64(st.Hist[k]))
		}
		top = math.Max(top, poissonPMF(k, st.Mean)*float64(st.N))
	}
	top *= 1.1
	bw := w / float64(kmax+1)
	bc := c.Tint(0.55)
	var curve [][2]float64
	for k := 0; k <= kmax; k++ {
		x := x0 + float64(k)*bw
		if k < len(st.Hist) && st.Hist[k] > 0 {
			bh := h * float64(st.Hist[k]) / top
			d.Rect(x+bw*0.12, y0+h-bh, bw*0.76, bh, &bc, nil)
		}
		curve = append(curve, [2]float64{x + bw/2, y0 + h - h*poissonPMF(k, st.Mean)*float64(st.N)/top})
		lstep := int(math.Max(1, math.Ceil(float64(kmax+1)/12)))
		if k%lstep == 0 {
			lbl := fmt.Sprint(k)
			d.Text(x+bw/2-pdf.TextWidth(lbl, pdf.Mono, 6)/2, y0+h+8, pdf.Mono, 6, cMuted, lbl)
		}
	}
	d.Polyline(curve, 0.9, cExp)
	d.Line(x0, y0+h, x0+w, y0+h, 0.5, cLine, false)
}
