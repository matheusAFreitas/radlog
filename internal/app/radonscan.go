package app

// GQ RadonScan (2.105.0): detector de radônio no ar, pelo USB.
//
// O aparelho só guarda quatro médias (1 hora, 1 dia, 1 semana e longo
// prazo) e recalcula o radônio de hora em hora; não tem relógio que o app
// consiga ler. O app lê as médias a cada minuto e grava uma linha em
// radonscan.csv quando elas mudam (ou a cada hora), com a hora do PC e o
// cômodo onde a pessoa disse que o aparelho está. Só leitura: nenhum
// comando muda o RadonScan.
//
// O RadonScan usa o mesmo chip USB do GMC-800 (1A86:7523): a procura só fica
// com a porta que responde "RadonScan…" ao <GETVER>> e nunca mexe na porta
// do GMC-800.

import (
	"bufio"
	"errors"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"radlog/internal/gmc"
	"radlog/internal/radonscan"
)

const (
	radonRetry  = 30 * time.Second // procura do aparelho, desconectado
	radonPoll   = time.Minute      // leitura das médias, conectado
	radonKeep   = time.Hour        // grava uma linha pelo menos a cada hora
	radonWeek   = 7 * 24 * time.Hour
	radonGapMax = 2 * time.Hour // buraco maior que isso não conta como tempo no cômodo
)

// cômodos que a pessoa pode escolher ("" = não disse)
var radonPlaces = map[string]bool{"": true, "sala": true, "quarto": true, "outro": true}

// RadonRow é uma leitura gravada (médias em pCi/L, como o aparelho manda).
type RadonRow struct {
	T     int64   `json:"t"`
	Place string  `json:"place"`
	Hour  float64 `json:"hour"`
	Day   float64 `json:"day"`
	Week  float64 `json:"week"`
	Long  float64 `json:"long"`
	// 2.106.0: hora trazida da memória do aparelho (só Hour vale; T = fim da hora)
	Mem    bool `json:"mem,omitempty"`
	Counts int  `json:"counts,omitempty"`
}

// RadonPlaceStat junta o tempo de cada cômodo com o fundo do GMC-800 do
// mesmo período (o GMC-800 em casa, fora das medições de objeto).
type RadonPlaceStat struct {
	Place  string  `json:"place"`
	Hours  float64 `json:"hours"`
	PCiL   float64 `json:"pciL"`   // média de 1 hora, média no tempo
	GmcCPM float64 `json:"gmcCpm"` // 0 = sem minutos do GMC-800 em casa
	GmcMin int     `json:"gmcMin"`
}

// RadonState vai para a tela (GetRadonScan e o evento "radonscan").
type RadonState struct {
	State   string           `json:"state"` // procurando, conectado, erro
	Port    string           `json:"port"`
	Version string           `json:"version"`
	Err     string           `json:"err"`
	Place   string           `json:"place"`
	Last    *RadonRow        `json:"last"`   // última leitura (gravada ou não)
	ReadAt  int64            `json:"readAt"` // hora da última leitura do aparelho
	First   int64            `json:"first"`  // primeira linha gravada (0 = nenhuma)
	Rows    []RadonRow       `json:"rows"`   // últimos 7 dias
	Places  []RadonPlaceStat `json:"places"` // tempo e média por cômodo
	Seen    bool             `json:"seen"`   // já conectou alguma vez (mostra a página)
	// 2.106.0: horas trazidas da memória do aparelho (também entram em Rows,
	// com Mem), buracos que a memória ainda não cobriu e a última leitura dela
	Gaps    []RadonGap      `json:"gaps"`
	MemLast *RadonMemResult `json:"memLast"`
}

type radonState struct {
	mu      sync.Mutex
	dev     *radonscan.Device
	port    string
	version string
	state   string
	err     string
	last    *RadonRow
	readAt  time.Time
	saved   *RadonRow // última linha gravada
	lastWhy string    // último resultado da procura que foi para o log
	wake    chan struct{}
	// memória do aparelho (2.106.0)
	memLast  *RadonMemResult
	memTries int       // tentativas automáticas nesta conexão
	memNext  time.Time // próxima tentativa automática
}

func (a *App) radonPath() string { return filepath.Join(a.dataDir, "radonscan.csv") }

// radonPort é a porta que o RadonScan está usando ("" = nenhuma): o GMC-800
// não tenta conectar nela.
func (a *App) radonPort() string {
	a.radon.mu.Lock()
	defer a.radon.mu.Unlock()
	if a.radon.dev == nil {
		return ""
	}
	return a.radon.port
}

func (a *App) radonWake() {
	a.radon.mu.Lock()
	w := a.radon.wake
	a.radon.mu.Unlock()
	if w != nil {
		select {
		case w <- struct{}{}:
		default:
		}
	}
}

