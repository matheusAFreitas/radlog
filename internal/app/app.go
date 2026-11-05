package app

import (
	"context"
	"encoding/csv"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	wruntime "github.com/wailsapp/wails/v2/pkg/runtime"

	"radlog/internal/gmc"
	"radlog/internal/store"
)

// Faixas de nível do manual do GMC-800 (nomes "atenção"/"alerta" do medidor
// na tela). Os níveis que disparam avisos estão em levels.go.
const (
	manualAttnCPM = 50
	warningCPM    = 100
)

// dangerStep atualiza o estado de perigo (não depende do alarme do usuário;
// só dá para silenciar o som por um tempo, na tela).
func dangerStep(on *bool, cpm float64, lv Levels) (started, ended bool) {
	switch {
	case cpm >= lv.dangerEnter() && !*on:
		*on = true
		return true, false
	case cpm < lv.dangerExit() && *on:
		*on = false
		return false, true
	}
	return false, false
}

// Settings são as preferências salvas em config.json.
type Settings struct {
	Port         string `json:"port"`
	AutoConnect  bool   `json:"autoConnect"`
	AlarmEnabled bool   `json:"alarmEnabled"`
	AlarmCPM     int    `json:"alarmCPM"`
	AlarmSound   string `json:"alarmSound"` // sirene, bipes, mudo
	AlarmRaise   bool   `json:"alarmRaise"` // trazer a janela para a frente
	// Aviso de atenção a partir de 50 CPM (faixa do manual), mais leve que o alarme.
	AttentionEnabled bool `json:"attentionEnabled"`
	// Chuva no gráfico (Open-Meteo), pelo local escolhido.
	RainEnabled bool    `json:"rainEnabled"`
	RainLat     float64 `json:"rainLat"`
	RainLon     float64 `json:"rainLon"`
	RainPlace   string  `json:"rainPlace"`
	RainHide    bool    `json:"rainHidePlace"` // não mostrar a cidade na tela
	// níveis do app (levels.go)
	Levels Levels `json:"levels"`
	// ao baixar o histórico, pergunta se trechos novos a partir disto (min)
	// foram fora de casa (periods.go)
	AwayAskMin int `json:"awayAskMin"`
	// fechar a janela deixa o app rodando na bandeja (tray.go)
	TrayOnClose bool `json:"trayOnClose"`
	// iniciar com o Windows, direto na bandeja
	StartWithWindows bool `json:"startWithWindows"`
	// atualização pelo GitHub (update.go): procura ao abrir e uma vez por dia
	NoUpdateCheck bool   `json:"noUpdateCheck"`
	SkipVersion   string `json:"skipVersion"` // "Pular esta versão"
	// pasta dos relatórios em PDF; vazio = Documentos/GMC Monitor/Relatórios
	// (report.go; só muda por ChooseReportDir)
	ReportDir string `json:"reportDir"`
}

func defaultSettings() Settings {
	return Settings{AutoConnect: true, AlarmEnabled: true, AlarmCPM: warningCPM, AlarmSound: "sirene", AlarmRaise: true, AttentionEnabled: true, Levels: DefaultLevels(), AwayAskMin: awayAskDefault, TrayOnClose: true}
}

// Reading é a leitura ao vivo mandada para a tela a cada segundo.
type Reading struct {
	Time int64  `json:"time"` // ms desde 1970
	CPM  uint32 `json:"cpm"`  // já com a compensação de tempo morto, se ligada
	CPS  uint32 `json:"cps"`
	// Compensação de tempo morto: o CPM que o contador mandou e quanto a
	// correção somou (%). Zeros quando desligada ou sem efeito.
	RawCPM uint32  `json:"rawCPM,omitempty"`
	DTPct  float64 `json:"dtPct,omitempty"`
	USv    float64 `json:"usv"`
	MRh    float64 `json:"mrh"`   // mrem/h (1 mrem = 10 µSv, exato)
	Level  string  `json:"level"` // seguro, atencao, alerta, perigo
	Alarm  bool    `json:"alarm"`
	// Perigo: passou de 1000 CPM. Vale mesmo com o alarme desligado.
	Danger bool `json:"danger"`
	// Geiger saturado (saturation.go): "limite" ou "queda".
	Saturated bool `json:"saturated"`
	// Sem contagens: CPM em 0 há 2 min ou mais (nocount.go).
	NoCount      bool      `json:"noCount"`
	NoCountSince int64     `json:"noCountSince,omitempty"`
	SatReason    string    `json:"satReason,omitempty"`
	Rise         *RiseInfo `json:"rise,omitempty"` // como subiu até o perigo
	Demo         bool      `json:"demo"`           // valor simulado (modo demo)
	// Atenção: passou de 50 CPM (mesmo que ainda abaixo do alarme).
	Attention bool `json:"attention"`
	// Fundo (mediana dos últimos 30 dias) e quantos desvios de Poisson a
	// leitura está acima dele. Sigma vem nulo enquanto não houver fundo.
	BgCPM float64  `json:"bgCPM"`
	Sigma *float64 `json:"sigma"`
}

// attentionStep atualiza o estado de atenção e diz se um episódio começou.
// A saída fica um pouco abaixo da entrada, para não ficar entrando e saindo
// quando o valor oscila perto do nível.
func attentionStep(on *bool, enabled bool, cpm float64, lv Levels) (started bool) {
	switch {
	case !enabled:
		*on = false
	case cpm >= lv.attnEnter() && !*on:
		*on = true
		return true
	case cpm < lv.attnExit():
		*on = false
	}
	return false
}

// DeviceInfo descreve o contador conectado.
type DeviceInfo struct {
	Port        string          `json:"port"`
	Version     string          `json:"version"`
	Serial      string          `json:"serial"`
	ClockDrift  float64         `json:"clockDrift"` // segundos (contador - PC)
	Calibration gmc.Calibration `json:"calibration"`
	USvPerCPM   float64         `json:"usvPerCPM"`
}

// Status é o estado da conexão.
type Status struct {
	State       string     `json:"state"` // desconectado, conectando, conectado, erro
	Message     string     `json:"message"`
	Device      DeviceInfo `json:"device"`
	Downloading bool       `json:"downloading"`
	DataDir     string     `json:"dataDir"`
	// fator do fundo (µSv/h por CPM) em uso agora, mesmo desconectado
	USvPerCPM float64 `json:"usvPerCPM"`
	CalSource string  `json:"calSource"`
}

