package app

import (
	"errors"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"radlog/internal/store"

	wruntime "github.com/wailsapp/wails/v2/pkg/runtime"
)

// ---------- relatório do período (aba Resumo → Relatório) ----------
//
// Um PDF (pdf.go/reportpdf.go) com os mesmos números da aba Resumo.

type reportDay struct {
	Label         string
	Avg, Min, Max float64
	USv, Dose     float64
	Minutes       int
}

type reportAlert struct {
	When, Level, LevelCls, Span, Peak string
}

type reportMeas struct {
	When, Name, Result, Verdict string
}

type reportData struct {
	Title, Period, Generated, Device, Version string
	S                                         Summary
	HasData                                   bool
	Avg, AvgUSv, Min, Max, MinAt, MaxAt       string
	Dose, DoseYear, XRay, Bg, BgUSv, VsBg     string
	Coverage, Away                            string
	Alerts                                    string
	Rain                                      string
	Days                                      []reportDay
	AlertList                                 []reportAlert
	Meas                                      []reportMeas
	Cal, DeadTime                             string

	// relatório técnico (reportstats.go)
	From, To     time.Time     // período; To não passa de agora
	Pts          []store.Point // leituras por minuto do período
	St           minuteStats
	Gaps         []gap
	SpanMin      int // minutos do período (até agora)
	DaysData     int
	Bins         []histBin
	Episodes     []reportEpisode
	Above        []reportAbove
	Config       [][2]string // configurações e método (rótulo, valor)
	AlertPeakMax float64     // maior pico dos alertas (pode vir do detalhe por segundo)
}

type reportEpisode struct {
	Start, End        time.Time
	Level, LevelCls   string
	Minutes           int
	N                 int // minutos com leitura dentro do episódio
	MinC, MeanC, MaxC float64
	Peak              float64 // pico do alerta (por segundo, se ao vivo)
	Limit             int
	Retro             bool
}

type reportAbove struct {
	Label   string
	Limit   int
	Minutes int
}

func fmtBR(v float64, dec int) string {
	s := fmt.Sprintf("%.*f", dec, v)
	return strings.Replace(s, ".", ",", 1)
}

func levelName(l string) string {
	switch l {
	case "perigo":
		return "Perigo"
	case "alarme":
		return "Alarme"
	}
	return "Atenção"
}

func fmtMinutes(m int) string {
	if m < 60 {
		return fmt.Sprintf("%d min", m)
	}
	h, r := m/60, m%60
	if r == 0 {
		return fmt.Sprintf("%d h", h)
	}
	return fmt.Sprintf("%d h %02d min", h, r)
}