func (a *App) radonSet(state, msg string) {
	a.radon.mu.Lock()
	changed := a.radon.state != state || a.radon.err != msg
	a.radon.state, a.radon.err = state, msg
	a.radon.mu.Unlock()
	if changed {
		a.emitRadon()
	}
}

func (a *App) emitRadon() { a.emit("radonscan", a.GetRadonScan()) }

// radonCandidates escolhe as portas onde o RadonScan pode estar: USB com o
// chip dos aparelhos GQ, tirando a do GMC-800 (conectado, ou a última dele
// enquanto ele tenta reconectar). A última porta do RadonScan vem primeiro.
func radonCandidates(ports []gmc.PortInfo, known, gmcPort string) []gmc.PortInfo {
	var first, rest []gmc.PortInfo
	for _, p := range ports {
		if !p.IsUSB || !p.LikelyGMC() {
			continue
		}
		if p.Name == gmcPort && p.Name != known {
			continue
		}
		if p.Name == known {
			first = append(first, p)
		} else {
			rest = append(rest, p)
		}
	}
	return append(first, rest...)
}

// radonFind procura o RadonScan e devolve o aparelho aberto.
func (a *App) radonFind() (*radonscan.Device, string, string, error) {
	ports, err := gmc.ListPorts()
	if err != nil {
		return nil, "", "", err
	}
	a.mu.Lock()
	known := a.settings.RadonPort
	gmcPort := a.settings.Port
	if a.dev != nil {
		gmcPort = a.info.Port
	}
	a.mu.Unlock()
	// o GMC-800 no meio de uma conexão segura connMu: não disputa a porta
	if !a.connMu.TryLock() {
		return nil, "", "", errRadonBusy
	}
	a.connMu.Unlock()
	cands := radonCandidates(ports, known, gmcPort)
	var why []string
	for _, p := range cands {
		d, err := withTimeout(5*time.Second, func() (*radonscan.Device, error) { return radonscan.Open(p.Name) })
		if err != nil {
			why = append(why, fmt.Sprintf("%s não abriu (%v)", p.Name, err))
			continue
		}
		ver, err := withTimeout(5*time.Second, d.Version)
		if err == nil && radonscan.IsRadonScan(ver) {
			a.radonProbeLog("")
			return d, p.Name, ver, nil
		}
		d.Close()
		if err != nil {
			why = append(why, fmt.Sprintf("%s não respondeu (%v)", p.Name, err))
		} else {
			why = append(why, fmt.Sprintf("%s: outro aparelho (%q)", p.Name, ver))
		}
	}
	if len(cands) == 0 {
		why = append(why, "nenhuma porta USB com o chip dos aparelhos GQ além da do GMC-800")
	}
	a.radonProbeLog(strings.Join(why, "; "))
	return nil, "", "", errRadonNotFound
}

// radonProbeLog registra no log técnico o resultado da procura só quando
// ele muda (a procura roda a cada 30 s; 2.105.1).
func (a *App) radonProbeLog(why string) {
	a.radon.mu.Lock()
	same := a.radon.lastWhy == why
	a.radon.lastWhy = why
	a.radon.mu.Unlock()
	if !same && why != "" {
		a.logf("radonscan: procurando: %s", why)
	}
}

var (
	errRadonNotFound = errors.New("RadonScan não encontrado")
	errRadonBusy     = errors.New("GMC-800 conectando")
)

// radonLoop procura o RadonScan e lê as médias enquanto ele estiver ligado.
func (a *App) radonLoop() {
	a.radon.mu.Lock()
	a.radon.wake = make(chan struct{}, 1)
	wake := a.radon.wake
	a.radon.mu.Unlock()
	wait := func(d time.Duration) {
		select {
		case <-wake:
		case <-time.After(d):
		}
	}
	a.radonLoadSaved()
	for {
		if a.closing.Load() || a.quitting.Load() {
			return
		}
		d, port, ver, err := a.radonFind()
		if err != nil {
			a.radonSet("procurando", "")
			wait(radonRetry)
			continue
		}
		a.radon.mu.Lock()
		a.radon.dev, a.radon.port, a.radon.version = d, port, ver
		// memória: primeira tentativa uns segundos depois da 1ª leitura
		a.radon.memTries, a.radon.memNext = 0, time.Now().Add(10*time.Second)
		a.radon.mu.Unlock()
		a.mu.Lock()
		if a.settings.RadonPort != port {
			a.settings.RadonPort = port
			a.saveSettingsLocked()
		}
		a.mu.Unlock()
		a.logf("radonscan: conectado, %s em %s", ver, port)
		a.logEvent(fmt.Sprintf("RadonScan conectado (%s em %s)", ver, port))
		a.radonSet("conectado", "")
		for {
			avg, err := withTimeout(10*time.Second, d.ReadAverages)
			if err != nil {
				a.logf("radonscan: leitura falhou: %v", err)
				break
			}
			a.radonReading(time.Now(), avg)
			a.radonMemAuto(time.Now())
			wait(radonPoll)
			if a.closing.Load() || a.quitting.Load() {
				break
			}
		}
		a.radonClose()
		a.logEvent("RadonScan desconectado")
		a.radonSet("procurando", "")
		wait(radonRetry)
	}
}