// Event é uma linha do registro de eventos.
type Event struct {
	ID   int64  `json:"id"`
	Time int64  `json:"time"`
	Text string `json:"text"`
	Kind string `json:"kind"` // "evento" (conexão, histórico...) ou "alerta" (radiação)

	// só nos alertas (alerts.go)
	Level  string  `json:"level,omitempty"`  // "atencao" ou "alarme"
	CPM    float64 `json:"cpm,omitempty"`    // valor que disparou
	Peak   float64 `json:"peak,omitempty"`   // maior valor no episódio
	PeakAt int64   `json:"peakAt,omitempty"` // quando foi o pico (ms)
	End    int64   `json:"end,omitempty"`    // fim (ms); 0 = ainda ativo ou não se sabe
	Retro  bool    `json:"retro,omitempty"`  // achado depois, no histórico do contador
	Active bool    `json:"active,omitempty"` // acontecendo agora
	Demo   bool    `json:"demo,omitempty"`
}

const (
	kindEvent = "evento"
	kindAlert = "alerta"
	maxEvents = 5000
)

// Chart são os pontos do gráfico. Valores nulos são buracos sem dados.
type Chart struct {
	From   int64      `json:"from"`
	To     int64      `json:"to"`
	StepMs int64      `json:"stepMs"`
	Times  []int64    `json:"times"`
	Values []*float64 `json:"values"`
	Avg    *float64   `json:"avg"`
	Min    *float64   `json:"min"`
	Max    *float64   `json:"max"`

	// Os mesmos valores em µSv/h, pela calibração atual.
	USv          []*float64 `json:"usv"`
	AvgUSv       *float64   `json:"avgUSv"`
	MinUSv       *float64   `json:"minUSv"`
	MaxUSv       *float64   `json:"maxUSv"`
	AttentionCPM int        `json:"attentionCPM"`
	AttentionUSv float64    `json:"attentionUSv"` // nível de atenção em µSv/h
	// limites dos alarmes, para as linhas do gráfico durante o alarme
	AlarmCPM  int     `json:"alarmCPM"`
	AlarmUSv  float64 `json:"alarmUSv"`
	DangerCPM int     `json:"dangerCPM"`
	DangerUSv float64 `json:"dangerUSv"`
	// picos dos alertas no período (leitura por segundo), para o ponto no
	// gráfico; o "máx" do período já os inclui
	Peaks []ChartPeak `json:"peaks"`
	// trecho antigo com passo menor que 1 min: o detalhe por segundo só
	// existe perto dos alertas (o resto é o valor do minuto)
	FineAlertsOnly bool `json:"fineAlertsOnly"`
}

// withUSv preenche as séries em µSv/h.
func (a *App) withUSv(c Chart) Chart {
	a.mu.Lock()
	cal := a.convCalLocked()
	alarmCPM := a.settings.AlarmCPM
	lv := a.settings.Levels.norm()
	a.mu.Unlock()
	if alarmCPM <= 0 {
		alarmCPM = warningCPM
	}
	c.AlarmCPM, c.AlarmUSv = alarmCPM, cal.USv(float64(alarmCPM))
	c.DangerCPM, c.DangerUSv = lv.DangerCPM, cal.USv(lv.dangerEnter())
	c.AttentionCPM, c.AttentionUSv = lv.AttentionCPM, cal.USv(lv.attnEnter())
	conv := func(v *float64) *float64 {
		if v == nil {
			return nil
		}
		u := cal.USv(*v)
		return &u
	}
	c.USv = make([]*float64, len(c.Values))
	for i, v := range c.Values {
		c.USv[i] = conv(v)
	}
	if c.Values != nil {
		var sum float64
		var n int
		for _, u := range c.USv {
			if u != nil {
				sum += *u
				n++
			}
		}
		if n > 0 {
			avg := sum / float64(n)
			c.AvgUSv = &avg
		}
	}
	c.Peaks = a.alertPeaks(time.UnixMilli(c.From), time.UnixMilli(c.To))
	for _, p := range c.Peaks {
		if c.Max != nil && p.CPM > *c.Max {
			v := p.CPM
			c.Max = &v
		}
	}
	c.MinUSv, c.MaxUSv = conv(c.Min), conv(c.Max)
	return c
}

// Stats são os números dos cartões.
type Stats struct {
	Avg1h        *float64 `json:"avg1h"`
	Min24h       *float64 `json:"min24h"`
	Max24h       *float64 `json:"max24h"`
	Max24hAt     int64    `json:"max24hAt"`
	DoseToday    float64  `json:"doseToday"` // µSv
	MinutesToday int      `json:"minutesToday"`
	BgCPM        float64  `json:"bgCPM"` // fundo (mediana de 30 dias), 0 = ainda sem
}

// DayStat é uma linha da tabela de fundo diário.
type DayStat struct {
	Day     int64   `json:"day"`
	CPM     float64 `json:"cpm"`
	USv     float64 `json:"usv"`
	Minutes int     `json:"minutes"`
}

// ImportResult é o resumo de uma leitura de histórico.
type ImportResult struct {
	Bytes   int `json:"bytes"`
	Records int `json:"records"`
	Added   int `json:"added"`
	Alerts  int `json:"alerts"` // alertas achados nos minutos novos
	// trechos novos (que o app não tinha visto), para perguntar se foram
	// fora de casa
	NewRanges []NewRange `json:"newRanges"`
	File      string     `json:"file"`
}

// App é o que a interface enxerga (os métodos exportados viram funções
// em window.go.main.App).
type App struct {
	ctx     context.Context
	dataDir string
	store   *store.Store

	mu       sync.Mutex
	settings Settings
	dev      *gmc.Device
	info     DeviceInfo
	cal      gmc.Calibration
	state    string
	message  string
	stopPoll chan struct{}
	events   []Event
	evSeq    int64    // último ID de evento
	alerts   alertIDs // alertas reais abertos agora

	connMu sync.Mutex // uma conexão por vez

	downloading    atomic.Bool
	cfgBusy        atomic.Bool // gravando/lendo a configuração do contador
	cfgBackedUp    bool        // já salvou a cópia da config nesta conexão
	cfgSnapshot    []byte      // "foto" da config para comparar
	cancelDownload chan struct{}

	// pausa a reconexão automática quando o usuário desconecta na mão
	manualOff bool
	retryStop chan struct{}
	closing   atomic.Bool // janela fechando (watchdog.go)
	// bandeja (tray.go)
	inTray    atomic.Bool
	quitting  atomic.Bool
	trayReady atomic.Bool
	trayMu    sync.Mutex
	trayTip   string
	trayLevel string
	notes     []Note // anotações no gráfico (notes.go)

	alarmOn  bool
	attnOn   bool
	dangerOn bool
	sat      satState
	noCount  noCountState
	interf   interferenceDetector
	upd      updater

	live *ring    // última hora de leituras reais por segundo
	demo demoData // modo demo

	bgCPM float64 // fundo em cache (stats.go)
	bgOK  bool
	bgAt  time.Time

	measure      MeasureState // medição de objeto (measure.go)
	rain         *rainCache   // chuva (weather.go)
	rainSt       rainState
	radar        *radarStore     // chuva pelo radar (radar.go)
	radarKick    chan struct{}   // busca o radar já (lugar novo)
	periods      []Period        // fora de casa (periods.go)
	measurements []SavedMeasure  // medições de objeto salvas (measure.go)
	measSamples  []MeasureSample // segundos da medição em andamento
}