// buildReport monta os dados do relatório de [from, to).
func (a *App) buildReport(fromMs, toMs int64, label string) (reportData, error) {
	s, err := a.GetSummary(fromMs, toMs)
	if err != nil {
		return reportData{}, err
	}
	from, to := time.UnixMilli(s.From), time.UnixMilli(s.To)
	a.mu.Lock()
	cal := a.convCalLocked()
	info := a.info
	connected := a.state == "conectado"
	demo := a.demo.Active
	lv := a.settings.Levels.norm()
	var evs []Event
	for _, e := range a.events {
		if e.Kind == kindAlert && e.Time >= s.From && e.Time < s.To && (!e.Demo || demo) {
			evs = append(evs, e)
		}
	}
	var meas []SavedMeasure
	for _, m := range a.measurements {
		if m.Time >= s.From && m.Time < s.To {
			meas = append(meas, m)
		}
	}
	a.mu.Unlock()

	r := reportData{
		Title:     "Relatório de radiação",
		Period:    fmt.Sprintf("%s · %s a %s", label, from.Format("02/01/2006 15:04"), to.Format("02/01/2006 15:04")),
		Generated: time.Now().Format("02/01/2006 15:04"),
		Version:   AppVersion,
		S:         s,
		HasData:   s.Minutes > 0,
	}
	if connected && info.Version != "" {
		r.Device = info.Version
		if info.Serial != "" {
			r.Device += " · série " + info.Serial
		}
	}
	if s.Avg != nil {
		r.Avg = fmtBR(*s.Avg, 1)
		r.AvgUSv = fmtBR(s.AvgUSv, 3)
		r.DoseYear = fmtBR(s.AvgUSv*24*365/1000, 2)
	}
	if s.Min != nil {
		r.Min = fmtBR(*s.Min, 0)
		r.MinAt = time.UnixMilli(s.MinAt).Format("02/01 15:04")
	}
	if s.Max != nil {
		r.Max = fmtBR(*s.Max, 0)
		r.MaxAt = time.UnixMilli(s.MaxAt).Format("02/01 15:04")
	}
	r.Dose = fmtBR(s.Dose, 2)
	if xr := s.Dose / 20; xr >= 1 {
		r.XRay = "≈ " + fmtBR(xr, 1) + " raios-X de tórax · "
	} else if xr >= 0.001 {
		r.XRay = fmt.Sprintf("≈ 1/%.0f de um raio-X de tórax · ", 1/xr)
	}
	if s.BgOK && s.BgCPM > 0 {
		r.Bg = fmtBR(s.BgCPM, 1)
		r.BgUSv = fmtBR(cal.USv(s.BgCPM), 3)
		if s.Avg != nil {
			sign := "+"
			if s.VsBgPct < 0 {
				sign = ""
			}
			r.VsBg = sign + fmtBR(s.VsBgPct, 1) + "%"
		}
	}
	r.Coverage = fmtBR(s.Coverage*100, 0) + "%"
	if s.AwayMinutes > 0 {
		r.Away = fmtMinutes(s.AwayMinutes)
	}
	var parts []string
	if s.Danger > 0 {
		parts = append(parts, fmt.Sprintf("%d perigo", s.Danger))
	}
	if s.Alarm > 0 {
		parts = append(parts, plural(s.Alarm, "1 alarme", fmt.Sprintf("%d alarmes", s.Alarm)))
	}
	if s.Attention > 0 {
		parts = append(parts, plural(s.Attention, "1 atenção", fmt.Sprintf("%d atenções", s.Attention)))
	}
	r.Alerts = "nenhum"
	if len(parts) > 0 {
		r.Alerts = strings.Join(parts, ", ")
	}
	if s.RainKnown {
		r.Rain = fmt.Sprintf("%s mm em %s", fmtBR(s.RainMM, 1), plural(s.RainHours, "1 hora", fmt.Sprintf("%d horas", s.RainHours)))
		if s.RainHours == 0 {
			r.Rain = "sem chuva"
		}
	}

	// leituras por minuto do período (base de tudo o que vem abaixo)
	pts, err := a.points(from.Truncate(time.Minute), to)
	if err == nil {
		k := 0
		for _, p := range pts {
			if !p.T.Before(from) && p.T.Before(to) {
				pts[k] = p
				k++
			}
		}
		pts = pts[:k]
		sort.Slice(pts, func(i, j int) bool { return pts[i].T.Before(pts[j].T) })
	}
	effTo := to
	if now := time.Now(); effTo.After(now) {
		effTo = now
	}
	r.From, r.To, r.Pts = from, effTo, pts
	r.St = calcMinuteStats(pts)
	r.Gaps = findGaps(pts, from, effTo)
	r.SpanMin = int(math.Round(effTo.Sub(from).Minutes()))
	r.DaysData = daysWithData(pts)
	r.Bins = histogram(pts, r.St.Mean)
	if r.St.N > 0 {
		r.Min = fmtBR(r.St.Min, 0)
		r.MinAt = r.St.MinAt.Format("02/01 15:04")
		r.Max = fmtBR(r.St.Max, 0)
		r.MaxAt = r.St.MaxAt.Format("02/01 15:04")
	}
	pct := 0.0
	if r.SpanMin > 0 {
		pct = math.Min(100, float64(r.St.N)/float64(r.SpanMin)*100)
	}
	r.Coverage = fmt.Sprintf("%s%% · %d de %d min", fmtBR(pct, 0), r.St.N, r.SpanMin)

	// por dia
	if err == nil {
		type acc struct {
			sum, dose, min, max float64
			n                   int
		}
		byDay := map[int64]*acc{}
		var keys []int64
		for _, p := range pts {
			if p.T.Before(from) {
				continue
			}
			y, m, d := p.T.Date()
			k := time.Date(y, m, d, 0, 0, 0, 0, p.T.Location()).UnixMilli()
			x := byDay[k]
			if x == nil {
				x = &acc{min: p.CPM, max: p.CPM}
				byDay[k] = x
				keys = append(keys, k)
			}
			x.sum += p.CPM
			x.dose += cal.USv(p.CPM) / 60
			x.min = math.Min(x.min, p.CPM)
			x.max = math.Max(x.max, p.CPM)
			x.n++
		}
		sort.Slice(keys, func(i, j int) bool { return keys[i] < keys[j] })
		wd := []string{"dom", "seg", "ter", "qua", "qui", "sex", "sáb"}
		for _, k := range keys {
			x := byDay[k]
			t := time.UnixMilli(k)
			avg := x.sum / float64(x.n)
			r.Days = append(r.Days, reportDay{
				Label: wd[t.Weekday()] + " " + t.Format("02/01"),
				Avg:   avg, Min: x.min, Max: x.max, USv: cal.USv(avg), Dose: x.dose, Minutes: x.n,
			})
		}
	}

	// episódios com a estatística dos minutos de dentro e o limite passado
	a.mu.Lock()
	alarmCPM, alarmOn := a.settings.AlarmCPM, a.settings.AlarmEnabled
	a.mu.Unlock()
	limitOf := func(level string) int {
		switch level {
		case "perigo":
			return lv.DangerCPM
		case "alarme":
			return alarmCPM
		}
		return lv.AttentionCPM
	}
	retroOf := map[int64]bool{}
	for _, e := range evs {
		retroOf[e.ID] = e.Retro
	}
	for _, ep := range alertEpisodes(evs, time.Now()) {
		st, en := time.UnixMilli(ep.Start), time.UnixMilli(ep.Until)
		re := reportEpisode{Start: st, End: en, Level: levelName(ep.Level), LevelCls: ep.Level,
			Minutes: int(math.Ceil(en.Sub(st).Minutes())), Peak: ep.Peak, Limit: limitOf(ep.Level)}
		if len(ep.IDs) > 0 {
			re.Retro = retroOf[ep.IDs[0]]
		}
		if re.Retro {
			re.Minutes++ // do histórico: de minuto a minuto, inclusive o último
		}
		in := pointsIn(pts, st.Truncate(time.Minute), en)
		if ms := calcMinuteStats(in); ms.N > 0 {
			re.N, re.MinC, re.MeanC, re.MaxC = ms.N, ms.Min, ms.Mean, ms.Max
		}
		if ep.Peak > r.AlertPeakMax {
			r.AlertPeakMax = ep.Peak
		}
		r.Episodes = append(r.Episodes, re)
	}
	r.Above = append(r.Above, reportAbove{"Atenção", lv.AttentionCPM, minutesAtOrAbove(pts, float64(lv.AttentionCPM))})
	if alarmOn {
		r.Above = append(r.Above, reportAbove{"Alarme", alarmCPM, minutesAtOrAbove(pts, float64(alarmCPM))})
	}
	r.Above = append(r.Above, reportAbove{"Perigo", lv.DangerCPM, minutesAtOrAbove(pts, float64(lv.DangerCPM))})

	// alertas por episódio (o mais recente primeiro)
	eps := alertEpisodes(evs, time.Now())
	for i := len(eps) - 1; i >= 0; i-- {
		ep := eps[i]
		st := time.UnixMilli(ep.Start)
		span := st.Format("15:04")
		if ep.Until > ep.Start {
			span += "–" + time.UnixMilli(ep.Until).Format("15:04") + " (" + fmtMinutes(int(math.Round(float64(ep.Until-ep.Start)/60000))) + ")"
		}
		pk := fmtBR(ep.Peak, 0) + " CPM"
		if s.BgCPM > 0 {
			pk += fmt.Sprintf(" (%sσ)", fmtBR((ep.Peak-s.BgCPM)/math.Sqrt(s.BgCPM), 1))
		}
		r.AlertList = append(r.AlertList, reportAlert{When: st.Format("02/01"), Level: levelName(ep.Level), LevelCls: ep.Level, Span: span, Peak: pk})
	}

	for _, m := range meas {
		sign := "+"
		if m.Result.NetCPM < 0 {
			sign = ""
		}
		res := fmt.Sprintf("%s%s ± %s CPM", sign, fmtBR(m.Result.NetCPM, 1), fmtBR(m.Result.NetErr, 1))
		if m.Result.NetUSv != 0 {
			res += fmt.Sprintf(" (≈ %s µSv/h)", fmtBR(m.Result.NetUSv, 3))
		}
		verdict := m.Result.Verdict
		if i := strings.IndexAny(verdict, ":(—"); i > 0 {
			verdict = strings.TrimSpace(verdict[:i])
		}
		r.Meas = append(r.Meas, reportMeas{When: time.UnixMilli(m.Time).Format("02/01 15:04"), Name: m.Name, Result: res, Verdict: verdict})
	}

	r.Cal = fmtBR(cal.USv(100)/100, 4) + " µSv/h por CPM"
	if lv.DeadTimeOn {
		r.DeadTime = fmt.Sprintf("ligada (τ = %s µs)", fmtBR(lv.DeadTimeUs, 0))
	} else {
		r.DeadTime = "desligada"
	}

	// configurações e método
	calSrc := "lida do contador agora"
	switch cal.Source {
	case calFromLast:
		calSrc = "última lida do contador; ele não estava conectado"
	case "padrão":
		calSrc = "padrão de fábrica do GMC-800; o app ainda não leu a do contador"
	}
	alarm := "desligado"
	if alarmOn {
		alarm = fmt.Sprintf("a partir de %d CPM", alarmCPM)
	}
	bg := "ainda sem fundo"
	if s.BgOK && s.BgCPM > 0 {
		bg = fmt.Sprintf("%s CPM (≈ %s µSv/h), mediana das leituras por minuto dos últimos 30 dias em casa", fmtBR(s.BgCPM, 1), fmtBR(cal.USv(s.BgCPM), 3))
	}
	dev := r.Device
	if dev == "" {
		dev = "contador não conectado na hora do relatório"
	}
	zone, _ := time.Now().Zone()
	r.Config = [][2]string{
		{"Contador", dev},
		{"Calibração", r.Cal + " (" + calSrc + ")"},
		{"Tempo morto", r.DeadTime},
		{"Fundo de referência", bg},
		{"Aviso de atenção", fmt.Sprintf("a partir de %d CPM (limite operacional configurado)", lv.AttentionCPM)},
		{"Alarme", alarm + " (limite operacional configurado)"},
		{"Alarme de perigo", fmt.Sprintf("a partir de %d CPM (limite operacional configurado)", lv.DangerCPM)},
		{"Dose", "calculada pelo app: soma, minuto a minuto, de CPM × calibração ÷ 60. Não é o acumulador interno do contador."},
		{"σ (Poisson)", "√média: a variação que o acaso sozinho dá numa contagem de 1 minuto"},
		{"Percentis", "interpolação linear entre as leituras ordenadas"},
		{"Lacuna", "minuto sem leitura (cabo fora, app fechado sem histórico baixado); nunca conta como zero"},
		{"Horários", "hora local (" + zone + ")"},
		{"Programa", "GMC Monitor " + AppVersion},
	}
	return r, nil
}