// radonClose fecha o aparelho (ao desconectar e ao fechar o app).
func (a *App) radonClose() {
	a.radon.mu.Lock()
	d := a.radon.dev
	a.radon.dev = nil
	a.radon.mu.Unlock()
	if d != nil {
		d.Close()
	}
}

// radonReading guarda a leitura e grava a linha quando as médias mudaram,
// o cômodo mudou ou já passou uma hora desde a última.
func (a *App) radonReading(now time.Time, avg radonscan.Averages) {
	a.mu.Lock()
	place := a.settings.RadonPlace
	a.mu.Unlock()
	row := RadonRow{T: now.UnixMilli(), Place: place, Hour: avg.Hour, Day: avg.Day, Week: avg.Week, Long: avg.Long}
	a.radon.mu.Lock()
	a.radon.last, a.radon.readAt = &row, now
	s := a.radon.saved
	save := s == nil || s.Place != row.Place || s.Hour != row.Hour || s.Day != row.Day || s.Week != row.Week || s.Long != row.Long ||
		now.Sub(time.UnixMilli(s.T)) >= radonKeep
	a.radon.mu.Unlock()
	if save {
		if err := a.radonAppend(row); err != nil {
			a.logf("radonscan: não gravei a leitura: %v", err)
		} else {
			a.radon.mu.Lock()
			a.radon.saved = &row
			a.radon.mu.Unlock()
		}
	}
	a.emitRadon()
}

const radonHeader = "hora,comodo,media_1h_pCiL,media_1d_pCiL,media_1sem_pCiL,longo_prazo_pCiL"

func (a *App) radonAppend(r RadonRow) error {
	path := a.radonPath()
	_, statErr := os.Stat(path)
	f, err := os.OpenFile(path, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o644)
	if err != nil {
		return err
	}
	w := bufio.NewWriter(f)
	if os.IsNotExist(statErr) {
		w.WriteString(radonHeader + "\n")
	}
	w.WriteString(formatRadonRow(r) + "\n")
	if err := w.Flush(); err != nil {
		f.Close()
		return err
	}
	return f.Close()
}

func formatRadonRow(r RadonRow) string {
	ff := func(v float64) string { return strconv.FormatFloat(v, 'f', -1, 64) }
	return time.UnixMilli(r.T).Format(time.RFC3339) + "," + r.Place + "," + ff(r.Hour) + "," + ff(r.Day) + "," + ff(r.Week) + "," + ff(r.Long)
}

func parseRadonRow(line string) (RadonRow, bool) {
	f := strings.Split(strings.TrimSpace(line), ",")
	if len(f) != 6 {
		return RadonRow{}, false
	}
	t, err := time.Parse(time.RFC3339, f[0])
	if err != nil {
		return RadonRow{}, false
	}
	r := RadonRow{T: t.UnixMilli(), Place: f[1]}
	for i, p := range []*float64{&r.Hour, &r.Day, &r.Week, &r.Long} {
		v, err := strconv.ParseFloat(f[2+i], 64)
		if err != nil || v < 0 || math.IsNaN(v) {
			return RadonRow{}, false
		}
		*p = v
	}
	return r, true
}

// radonRows lê o arquivo inteiro (umas 24 linhas por dia: é pequeno).
func (a *App) radonRows() []RadonRow {
	b, err := os.ReadFile(a.radonPath())
	if err != nil {
		return nil
	}
	var out []RadonRow
	for _, ln := range strings.Split(string(b), "\n") {
		if r, ok := parseRadonRow(ln); ok {
			out = append(out, r)
		}
	}
	sort.SliceStable(out, func(i, j int) bool { return out[i].T < out[j].T })
	return out
}

func (a *App) radonLoadSaved() {
	rows := a.radonRows()
	if len(rows) == 0 {
		return
	}
	r := rows[len(rows)-1]
	a.radon.mu.Lock()
	if a.radon.saved == nil {
		a.radon.saved = &r
	}
	a.radon.mu.Unlock()
}