// NewApp cria o App.
func NewApp() *App {
	return &App{state: "desconectado", cal: gmc.DefaultCalibration(), live: newRing(), radarKick: make(chan struct{}, 1)}
}

func (a *App) startup(ctx context.Context) {
	a.ctx = ctx
	base, err := os.UserConfigDir()
	if err != nil || base == "" {
		base, _ = os.Getwd()
	}
	a.dataDir = filepath.Join(base, "GMC Monitor")
	_ = os.MkdirAll(a.dataDir, 0o755)
	a.logStart()

	a.settings = a.loadSettings()
	a.loadLastCal()
	a.loadRain()
	a.loadPeriods()
	a.loadNotes()
	a.loadMeasurements()
	a.loadRadar()
	go a.radarLoop(nil)
	a.startTray()
	a.inTray.Store(startedInTray())
	if a.settings.StartWithWindows {
		a.syncAutostart(true) // atualiza o caminho se o .exe mudou (versão nova)
	}
	if startedUpdating() {
		a.logf("aberta pela versão anterior (troca de versão)")
	}
	a.logAutostart()
	a.events = a.loadEvents()

	st, err := store.Open(a.dataDir)
	if err != nil {
		a.logEvent("Erro ao abrir os dados: " + err.Error())
		st, _ = store.Open(filepath.Join(os.TempDir(), "GMC Monitor"))
	}
	a.store = st
	go a.scanRetroOnce()
	go a.updateLoop()

	if a.settings.AutoConnect && a.settings.Port != "" {
		go a.autoReconnect(0)
	}
	if runtime.GOOS == "windows" {
		// espera a versão anterior fechar de vez (troca de versão)
		time.AfterFunc(30*time.Second, a.cleanOldWebviews)
	}
}

func (a *App) shutdown(ctx context.Context) {
	a.logf("==== fechando o app")
	a.beforeClose()
	a.stopTray()
	a.mu.Lock()
	if a.retryStop != nil {
		close(a.retryStop)
		a.retryStop = nil
	}
	a.mu.Unlock()
	a.StopDemo()
	a.disconnect("")
}

// ---------- configurações e eventos em disco ----------

func (a *App) settingsPath() string { return filepath.Join(a.dataDir, "config.json") }

func (a *App) loadSettings() Settings {
	s := defaultSettings()
	b, err := os.ReadFile(a.settingsPath())
	if err == nil {
		_ = json.Unmarshal(b, &s)
	}
	if s.AlarmCPM <= 0 {
		s.AlarmCPM = warningCPM
	}
	if s.AlarmSound == "" {
		s.AlarmSound = "sirene"
	}
	s.Levels = s.Levels.norm()
	s.AwayAskMin = normAwayAsk(s.AwayAskMin)
	return s
}

func (a *App) saveSettingsLocked() {
	b, _ := json.MarshalIndent(a.settings, "", "  ")
	_ = os.WriteFile(a.settingsPath(), b, 0o644)
}

func (a *App) eventsPath() string { return filepath.Join(a.dataDir, "eventos.csv") }

func (a *App) loadEvents() []Event {
	f, err := os.Open(a.eventsPath())
	if err != nil {
		return nil
	}
	defer f.Close()
	r := csv.NewReader(f)
	r.FieldsPerRecord = -1
	var out []Event
	for {
		rec, err := r.Read()
		if err == io.EOF {
			break
		}
		if err != nil {
			if _, ok := err.(*csv.ParseError); ok {
				continue
			}
			break
		}
		if ev, ok := parseEventRow(rec); ok {
			out = append(out, ev)
		}
	}
	sort.SliceStable(out, func(i, j int) bool { return out[i].Time < out[j].Time })
	if len(out) > maxEvents {
		out = out[len(out)-maxEvents:]
	}
	for i := range out {
		a.evSeq++
		out[i].ID = a.evSeq
	}
	return out
}

// logEvent registra um evento (conexão, histórico, configuração...).
func (a *App) logEvent(text string) { a.logEntry(text, kindEvent, true) }

// logAlert registra um alerta de radiação (atenção ou alarme).
func (a *App) logAlert(text string) {
	level := "atencao"
	if strings.HasPrefix(text, "Alarme") {
		level = "alarme"
	}
	a.addEvent(Event{Time: time.Now().UnixMilli(), Text: text, Kind: kindAlert, Level: level}, true)
}

func (a *App) logEntry(text, kind string, persist bool) {
	if persist {
		a.logf("evento: %s", text)
	}
	a.addEvent(Event{Time: time.Now().UnixMilli(), Text: text, Kind: kind}, persist)
}

func (a *App) emit(name string, data any) {
	if a.ctx != nil {
		wruntime.EventsEmit(a.ctx, name, data)
	}
}

// ---------- métodos chamados pela interface ----------

// GetSettings devolve as preferências.
func (a *App) GetSettings() Settings {
	a.mu.Lock()
	defer a.mu.Unlock()
	return a.settings
}

// SaveSettings grava as preferências.
func (a *App) SaveSettings(s Settings) Settings {
	if s.AlarmCPM <= 0 {
		s.AlarmCPM = warningCPM
	}
	switch s.AlarmSound {
	case "sirene", "bipes", "mudo":
	default:
		s.AlarmSound = "sirene"
	}
	s.Levels = s.Levels.norm()
	s.AwayAskMin = normAwayAsk(s.AwayAskMin)
	a.mu.Lock()
	s.Port = a.settings.Port // a porta é salva ao conectar
	s.ReportDir = a.settings.ReportDir
	s.SkipVersion = a.settings.SkipVersion // muda só por SkipUpdate
	old := a.settings
	autostartChanged := old.StartWithWindows != s.StartWithWindows
	if old.RainLat != s.RainLat || old.RainLon != s.RainLon || old.RainEnabled != s.RainEnabled {
		a.rainSt.err = "" // lugar novo (ou religou): tenta já
		a.kickRadarLocked()
	}
	a.settings = s
	a.saveSettingsLocked()
	a.mu.Unlock()
	if autostartChanged {
		a.syncAutostart(s.StartWithWindows)
		if s.StartWithWindows && startupDisabled() {
			// ligou no app: tira a trava do Gerenciador de Tarefas também
			if err := enableStartup(); err != nil {
				a.logEvent("Não consegui reativar o início no Windows: " + err.Error())
			}
		}
		a.logAutostart()
	}
	a.emitRain()
	return s
}

// GetStatus devolve o estado atual.
func (a *App) GetStatus() Status {
	a.mu.Lock()
	defer a.mu.Unlock()
	return a.statusLocked()
}