// defaultReportDir é Documentos/GMC Monitor/Relatórios (no Linux, a pasta
// Documentos ou Documents que existir; senão a pasta pessoal).
func defaultReportDir() string {
	home, err := os.UserHomeDir()
	if err != nil || home == "" {
		return ""
	}
	docs := home
	for _, n := range []string{"Documents", "Documentos"} {
		if st, err := os.Stat(filepath.Join(home, n)); err == nil && st.IsDir() {
			docs = filepath.Join(home, n)
			break
		}
	}
	return filepath.Join(docs, "GMC Monitor", "Relatórios")
}

// ReportDirInfo é o que as Configurações mostram.
type ReportDirInfo struct {
	Dir     string `json:"dir"`
	Default bool   `json:"default"` // usando a pasta padrão
}

func (a *App) reportDir() ReportDirInfo {
	a.mu.Lock()
	d := a.settings.ReportDir
	a.mu.Unlock()
	if d == "" {
		return ReportDirInfo{Dir: defaultReportDir(), Default: true}
	}
	return ReportDirInfo{Dir: d}
}

// GetReportDir devolve a pasta onde os relatórios são salvos.
func (a *App) GetReportDir() ReportDirInfo { return a.reportDir() }

// ChooseReportDir abre o seletor de pasta. Cancelar não muda nada.
func (a *App) ChooseReportDir() (ReportDirInfo, error) {
	if a.ctx == nil {
		return a.reportDir(), errors.New("sem janela")
	}
	cur := a.reportDir().Dir
	_ = os.MkdirAll(cur, 0o755)
	dir, err := wruntime.OpenDirectoryDialog(a.ctx, wruntime.OpenDialogOptions{Title: "Pasta dos relatórios", DefaultDirectory: cur, CanCreateDirectories: true})
	if err != nil || dir == "" {
		return a.reportDir(), err
	}
	a.mu.Lock()
	a.settings.ReportDir = dir
	a.saveSettingsLocked()
	a.mu.Unlock()
	a.logf("relatórios: pasta trocada para %s", dir)
	return a.reportDir(), nil
}