// radonPlaceStats: tempo em cada cômodo (cada linha vale até a próxima,
// no máximo radonGapMax), a média de 1 hora no tempo e o fundo do GMC-800 em
// casa no mesmo período.
func radonPlaceStats(rows []RadonRow, now time.Time, gmcMean func(from, to time.Time) (float64, int)) []RadonPlaceStat {
	type acc struct {
		hours, sum float64
		spans      [][2]time.Time
	}
	by := map[string]*acc{}
	var order []string
	for i, r := range rows {
		if r.Place == "" {
			continue
		}
		end := now
		if i+1 < len(rows) {
			end = time.UnixMilli(rows[i+1].T)
		}
		from := time.UnixMilli(r.T)
		if end.Sub(from) > radonGapMax {
			end = from.Add(radonGapMax)
		}
		h := end.Sub(from).Hours()
		if h <= 0 {
			continue
		}
		x := by[r.Place]
		if x == nil {
			x = &acc{}
			by[r.Place] = x
			order = append(order, r.Place)
		}
		x.hours += h
		x.sum += h * r.Hour
		if n := len(x.spans); n > 0 && !x.spans[n-1][1].Before(from) {
			x.spans[n-1][1] = end
		} else {
			x.spans = append(x.spans, [2]time.Time{from, end})
		}
	}
	var out []RadonPlaceStat
	for _, p := range order {
		x := by[p]
		st := RadonPlaceStat{Place: p, Hours: x.hours, PCiL: x.sum / x.hours}
		if gmcMean != nil {
			var tot float64
			for _, sp := range x.spans {
				m, n := gmcMean(sp[0], sp[1])
				tot += m * float64(n)
				st.GmcMin += n
			}
			if st.GmcMin > 0 {
				st.GmcCPM = tot / float64(st.GmcMin)
			}
		}
		out = append(out, st)
	}
	return out
}

// radonGmcMean é o CPM médio do GMC-800 em casa (fora das medições) entre
// from e to.
func (a *App) radonGmcMean() func(from, to time.Time) (float64, int) {
	if a.store == nil {
		return nil
	}
	away := a.awayCheckerFor("gmc")
	measuring := a.measuringChecker()
	return func(from, to time.Time) (float64, int) {
		pts, err := a.store.Range(from, to)
		if err != nil {
			return 0, 0
		}
		var sum float64
		n := 0
		for _, p := range pts {
			if away(p.T) || measuring(p.T) {
				continue
			}
			sum += p.CPM
			n++
		}
		if n == 0 {
			return 0, 0
		}
		return sum / float64(n), n
	}
}

// GetRadonScan devolve o estado do RadonScan, os últimos 7 dias e o quadro
// por cômodo.
func (a *App) GetRadonScan() RadonState {
	now := time.Now()
	rows := a.radonRows()
	a.mu.Lock()
	place := a.settings.RadonPlace
	known := a.settings.RadonPort != ""
	a.mu.Unlock()
	a.radon.mu.Lock()
	st := RadonState{State: a.radon.state, Err: a.radon.err, Place: place}
	if st.State == "" {
		st.State = "procurando"
	}
	if a.radon.dev != nil {
		st.Port, st.Version = a.radon.port, a.radon.version
	}
	if a.radon.last != nil {
		l := *a.radon.last
		st.Last = &l
		st.ReadAt = a.radon.readAt.UnixMilli()
	}
	a.radon.mu.Unlock()
	if st.Last == nil && len(rows) > 0 {
		l := rows[len(rows)-1]
		st.Last = &l
	}
	if len(rows) > 0 {
		st.First = rows[0].T
	}
	mem := a.radonMemRows()
	all := radonMergeMem(rows, mem)
	cut := now.Add(-radonWeek).UnixMilli()
	for _, r := range all {
		if r.T >= cut {
			st.Rows = append(st.Rows, r)
		}
	}
	st.Places = radonPlaceStats(all, now, a.radonGmcMean())
	st.Gaps = radonGaps(rows, mem, now)
	a.radon.mu.Lock()
	if a.radon.memLast != nil {
		m := *a.radon.memLast
		st.MemLast = &m
	}
	a.radon.mu.Unlock()
	st.Seen = known || len(rows) > 0 || st.State == "conectado"
	return st
}

// SetRadonPlace muda o cômodo onde o RadonScan está ("sala", "quarto",
// "outro" ou "" para não dizer). A próxima leitura já sai com ele.
func (a *App) SetRadonPlace(place string) (RadonState, error) {
	if !radonPlaces[place] {
		return a.GetRadonScan(), fmt.Errorf("cômodo desconhecido: %q", place)
	}
	a.mu.Lock()
	old := a.settings.RadonPlace
	a.settings.RadonPlace = place
	a.saveSettingsLocked()
	a.mu.Unlock()
	if old != place {
		a.logf("radonscan: cômodo %q → %q", old, place)
		a.radonWake() // lê agora e grava a linha com o cômodo novo
	}
	return a.GetRadonScan(), nil
}