func (a *App) statusLocked() Status {
	return Status{
		State: a.state, Message: a.message, Device: a.info,
		Downloading: a.downloading.Load(), DataDir: a.dataDir,
		USvPerCPM: a.cal.USv(100) / 100, CalSource: a.cal.Source,
	}
}

func (a *App) setState(state, msg string) {
	a.mu.Lock()
	a.state, a.message = state, msg
	st := a.statusLocked()
	a.mu.Unlock()
	a.emit("status", st)
}

// GetEvents devolve os eventos mais recentes, do mais novo para o mais velho.
// kind: "evento", "alerta" ou "" para todos.
func (a *App) GetEvents(kind string) []Event {
	a.mu.Lock()
	defer a.mu.Unlock()
	out := []Event{}
	for i := len(a.events) - 1; i >= 0; i-- {
		if e := a.events[i]; kind == "" || e.Kind == kind {
			e.Active = a.isActiveLocked(e.ID)
			out = append(out, e)
		}
	}
	return out
}

// ListPorts lista as portas seriais.
func (a *App) ListPorts() ([]gmc.PortInfo, error) {
	return gmc.ListPorts()
}

// DetectPort procura o contador nas portas USB e devolve a primeira que
// responde como um GMC.
func (a *App) DetectPort() (string, error) {
	ports, err := gmc.ListPorts()
	if err != nil {
		return "", err
	}
	a.mu.Lock()
	connected := a.dev != nil
	current := a.info.Port
	a.mu.Unlock()
	if connected {
		return current, nil
	}
	for _, p := range ports {
		if !p.IsUSB {
			continue
		}
		d, err := gmc.Open(p.Name)
		if err != nil {
			continue
		}
		v, err := d.Version()
		d.Close()
		if err == nil && strings.HasPrefix(v, "GMC") {
			return p.Name, nil
		}
	}
	return "", errors.New("nenhum contador GMC encontrado nas portas USB")
}

// Connect conecta ao contador na porta indicada.
func (a *App) Connect(port string) (DeviceInfo, error) {
	a.mu.Lock()
	a.manualOff = false
	if a.retryStop != nil {
		close(a.retryStop)
		a.retryStop = nil
	}
	a.mu.Unlock()
	info, err := a.connect(port)
	if err != nil {
		a.setState("erro", err.Error())
		return DeviceInfo{}, err
	}
	return info, nil
}

func (a *App) connect(port string) (DeviceInfo, error) {
	a.connMu.Lock()
	defer a.connMu.Unlock()
	a.disconnect("")
	a.setState("conectando", "Conectando em "+port+"…")
	a.logf("conexão: abrindo %s", port)

	// abrir e conversar com prazo: uma porta presa no driver travava a
	// reconexão para sempre
	d, err := withTimeout(8*time.Second, func() (*gmc.Device, error) { return gmc.Open(port) })
	if err != nil {
		a.logf("conexão: não abriu %s: %v", port, err)
		if errors.Is(err, errStuck) {
			a.logPorts("abrir travou")
			return DeviceInfo{}, fmt.Errorf("%s não abriu (a porta parece presa): tire e coloque o cabo USB", port)
		}
		return DeviceInfo{}, err
	}
	ver, err := withTimeout(6*time.Second, d.Version)
	if err != nil {
		a.logf("conexão: %s não respondeu ao GETVER: %v", port, err)
		d.Close()
		if errors.Is(err, errStuck) {
			return DeviceInfo{}, fmt.Errorf("%s abriu mas não respondeu (porta presa): tire e coloque o cabo USB", port)
		}
		return DeviceInfo{}, fmt.Errorf("%s não respondeu como um contador GMC (%v)", port, err)
	}
	if !strings.HasPrefix(ver, "GMC") {
		d.Close()
		return DeviceInfo{}, fmt.Errorf("%s respondeu %q, não parece um contador GMC", port, ver)
	}
	info := DeviceInfo{Port: port, Version: ver}
	info.Serial, _ = withTimeout(6*time.Second, d.Serial)

	cal := gmc.DefaultCalibration()
	if cfg, err := withTimeout(10*time.Second, d.Config); err == nil {
		if c, err := gmc.ParseCalibration800(cfg); err == nil {
			cal = c
		}
	}
	info.Calibration = cal
	if len(cal.Points) > 0 {
		info.USvPerCPM = cal.Points[0].USv / cal.Points[0].CPM
	}
	if dt, err := withTimeout(6*time.Second, d.DateTime); err == nil {
		info.ClockDrift = math.Round(dt.Sub(time.Now()).Seconds())
	}

	stop := make(chan struct{})
	a.mu.Lock()
	a.dev, a.info, a.cal, a.stopPoll = d, info, cal, stop
	a.cfgBackedUp = false
	a.settings.Port = port
	a.saveSettingsLocked()
	a.mu.Unlock()
	a.saveLastCal(cal)

	a.setState("conectado", "")
	a.logf("conexão: ok, %s em %s, série %s", ver, port, info.Serial)
	a.logEvent(fmt.Sprintf("Contador conectado (%s em %s)", ver, port))
	go a.poll(d, stop, port)
	return info, nil
}

// Disconnect desconecta e para a reconexão automática.
func (a *App) Disconnect() {
	a.mu.Lock()
	a.manualOff = true
	if a.retryStop != nil {
		close(a.retryStop)
		a.retryStop = nil
	}
	a.mu.Unlock()
	if a.disconnect("") {
		a.logEvent("Contador desconectado")
	}
}

// disconnect fecha a porta. Devolve true se havia algo conectado.
func (a *App) disconnect(errMsg string) bool {
	a.mu.Lock()
	d, stop := a.dev, a.stopPoll
	a.dev, a.stopPoll = nil, nil
	a.alarmOn = false
	a.attnOn = false
	a.dangerOn = false
	a.sat = satState{}
	a.noCount = noCountState{}
	a.mu.Unlock()
	a.closeAlerts(&a.alerts, time.Now(), false)
	a.trayShow("GMC Monitor · contador desconectado", "")
	if d == nil {
		return false
	}
	if stop != nil {
		close(stop)
	}
	if err := d.Close(); err != nil {
		a.logf("desconexão: fechar a porta falhou/travou: %v", err)
	} else {
		a.logf("desconexão: porta fechada (%s)", map[bool]string{true: errMsg, false: "normal"}[errMsg != ""])
	}
	if errMsg != "" {
		a.setState("erro", errMsg)
	} else {
		a.setState("desconectado", "")
	}
	return true
}