// ResetReportDir volta para a pasta padrão.
func (a *App) ResetReportDir() ReportDirInfo {
	a.mu.Lock()
	a.settings.ReportDir = ""
	a.saveSettingsLocked()
	a.mu.Unlock()
	return a.reportDir()
}

// OpenReportDir abre a pasta dos relatórios.
func (a *App) OpenReportDir() error {
	d := a.reportDir().Dir
	if err := os.MkdirAll(d, 0o755); err != nil {
		return err
	}
	return openFolder(d)
}

// uniquePath não sobrescreve: nome.pdf, nome-2.pdf, nome-3.pdf...
func uniquePath(p string) string {
	if _, err := os.Stat(p); err != nil {
		return p
	}
	ext := filepath.Ext(p)
	base := strings.TrimSuffix(p, ext)
	for i := 2; ; i++ {
		q := fmt.Sprintf("%s-%d%s", base, i, ext)
		if _, err := os.Stat(q); err != nil {
			return q
		}
	}
}

// ExportReport salva o relatório (PDF) do período na pasta dos relatórios e
// abre no leitor padrão. Devolve o caminho completo.
func (a *App) ExportReport(fromMs, toMs int64, label string) (string, error) {
	dir := a.reportDir().Dir
	if dir == "" {
		return "", errors.New("não achei uma pasta para salvar (escolha uma nas Configurações → Relatórios)")
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", fmt.Errorf("não consegui criar a pasta %s: %w", dir, err)
	}
	slug := strings.Trim(badFileChars.ReplaceAllString(strings.ReplaceAll(strings.ToLower(label), " ", "-"), ""), "-")
	if slug == "" {
		slug = "periodo"
	}
	from, to := time.UnixMilli(fromMs), time.UnixMilli(toMs)
	span := from.Format("2006-01-02")
	if to.Sub(from) < 26*time.Hour && from.YearDay() == to.Add(-time.Millisecond).YearDay() {
		if to.Sub(from) < 23*time.Hour {
			span += "_" + from.Format("15h04") + "-" + to.Format("15h04")
		}
	} else {
		span += "_a_" + to.Add(-time.Millisecond).Format("2006-01-02")
	}
	dst := uniquePath(filepath.Join(dir, fmt.Sprintf("gmc-relatorio-%s-%s.pdf", slug, span)))
	r, err := a.buildReport(fromMs, toMs, label)
	if err != nil {
		return "", err
	}
	if err := os.WriteFile(dst, renderReportPDF(r), 0o644); err != nil {
		return "", err
	}
	a.logf("relatório salvo: %s", dst)
	_ = openFolder(dst) // abre no leitor de PDF padrão
	return dst, nil
}