// autoReconnect tenta conectar na porta salva a cada 10 s até conseguir.
func (a *App) autoReconnect(wait time.Duration) {
	a.mu.Lock()
	if a.retryStop != nil || a.manualOff || a.settings.Port == "" {
		a.mu.Unlock()
		return
	}
	stop := make(chan struct{})
	a.retryStop = stop
	port := a.settings.Port
	a.mu.Unlock()

	// logo depois de fechar, o Windows ainda segura a porta por um instante
	// (dava "em uso por outro programa" na primeira tentativa)
	select {
	case <-stop:
		return
	case <-time.After(wait):
	}
	for {
		select {
		case <-stop:
			return
		default:
		}
		if _, err := a.connect(port); err == nil {
			a.mu.Lock()
			if a.retryStop == stop {
				a.retryStop = nil
			}
			a.mu.Unlock()
			return
		} else if errors.Is(err, gmc.ErrNotFound) {
			// cabo fora ou contador desligado: é o normal, não é erro
			a.logf("reconexão: %s não encontrada", port)
			a.setState("desconectado", port+" não encontrada")
		} else {
			a.logf("reconexão: falhou: %v", err)
			a.setState("erro", err.Error())
		}
		select {
		case <-stop:
			return
		case <-time.After(10 * time.Second):
		}
	}
}

// ---------- leitura ao vivo ----------

func (a *App) poll(d *gmc.Device, stop chan struct{}, port string) {
	ticker := time.NewTicker(time.Second)
	defer ticker.Stop()
	fails := 0
	var minute time.Time // minuto da última amostra
	var lastCPM uint32
	var lastAt time.Time
	lastTick := time.Now()
	began := time.Now()
	reads, errs := 0, 0
	why := ""
	{
		a.mu.Lock()
		l := a.settings.Levels.norm()
		a.mu.Unlock()
		dt := "tempo morto desligado"
		if l.DeadTimeOn {
			dt = fmt.Sprintf("tempo morto ligado (τ = %.0f µs)", l.DeadTimeUs)
		}
		a.logf("leitura ao vivo: iniciada em %s (GETCPM + GETCPS a cada 1 s, prazo de %.0f s por comando, %s)", port, pollTimeout.Seconds(), dt)
	}
	defer func() {
		if why == "" {
			why = "desconectado pelo app (botão Desconectar ou troca de porta)"
			if a.closing.Load() {
				why = "o app está fechando"
			}
		}
		a.logf("leitura ao vivo: encerrada em %s após %s (%d leituras, %d erros) — motivo: %s",
			port, fmtMinutes(int(time.Since(began).Minutes())), reads, errs, why)
	}()

	for {
		select {
		case <-stop:
			return
		case <-ticker.C:
		}
		// o laço roda a cada 1 s; um buraco grande quer dizer PC suspenso
		// (ou o Windows segurou o app)
		if gap := time.Since(lastTick); gap > 5*time.Second {
			a.logf("leitura ao vivo: laço parado por %.0f s (PC suspenso ou travado?)", gap.Seconds())
		}
		lastTick = time.Now()
		if a.downloading.Load() || a.cfgBusy.Load() {
			continue
		}
		t0 := time.Now()
		cpm, err := callTimeout(d.CPM, pollTimeout)
		cmd := "GETCPM"
		var cps uint32
		if err == nil {
			cmd = "GETCPS"
			cps, err = callTimeout(d.CPS, pollTimeout)
		}
		if took := time.Since(t0); took > time.Second && err == nil {
			a.logf("leitura ao vivo: lenta, %d ms", took.Milliseconds())
		}
		if errors.Is(err, errStuck) {
			// travou no driver: não adianta tentar de novo na mesma porta
			a.logf("leitura ao vivo: %s sem resposta em %.0f s (porta travada)", cmd, pollTimeout.Seconds())
			a.logPorts("na hora que travou")
			a.dumpDiag("leitura ao vivo sem resposta em 6 s (" + cmd + ")")
			a.logEvent("O contador parou de responder (a porta travou); reconectando")
			why = fmt.Sprintf("%s sem resposta em %.0f s: porta travada no driver", cmd, pollTimeout.Seconds())
			go a.lostConnection(err)
			return
		}
		if err != nil {
			fails++
			errs++
			hint := ""
			if fails == 1 {
				hint = serialErrHint(err)
			}
			a.logf("leitura ao vivo: erro %d/3 em %s: %v%s", fails, cmd, err, hint)
			if fails >= 3 {
				a.logPorts("depois de 3 erros")
				if gone := !portListed(port); gone {
					a.logf("leitura ao vivo: %s sumiu da lista de portas do Windows: cabo desconectado ou USB caiu", port)
					why = port + " sumiu da lista de portas (cabo ou USB)"
				} else {
					why = fmt.Sprintf("3 erros seguidos em %s (%v), com a porta ainda na lista", cmd, err)
				}
				go a.lostConnection(err)
				return
			}
			continue
		}
		reads++
		if fails > 0 {
			a.logf("leitura ao vivo: voltou a responder depois de %d erro(s)", fails)
		}
		fails = 0
		now := time.Now()

		// tempo morto: o CPM e o CPS corrigidos valem para tudo (gravação,
		// alertas, tela); a saturação olha o que o contador mandou (o fim da
		// escala é dele)
		rawCPM, rawCPS := cpm, cps
		a.mu.Lock()
		lvDT := a.settings.Levels.norm()
		a.mu.Unlock()
		cpm = uint32(math.Round(lvDT.dtCPM(float64(rawCPM))))
		cps = uint32(math.Round(lvDT.dtCPS(float64(rawCPS))))

		// Virou o minuto: grava o CPM medido no fim do minuto anterior
		// (o CPM do contador já é a soma dos últimos 60 s).
		m := now.Truncate(time.Minute)
		if !minute.IsZero() && m.After(minute) && lastAt.Sub(minute) >= 50*time.Second {
			a.storeMinute(minute, float64(lastCPM))
		}
		minute, lastCPM, lastAt = m, cpm, now

		a.mu.Lock()
		cal := a.convCalLocked()
		rawCal := a.cal // o que o contador mandou (sem correção): curva dele
		set := a.settings
		alarm := set.AlarmEnabled && int(cpm) >= set.AlarmCPM
		was := a.alarmOn
		started := alarm && !a.alarmOn
		if alarm {
			a.alarmOn = true
		} else if float64(cpm) < float64(set.AlarmCPM)*0.9 || !set.AlarmEnabled {
			a.alarmOn = false
		}
		ended := was && !a.alarmOn
		alarmOn := a.alarmOn && set.AlarmEnabled
		attnWas := a.attnOn
		lv := set.Levels.norm()
		attnStarted := attentionStep(&a.attnOn, set.AttentionEnabled, float64(cpm), lv)
		attnOn := a.attnOn
		satOn, satStarted := a.sat.step(now, rawCal.USv(float64(rawCPM)), rawCal.USv(float64(rawCPS)*60), lv)
		dCPM := float64(cpm)
		if satOn {
			dCPM = math.Max(dCPM, lv.dangerEnter()) // saturado: o perigo continua valendo
		}
		dangerStarted, dangerEnded := dangerStep(&a.dangerOn, dCPM, lv)
		dangerOn := a.dangerOn
		if dangerStarted {
			a.sat.rise = a.sat.riseFrom(now, cal.USv(lv.attnEnter()))
		}
		if !dangerOn {
			a.sat.rise = nil
		}
		satReason, rise := a.sat.reason, a.sat.rise
		zeroSince := a.noCount.since
		zeroStarted, zeroEnded := a.noCount.step(now, cpm, satOn)
		noCount, noCountSince := a.noCount.on, a.noCount.since
		a.mu.Unlock()
		if satStarted {
			a.logEvent(satText(satReason, lv))
		}
		if !satOn {
			a.checkInterference(now, float64(rawCPS))
		}
		if zeroStarted {
			a.logEvent("Sem contagens: 0 CPM há 2 min. O tubo pode estar com defeito; confira o contador")
		}
		if zeroEnded {
			a.logEvent(noCountText(zeroSince, now))
		}
		a.stepAlerts(&a.alerts, alertStep{
			Now: now, CPM: float64(cpm), AlarmCPM: set.AlarmCPM, Lv: lv,
			AttnStarted: attnStarted, AttnEnded: attnWas && !attnOn, AttnOn: attnOn,
			AlarmStarted: started, AlarmEnded: ended, AlarmOn: alarmOn,
			DangerStarted: dangerStarted, DangerEnded: dangerEnded,
		})
		if dangerStarted {
			a.dangerStarted(float64(cpm), false)
		} else if started {
			a.alarmStarted(float64(cpm), false)
		}
		if dangerEnded && alarmOn {
			// saiu do perigo mas continua no alarme do usuário: só o título
			a.flagWindow("ALARME", float64(cpm), false, false)
		}
		if ended || dangerEnded {
			a.alarmEnded()
		}

		usv := cal.USv(float64(cpm))
		r := Reading{
			Time: now.UnixMilli(), CPM: cpm, CPS: cps,
			USv: usv, MRh: usv / 10, Level: level(float64(cpm), lv), Alarm: alarmOn,
			Attention: attnOn, Danger: dangerOn,
			Saturated: satOn, SatReason: satReason, Rise: rise,
			NoCount: noCount,
		}
		if cpm != rawCPM {
			r.RawCPM = rawCPM
			r.DTPct = (float64(cpm)/math.Max(1, float64(rawCPM)) - 1) * 100
		}
		if noCount {
			r.NoCountSince = noCountSince.UnixMilli()
		}
		a.fillSigma(&r)
		trayLvl := ""
		switch {
		case dangerOn:
			trayLvl = "perigo"
		case alarmOn:
			trayLvl = "alarme"
		case attnOn:
			trayLvl = "atencao"
		}
		if !a.isDemo() {
			a.trayTick(cpm, usv, trayLvl, noCount, false) // no demo, o ícone mostra o demo
		}
		a.mu.Lock()
		a.live.add(now, float64(cpm))
		demo := a.demo.Active
		a.mu.Unlock()
		if !demo {
			a.feedMeasure(float64(cpm), cps, false)
		}
		if demo {
			// no demo a tela mostra o valor simulado; o real vai à parte
			// (e o alarme real continua valendo)
			a.emit("realReading", r)
		} else {
			a.emit("reading", r)
		}
	}
}

func level(cpm float64, lv Levels) string {
	switch {
	case cpm >= lv.dangerEnter():
		return "perigo"
	case cpm >= warningCPM:
		return "alerta"
	case cpm >= manualAttnCPM:
		return "atencao"
	}
	return "seguro"
}

func (a *App) lostConnection(err error) {
	a.disconnect("Conexão perdida: " + err.Error())
	a.logEvent("Conexão com o contador perdida")
	a.mu.Lock()
	auto := a.settings.AutoConnect && !a.manualOff
	a.mu.Unlock()
	if auto {
		go a.autoReconnect(3 * time.Second)
	}
}

func (a *App) storeMinute(t time.Time, cpm float64) {
	if err := a.store.Add(store.Point{T: t, CPM: cpm, Src: store.SrcLive}); err != nil {
		a.logEvent("Erro ao gravar leitura: " + err.Error())
		return
	}
	a.emit("stored", t.UnixMilli())

}

// ---------- relógio ----------

// SyncClock acerta o relógio do contador com o do computador.
func (a *App) SyncClock() (float64, error) {
	a.mu.Lock()
	d := a.dev
	a.mu.Unlock()
	if d == nil {
		return 0, errors.New("contador desconectado")
	}
	if err := d.SetDateTime(time.Now()); err != nil {
		return 0, err
	}
	drift := 0.0
	if dt, err := d.DateTime(); err == nil {
		drift = math.Round(dt.Sub(time.Now()).Seconds())
	}
	a.mu.Lock()
	a.info.ClockDrift = drift
	st := a.statusLocked()
	a.mu.Unlock()
	a.emit("status", st)
	a.logEvent("Relógio do contador sincronizado com o PC")
	return drift, nil
}

// ---------- histórico ----------

// DownloadHistory lê a memória do contador, salva uma cópia bruta (.bin) e
// junta as leituras aos dados do app.
func (a *App) DownloadHistory() (ImportResult, error) {
	a.mu.Lock()
	d := a.dev
	a.mu.Unlock()
	if d == nil {
		return ImportResult{}, errors.New("contador desconectado")
	}
	if a.cfgBusy.Load() {
		return ImportResult{}, errors.New("espere terminar a gravação da configuração")
	}
	if !a.downloading.CompareAndSwap(false, true) {
		return ImportResult{}, errors.New("já estou baixando o histórico")
	}
	a.logf("histórico: começou a baixar")
	defer a.logf("histórico: terminou")
	cancel := make(chan struct{})
	a.mu.Lock()
	a.cancelDownload = cancel
	a.mu.Unlock()
	defer func() {
		a.downloading.Store(false)
		a.mu.Lock()
		a.cancelDownload = nil
		st := a.statusLocked()
		a.mu.Unlock()
		a.emit("status", st)
	}()
	a.emit("status", a.GetStatus())

	time.Sleep(1200 * time.Millisecond) // deixa a leitura ao vivo em andamento terminar
	raw, err := d.ReadHistory(cancel, func(n int) {
		a.emit("history-progress", map[string]int{"bytes": n, "max": gmc.FlashSize})
	})
	if err != nil && len(raw) == 0 {
		return ImportResult{}, err
	}

	res := ImportResult{Bytes: len(raw)}
	if len(raw) > 0 {
		dir := filepath.Join(a.dataDir, "historico")
		_ = os.MkdirAll(dir, 0o755)
		res.File = filepath.Join(dir, "historico-"+time.Now().Format("2006-01-02_150405")+".bin")
		_ = os.WriteFile(res.File, raw, 0o644)
	}

	if err == nil {
		a.saveMemoryInfo(raw) // só quando leu até o fim
	}
	h := gmc.ParseHistory(raw)
	res.Records = len(h.Records)
	a.mu.Lock()
	lvDT := a.settings.Levels.norm()
	a.mu.Unlock()
	pts := recordsToPoints(h.Records, lvDT)
	added, ierr := a.store.ImportNew(pts)
	res.Added = len(added)
	res.Alerts = a.retroAlerts(added)
	res.NewRanges = a.newRanges(added)

	msg := fmt.Sprintf("Histórico baixado: %d registros, %d minutos novos", res.Records, res.Added)
	switch res.Alerts {
	case 0:
	case 1:
		msg += ", 1 alerta encontrado"
	default:
		msg += fmt.Sprintf(", %d alertas encontrados", res.Alerts)
	}
	if err != nil {
		msg += " (leitura interrompida: " + err.Error() + ")"
	}
	a.logEvent(msg)
	a.emit("stored", time.Now().UnixMilli())
	if ierr != nil {
		return res, ierr
	}
	return res, nil
}

// CancelDownload interrompe a leitura do histórico.
func (a *App) CancelDownload() {
	a.mu.Lock()
	if a.cancelDownload != nil {
		close(a.cancelDownload)
		a.cancelDownload = nil
	}
	a.mu.Unlock()
}

// recordsToPoints converte os registros do contador em pontos por minuto.
// Gravações por segundo (CPS) viram a soma de cada minuto. Com a
// compensação de tempo morto ligada, cada registro é corrigido antes.
func recordsToPoints(recs []gmc.Record, lv Levels) []store.Point {
	var pts []store.Point
	type acc struct {
		sum float64
		n   int
	}
	perMin := map[int64]*acc{}
	for _, r := range recs {
		switch r.Unit {
		case "CPM":
			pts = append(pts, store.Point{T: r.Time, CPM: math.Round(lv.dtCPM(float64(r.Count))), Src: store.SrcDevice})
		case "CPS":
			m := r.Time.Add(-time.Second).Truncate(time.Minute).Unix()
			x := perMin[m]
			if x == nil {
				x = &acc{}
				perMin[m] = x
			}
			x.sum += lv.dtCPS(float64(r.Count))
			x.n++
		}
	}
	for m, x := range perMin {
		if x.n < 30 { // minuto incompleto demais
			continue
		}
		pts = append(pts, store.Point{T: time.Unix(m, 0), CPM: math.Round(x.sum * 60 / float64(x.n)), Src: store.SrcDevice})
	}
	return pts
}

// ---------- dados para a tela ----------

// GetChart devolve o gráfico do período: "1h", "24h" ou "7d".
func (a *App) GetChart(period string) (Chart, error) {
	now := time.Now()
	var span, step time.Duration
	switch period {
	case "15m":
		span, step = 15*time.Minute, 5*time.Second
	case "30m":
		span, step = 30*time.Minute, 10*time.Second
	case "24h":
		span, step = 24*time.Hour, 5*time.Minute
	case "7d":
		span, step = 7*24*time.Hour, 30*time.Minute
	default:
		span, step = time.Hour, time.Minute
	}
	to := now.Truncate(step).Add(step)
	from := to.Add(-span)
	return a.buildChart(from, to, step)
}

// liveWindow é até onde vão as leituras por segundo guardadas na memória.
const liveWindow = time.Hour

// zoomSteps são os passos possíveis no zoom, do mais fino ao mais grosso.
var zoomSteps = []time.Duration{
	5 * time.Second, 10 * time.Second, 15 * time.Second, 30 * time.Second,
	time.Minute, 2 * time.Minute, 5 * time.Minute, 10 * time.Minute, 15 * time.Minute,
	30 * time.Minute, time.Hour, 2 * time.Hour, 6 * time.Hour, 24 * time.Hour,
}

// GetChartRange é o gráfico de um trecho escolhido arrastando no gráfico
// (zoom). Usa o passo mais fino que dá até ~720 pontos: leituras por segundo
// na última hora e por minuto antes disso.
func (a *App) GetChartRange(fromMs, toMs int64) (Chart, error) {
	now := time.Now()
	from, to := time.UnixMilli(fromMs), time.UnixMilli(toMs)
	if to.Before(from) {
		from, to = to, from
	}
	if max := now.Add(time.Minute); to.After(max) {
		to = max
	}
	if to.Sub(from) < 2*time.Minute { // muito estreito: abre 2 min em volta
		mid := from.Add(to.Sub(from) / 2)
		from, to = mid.Add(-time.Minute), mid.Add(time.Minute)
	}
	span := to.Sub(from)
	step := zoomSteps[len(zoomSteps)-1]
	for _, s := range zoomSteps {
		if span/s <= 720 {
			step = s
			break
		}
	}
	if step < time.Minute && from.Before(now.Add(-liveWindow)) && len(a.alertSamples(from, to)) == 0 {
		step = time.Minute // sem leituras por segundo tão antigas
		for span/step > 720 {
			step *= 2
		}
	}
	from = from.Truncate(step)
	to = ceilTo(to, step)
	c, err := a.buildChart(from, to, step)
	c.FineAlertsOnly = step < time.Minute && from.Before(now.Add(-liveWindow))
	return c, err
}

// buildChart monta o gráfico de [from, to) com o passo dado.
func (a *App) buildChart(from, to time.Time, step time.Duration) (Chart, error) {
	span := to.Sub(from)
	// o minuto gravado às 16:38 vale de 16:38:00 a 16:39:00; se o gráfico
	// começa no meio dele (16:38:20), ele também precisa entrar
	pts, err := a.points(from.Truncate(time.Minute), to)
	if err != nil {
		return Chart{}, err
	}
	if step < time.Minute {
		return a.withUSv(a.shortChart(from, to, step, pts)), nil
	}
	n := int(span / step)
	sums := make([]float64, n)
	cnt := make([]int, n)
	var all []float64
	for _, p := range pts {
		i := int(p.T.Sub(from) / step)
		if p.T.Before(from) || i >= n {
			continue
		}
		sums[i] += p.CPM
		cnt[i]++
		all = append(all, p.CPM)
	}
	c := Chart{From: from.UnixMilli(), To: to.UnixMilli(), StepMs: step.Milliseconds()}
	for i := 0; i < n; i++ {
		c.Times = append(c.Times, from.Add(time.Duration(i)*step).UnixMilli())
		if cnt[i] == 0 {
			c.Values = append(c.Values, nil)
			continue
		}
		v := sums[i] / float64(cnt[i])
		c.Values = append(c.Values, &v)
	}
	if len(all) > 0 {
		sum, mn, mx := 0.0, all[0], all[0]
		for _, v := range all {
			sum += v
			mn = math.Min(mn, v)
			mx = math.Max(mx, v)
		}
		avg := sum / float64(len(all))
		c.Avg, c.Min, c.Max = &avg, &mn, &mx
	}
	return a.withUSv(c), nil
}

// shortChart monta os gráficos de 15 e 30 min com as leituras por segundo.
// Onde não houver leitura por segundo (app aberto há pouco), usa o valor do
// minuto gravado.
func (a *App) shortChart(from, to time.Time, step time.Duration, pts []store.Point) Chart {
	n := int(to.Sub(from) / step)
	sums := make([]float64, n)
	cnt := make([]int, n)
	for _, s := range a.fineSamples(from, to) {
		i := int(s.t.Sub(from) / step)
		if i >= 0 && i < n {
			sums[i] += s.cpm
			cnt[i]++
		}
	}
	byMin := map[int64]float64{}
	for _, p := range pts {
		byMin[p.T.Unix()] = p.CPM
	}
	c := Chart{From: from.UnixMilli(), To: to.UnixMilli(), StepMs: step.Milliseconds()}
	var all []float64
	for i := 0; i < n; i++ {
		t := from.Add(time.Duration(i) * step)
		c.Times = append(c.Times, t.UnixMilli())
		var v float64
		switch {
		case cnt[i] > 0:
			v = sums[i] / float64(cnt[i])
		default:
			m, ok := byMin[t.Truncate(time.Minute).Unix()]
			if !ok {
				c.Values = append(c.Values, nil)
				continue
			}
			v = m
		}
		c.Values = append(c.Values, &v)
		all = append(all, v)
	}
	if len(all) > 0 {
		sum, mn, mx := 0.0, all[0], all[0]
		for _, v := range all {
			sum += v
			mn = math.Min(mn, v)
			mx = math.Max(mx, v)
		}
		avg := sum / float64(len(all))
		c.Avg, c.Min, c.Max = &avg, &mn, &mx
	}
	return c
}

// GetStats devolve média da última hora, mínimo e máximo de 24 h e a dose do dia.
func (a *App) GetStats() (Stats, error) {
	now := time.Now()
	a.mu.Lock()
	cal := a.convCalLocked()
	a.mu.Unlock()
	var s Stats
	pts, err := a.points(now.Add(-24*time.Hour), now.Add(time.Minute))
	if err != nil {
		return s, err
	}
	sum, n := 0.0, 0
	for _, p := range pts {
		if !p.T.Before(now.Add(-time.Hour)) {
			sum += p.CPM
			n++
		}
		if s.Min24h == nil || p.CPM < *s.Min24h {
			v := p.CPM
			s.Min24h = &v
		}
		if s.Max24h == nil || p.CPM > *s.Max24h {
			v := p.CPM
			s.Max24h = &v
			s.Max24hAt = p.T.UnixMilli()
		}
	}
	// o pico de um alerta (leitura por segundo) pode passar do valor do minuto
	for _, p := range a.alertPeaks(now.Add(-24*time.Hour), now.Add(time.Minute)) {
		if s.Max24h == nil || p.CPM > *s.Max24h {
			v := p.CPM
			s.Max24h = &v
			s.Max24hAt = p.T
		}
	}
	if n > 0 {
		v := sum / float64(n)
		s.Avg1h = &v
	}
	y, m, d := now.Date()
	today := time.Date(y, m, d, 0, 0, 0, 0, now.Location())
	tp, _ := a.points(today, now.Add(time.Minute))
	for _, p := range tp {
		s.DoseToday += cal.USv(p.CPM) / 60
	}
	s.MinutesToday = len(tp)
	if bg, ok := a.backgroundCPM(); ok && bg > 0 {
		s.BgCPM = bg
	}
	return s, nil
}

// GetDaily devolve a média de cada um dos últimos dias que têm dados.
func (a *App) GetDaily(days int) ([]DayStat, error) {
	if days <= 0 || days > 60 {
		days = 7
	}
	now := time.Now()
	a.mu.Lock()
	cal := a.convCalLocked()
	a.mu.Unlock()
	y, m, d := now.Date()
	today := time.Date(y, m, d, 0, 0, 0, 0, now.Location())
	from := today.AddDate(0, 0, -(days - 1))
	pts, err := a.points(from, now.Add(time.Minute))
	if err != nil {
		return nil, err
	}
	pts = a.homeOnly(pts) // fundo diário: só em casa
	type acc struct {
		sum, usv float64
		n        int
	}
	byDay := map[int64]*acc{}
	for _, p := range pts {
		yy, mm, dd := p.T.Date()
		k := time.Date(yy, mm, dd, 0, 0, 0, 0, p.T.Location()).UnixMilli()
		x := byDay[k]
		if x == nil {
			x = &acc{}
			byDay[k] = x
		}
		x.sum += p.CPM
		x.usv += cal.USv(p.CPM)
		x.n++
	}
	var out []DayStat
	for k, x := range byDay {
		out = append(out, DayStat{Day: k, CPM: x.sum / float64(x.n), USv: x.usv / float64(x.n), Minutes: x.n})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Day > out[j].Day })
	return out, nil
}

// ExportCSV pergunta onde salvar e exporta todas as leituras guardadas.
func (a *App) ExportCSV() (string, error) {
	path, err := wruntime.SaveFileDialog(a.ctx, wruntime.SaveDialogOptions{
		Title:           "Exportar leituras",
		DefaultFilename: "gmc-leituras-" + time.Now().Format("2006-01-02") + ".csv",
		Filters:         []wruntime.FileFilter{{DisplayName: "CSV", Pattern: "*.csv"}},
	})
	if err != nil || path == "" {
		return "", err
	}
	if !strings.HasSuffix(strings.ToLower(path), ".csv") {
		path += ".csv"
	}
	from, ok := a.store.FirstTime()
	if !ok {
		return "", errors.New("ainda não há leituras guardadas")
	}
	f, err := os.Create(path)
	if err != nil {
		return "", err
	}
	defer f.Close()
	a.mu.Lock()
	cal := a.convCalLocked()
	a.mu.Unlock()
	if err := a.store.ExportCSV(f, from, time.Now().Add(time.Minute), cal.USv); err != nil {
		return "", err
	}
	return path, nil
}

// OpenDataFolder abre a pasta de dados no gerenciador de arquivos.
func (a *App) OpenDataFolder() error {
	return openFolder(a.dataDir)
}

func openFolder(dir string) error {
	var cmd *exec.Cmd
	switch runtime.GOOS {
	case "windows":
		cmd = exec.Command("explorer", dir)
	case "darwin":
		cmd = exec.Command("open", dir)
	default:
		cmd = exec.Command("xdg-open", dir)
	}
	return cmd.Start()
}
