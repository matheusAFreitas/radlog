package app

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

	"radlog/internal/radiacode"
)

// ---------- RadiaCode 10x: conexão, leitura ao vivo e gravação ----------
//
// O app procura o RadiaCode sozinho: primeiro no USB e, se tiver endereço
// Bluetooth configurado (Linux), no Bluetooth. Se o celular estiver com o
// aparelho, a conexão Bluetooth falha e o app tenta de novo depois: quando o
// celular solta, o PC assume.
//
// Grava um ponto por minuto em <dados>/radiacode/AAAA-MM.csv (média do cps e
// do µSv/h que o aparelho calcula), separado das leituras do GMC.

const (
	rcPollEvery  = time.Second
	rcRetryUSB   = 10 * time.Second
	rcRetryBLE   = 30 * time.Second
	rcDirName    = "radiacode"
	rcHeader     = "hora,cps,usvh,n"
	rcCompareMax = 24 * 60
)

// RcState é o que a tela mostra do RadiaCode.
type RcState struct {
	State    string                 `json:"state"` // off, procurando, conectando, conectado, erro
	Via      string                 `json:"via"`   // usb, bluetooth
	Model    string                 `json:"model"`
	Serial   string                 `json:"serial"`
	Firmware string                 `json:"firmware"`
	Err      string                 `json:"err"`
	BLE      string                 `json:"ble"` // endereço configurado
	BLEOK    bool                   `json:"bleOK"`
	Off      bool                   `json:"off"`
	Time     int64                  `json:"time"` // ms da última leitura ao vivo
	CPS      float64                `json:"cps"`
	CPSErr   float64                `json:"cpsErr"` // %
	USvh     float64                `json:"usvh"`
	USvhErr  float64                `json:"usvhErr"` // %
	Dose     float64                `json:"dose"`    // µSv desde o último zerar do aparelho
	DoseSec  int64                  `json:"doseSec"` // s acumulando a dose
	Battery  float64                `json:"battery"` // %
	TempC    float64                `json:"tempC"`
	HasRare  bool                   `json:"hasRare"`
	Alarms   *radiacode.AlarmLimits `json:"alarms"`
	Since    int64                  `json:"since"` // ms em que conectou
	// para a tela principal, conectado ou não
	Today   float64 `json:"today"`   // µSv de hoje pelo que o app gravou
	LastT   int64   `json:"lastT"`   // ms do último minuto gravado
	LastUSv float64 `json:"lastUSv"` // µSv/h desse minuto
	LastCPS float64 `json:"lastCPS"`
}

type rcState struct {
	mu    sync.Mutex
	st    RcState
	dev   *radiacode.Device
	min   rcMinute
	wake  chan struct{}
	fileM sync.Mutex
	ring  []rcSample // última hora ao vivo (para o gráfico nos períodos curtos)
	today rcToday
}

type rcSample struct {
	t        time.Time
	usv, cps float64
}

// rcToday guarda a dose de hoje e o último minuto, sem reler o arquivo a cada
// segundo.
type rcToday struct {
	day     string
	loaded  bool
	sum     float64 // µSv
	last    RcPoint
	hasLast bool
}

type rcMinute struct {
	t        time.Time // início do minuto
	cps, usv float64
	n        int
}

// GetRadiacode devolve o estado do RadiaCode.
func (a *App) GetRadiacode() RcState {
	a.mu.Lock()
	off, ble := a.settings.RcOff, a.settings.RcBLE
	a.mu.Unlock()
	a.rc.mu.Lock()
	defer a.rc.mu.Unlock()
	st := a.rc.st
	if st.State == "" {
		st.State = "procurando"
	}
	if off {
		st.State = "off"
	}
	st.Off, st.BLE, st.BLEOK = off, ble, radiacode.BLESupported
	a.rcTodayLocked()
	st.Today = a.rc.today.sum
	if a.rc.min.n > 0 { // minuto em andamento: conta o que já passou dele
		st.Today += a.rc.min.usv / float64(a.rc.min.n) * float64(time.Since(a.rc.min.t)) / float64(time.Hour)
	}
	if a.rc.today.hasLast {
		l := a.rc.today.last
		st.LastT, st.LastUSv, st.LastCPS = l.T.UnixMilli(), l.USvh, l.CPS
	}
	return st
}

// rcTodayLocked carrega (uma vez por dia) a dose de hoje e o último minuto.
// Precisa de a.rc.mu.
func (a *App) rcTodayLocked() {
	day := time.Now().Format("2006-01-02")
	if a.rc.today.loaded && a.rc.today.day == day {
		return
	}
	last, hasLast := a.rc.today.last, a.rc.today.hasLast
	a.rc.today = rcToday{day: day, loaded: true, last: last, hasLast: hasLast}
	if a.dataDir == "" {
		return
	}
	now := time.Now()
	start := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())
	for _, p := range a.rcRange(start, now.Add(time.Minute)) {
		a.rc.today.sum += p.USvh / 60
	}
	if !hasLast {
		// último minuto gravado: procura neste mês e no anterior
		if pts := a.rcRange(start.AddDate(0, -1, 0), now.Add(time.Minute)); len(pts) > 0 {
			a.rc.today.last, a.rc.today.hasLast = pts[len(pts)-1], true
		}
	}
}

func (a *App) emitRc() { a.emit("radiacode", a.GetRadiacode()) }

// SetRadiacode liga/desliga a procura e grava o endereço Bluetooth.
func (a *App) SetRadiacode(on bool, ble string) RcState {
	ble = strings.ToUpper(strings.TrimSpace(ble))
	a.mu.Lock()
	a.settings.RcOff, a.settings.RcBLE = !on, ble
	a.saveSettingsLocked()
	a.mu.Unlock()
	a.rcWake()
	a.emitRc()
	return a.GetRadiacode()
}

// ScanRadiacodeBLE procura RadiaCodes no Bluetooth (uns 6 s).
func (a *App) ScanRadiacodeBLE() ([]radiacode.BLEDevice, error) {
	list, err := radiacode.ScanBLE(6 * time.Second)
	if err != nil {
		return nil, errors.New(err.Error())
	}
	sort.Slice(list, func(i, j int) bool { return list[i].RSSI > list[j].RSSI })
	return list, nil
}

// ReconnectRadiacode solta e procura de novo agora.
func (a *App) ReconnectRadiacode() RcState {
	a.rc.mu.Lock()
	if d := a.rc.dev; d != nil {
		d.Close()
	}
	a.rc.mu.Unlock()
	a.rcWake()
	return a.GetRadiacode()
}

func (a *App) rcWake() {
	a.rc.mu.Lock()
	w := a.rc.wake
	a.rc.mu.Unlock()
	if w != nil {
		select {
		case w <- struct{}{}:
		default:
		}
	}
}

func (a *App) rcSet(f func(*RcState)) {
	a.rc.mu.Lock()
	f(&a.rc.st)
	a.rc.mu.Unlock()
	a.emitRc()
}

func (a *App) rcLoop() {
	a.rc.mu.Lock()
	a.rc.wake = make(chan struct{}, 1)
	wake := a.rc.wake
	a.rc.mu.Unlock()
	lastBLE := time.Time{}
	wait := func(d time.Duration) {
		select {
		case <-wake:
		case <-time.After(d):
		}
	}
	for {
		a.mu.Lock()
		off, ble := a.settings.RcOff, a.settings.RcBLE
		a.mu.Unlock()
		if off {
			wait(time.Minute)
			continue
		}
		tr, err := radiacode.OpenUSB()
		if errors.Is(err, radiacode.ErrNotFound) && ble != "" && radiacode.BLESupported {
			if time.Since(lastBLE) < rcRetryBLE {
				wait(rcRetryUSB)
				continue
			}
			lastBLE = time.Now()
			a.rcSet(func(s *RcState) { s.State, s.Err = "conectando", "" })
			tr, err = radiacode.OpenBLE(ble)
		}
		if err != nil {
			if errors.Is(err, radiacode.ErrNotFound) {
				a.rcSet(func(s *RcState) { s.State, s.Err = "procurando", "" })
			} else {
				msg := err.Error()
				a.rcSet(func(s *RcState) { s.State, s.Err = "erro", msg })
			}
			wait(rcRetryUSB)
			continue
		}
		a.rcSet(func(s *RcState) { s.State, s.Via, s.Err = "conectando", tr.Kind(), "" })
		dev, err := radiacode.Open(tr)
		if err != nil {
			msg := err.Error()
			a.rcSet(func(s *RcState) { s.State, s.Err = "erro", msg })
			wait(rcRetryUSB)
			continue
		}
		a.rcRun(dev)
		wait(2 * time.Second)
	}
}

// rcRun lê o aparelho até a conexão cair.
func (a *App) rcRun(dev *radiacode.Device) {
	al, alErr := dev.Alarms()
	a.rc.mu.Lock()
	a.rc.dev = dev
	a.rc.st = RcState{State: "conectado", Via: dev.Kind(), Model: dev.Model, Serial: dev.Serial, Firmware: dev.Firmware,
		Since: time.Now().UnixMilli()}
	if alErr == nil {
		a.rc.st.Alarms = &al
	}
	a.rc.mu.Unlock()
	via := map[string]string{"usb": "USB", "bluetooth": "Bluetooth"}[dev.Kind()]
	a.logEvent(fmt.Sprintf("RadiaCode conectado (%s) · %s %s · firmware %s", via, dev.Model, dev.Serial, dev.Firmware))
	a.emitRc()
	fails := 0
	tick := time.NewTicker(rcPollEvery)
	defer tick.Stop()
	for range tick.C {
		a.mu.Lock()
		off := a.settings.RcOff
		a.mu.Unlock()
		if off {
			break
		}
		recs, err := dev.DataBuf()
		if err != nil {
			fails++
			if fails < 3 {
				continue
			}
			msg := err.Error()
			a.logEvent("RadiaCode desconectado: " + msg)
			a.rcSet(func(s *RcState) { s.State, s.Err = "erro", msg })
			break
		}
		fails = 0
		a.rcRecords(recs)
	}
	dev.Close()
	a.rcFlushMinute()
	a.rc.mu.Lock()
	a.rc.dev = nil
	if a.rc.st.State == "conectado" {
		a.rc.st.State = "procurando"
	}
	a.rc.mu.Unlock()
	a.emitRc()
}

func (a *App) rcRecords(recs []radiacode.Record) {
	changed := false
	for _, r := range recs {
		switch v := r.(type) {
		case radiacode.RealTime:
			// a hora do registro vem do relógio do aparelho; para gravar, usa a do PC
			now := time.Now()
			a.rc.mu.Lock()
			a.rc.st.Time, a.rc.st.CPS, a.rc.st.CPSErr = now.UnixMilli(), v.CPS, v.CPSErr
			a.rc.st.USvh, a.rc.st.USvhErr = v.DoseRate, v.DoseErr
			a.rc.mu.Unlock()
			a.rcAddMinute(now, v.CPS, v.DoseRate)
			a.rc.mu.Lock()
			a.rc.ring = append(a.rc.ring, rcSample{now, v.DoseRate, v.CPS})
			if cut := now.Add(-liveWindow); a.rc.ring[0].t.Before(cut) {
				i := 0
				for i < len(a.rc.ring) && a.rc.ring[i].t.Before(cut) {
					i++
				}
				a.rc.ring = append([]rcSample(nil), a.rc.ring[i:]...)
			}
			a.rc.mu.Unlock()
			changed = true
		case radiacode.Rare:
			a.rc.mu.Lock()
			a.rc.st.Dose, a.rc.st.DoseSec, a.rc.st.Battery, a.rc.st.TempC, a.rc.st.HasRare = v.Dose, int64(v.Duration), v.Battery, v.TempC, true
			a.rc.mu.Unlock()
			changed = true
		case radiacode.Event:
			a.logEvent("RadiaCode: " + radiacode.EventName(v.ID))
		}
	}
	if changed {
		a.emitRc()
	}
}

// ---------- gravação por minuto ----------

func (a *App) rcAddMinute(t time.Time, cps, usv float64) {
	m := t.Truncate(time.Minute)
	a.rc.mu.Lock()
	cur := a.rc.min
	if !cur.t.IsZero() && !cur.t.Equal(m) {
		a.rc.min = rcMinute{}
		a.rc.mu.Unlock()
		a.rcWriteMinute(cur)
		a.rc.mu.Lock()
	}
	if a.rc.min.t.IsZero() {
		a.rc.min.t = m
	}
	a.rc.min.cps += cps
	a.rc.min.usv += usv
	a.rc.min.n++
	a.rc.mu.Unlock()
}

func (a *App) rcFlushMinute() {
	a.rc.mu.Lock()
	cur := a.rc.min
	a.rc.min = rcMinute{}
	a.rc.mu.Unlock()
	if cur.n > 0 {
		a.rcWriteMinute(cur)
	}
}

func (a *App) rcDir() string { return filepath.Join(a.dataDir, rcDirName) }

func (a *App) rcWriteMinute(m rcMinute) {
	if m.n == 0 || a.dataDir == "" {
		return
	}
	n := float64(m.n)
	if !a.rcAppend(m) {
		return
	}
	// dose de hoje e último minuto, sem reler o arquivo (fora do fileM: o
	// GetRadiacode tranca rc.mu e depois fileM)
	a.rc.mu.Lock()
	if a.rc.today.loaded && a.rc.today.day == m.t.Format("2006-01-02") {
		a.rc.today.sum += m.usv / n / 60
	}
	a.rc.today.last, a.rc.today.hasLast = RcPoint{T: m.t, CPS: m.cps / n, USvh: m.usv / n, N: m.n}, true
	a.rc.mu.Unlock()
}

func (a *App) rcAppend(m rcMinute) bool {
	a.rc.fileM.Lock()
	defer a.rc.fileM.Unlock()
	dir := a.rcDir()
	os.MkdirAll(dir, 0o755)
	path := filepath.Join(dir, m.t.Format("2006-01")+".csv")
	_, statErr := os.Stat(path)
	f, err := os.OpenFile(path, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o644)
	if err != nil {
		return false
	}
	defer f.Close()
	if statErr != nil {
		fmt.Fprintln(f, rcHeader)
	}
	n := float64(m.n)
	fmt.Fprintf(f, "%s,%.3f,%.5f,%d\n", m.t.Format(time.RFC3339), m.cps/n, m.usv/n, m.n)
	return true
}

// RcPoint é um minuto gravado do RadiaCode.
type RcPoint struct {
	T    time.Time
	CPS  float64
	USvh float64
	N    int
}

// rcRange lê os minutos gravados entre from e to.
func (a *App) rcRange(from, to time.Time) []RcPoint {
	a.rc.fileM.Lock()
	defer a.rc.fileM.Unlock()
	var out []RcPoint
	for m := time.Date(from.Year(), from.Month(), 1, 0, 0, 0, 0, from.Location()); !m.After(to); m = m.AddDate(0, 1, 0) {
		f, err := os.Open(filepath.Join(a.rcDir(), m.Format("2006-01")+".csv"))
		if err != nil {
			continue
		}
		sc := bufio.NewScanner(f)
		for sc.Scan() {
			p := strings.Split(sc.Text(), ",")
			if len(p) < 4 {
				continue
			}
			t, err := time.Parse(time.RFC3339, p[0])
			if err != nil || t.Before(from) || !t.Before(to) {
				continue
			}
			cps, _ := strconv.ParseFloat(p[1], 64)
			usv, _ := strconv.ParseFloat(p[2], 64)
			n, _ := strconv.Atoi(p[3])
			out = append(out, RcPoint{T: t.Local(), CPS: cps, USvh: usv, N: n})
		}
		f.Close()
	}
	sort.Slice(out, func(i, j int) bool { return out[i].T.Before(out[j].T) })
	return out
}

// RcCompare são os minutos em que os dois aparelhos mediram, lado a lado.
type RcCompare struct {
	T       []int64   `json:"t"`   // ms
	GMC     []float64 `json:"gmc"` // µSv/h (calibração do GMC)
	GMCCPM  []float64 `json:"gmcCPM"`
	RC      []float64 `json:"rc"` // µSv/h do RadiaCode
	RCCPS   []float64 `json:"rcCPS"`
	Ratio   float64   `json:"ratio"` // média RC ÷ média GMC (µSv/h)
	RatioE  float64   `json:"ratioErr"`
	Corr    float64   `json:"corr"`    // correlação minuto a minuto
	Minutes int       `json:"minutes"` // minutos com os dois
	RCOnly  int       `json:"rcOnly"`  // minutos só do RadiaCode
}

// GetRcCompare junta os últimos minutos do GMC e do RadiaCode (intercalibração).
func (a *App) GetRcCompare(minutes int) (RcCompare, error) {
	if minutes <= 0 || minutes > rcCompareMax*7 {
		minutes = 6 * 60
	}
	to := time.Now()
	from := to.Add(-time.Duration(minutes) * time.Minute)
	rcs := a.rcRange(from, to)
	var out RcCompare
	if len(rcs) == 0 || a.store == nil {
		return out, nil
	}
	pts, err := a.store.Range(from, to.Add(time.Minute))
	if err != nil {
		return out, err
	}
	a.mu.Lock()
	cal := a.convCalLocked()
	a.mu.Unlock()
	gm := map[int64]float64{}
	for _, p := range pts {
		gm[p.T.Truncate(time.Minute).Unix()] = p.CPM
	}
	var sg, sr, sgg, srr, sgr float64
	for _, r := range rcs {
		cpm, ok := gm[r.T.Truncate(time.Minute).Unix()]
		if !ok {
			out.RCOnly++
			continue
		}
		g := cal.USv(cpm)
		out.T = append(out.T, r.T.UnixMilli())
		out.GMC = append(out.GMC, g)
		out.GMCCPM = append(out.GMCCPM, cpm)
		out.RC = append(out.RC, r.USvh)
		out.RCCPS = append(out.RCCPS, r.CPS)
		sg += g
		sr += r.USvh
		sgg += g * g
		srr += r.USvh * r.USvh
		sgr += g * r.USvh
	}
	n := float64(len(out.T))
	out.Minutes = len(out.T)
	if n >= 2 && sg > 0 {
		out.Ratio = sr / sg
		// incerteza da razão pelo espalhamento das razões por bloco de 10 min
		out.RatioE = blockRatioErr(out.RC, out.GMC, 10)
		cov := sgr/n - (sg/n)*(sr/n)
		vg, vr := sgg/n-(sg/n)*(sg/n), srr/n-(sr/n)*(sr/n)
		if vg > 0 && vr > 0 {
			out.Corr = cov / math.Sqrt(vg*vr)
		}
	}
	return out, nil
}

func blockRatioErr(rc, gmc []float64, size int) float64 {
	var rs []float64
	for i := 0; i+size <= len(rc); i += size {
		var a, b float64
		for j := i; j < i+size; j++ {
			a += rc[j]
			b += gmc[j]
		}
		if b > 0 {
			rs = append(rs, a/b)
		}
	}
	if len(rs) < 3 {
		return 0
	}
	var m, v float64
	for _, r := range rs {
		m += r
	}
	m /= float64(len(rs))
	for _, r := range rs {
		v += (r - m) * (r - m)
	}
	return math.Sqrt(v/float64(len(rs)-1)) / math.Sqrt(float64(len(rs)))
}

// withRc põe a curva do RadiaCode (µSv/h) no gráfico, alinhada aos pontos do
// GMC: nos passos de menos de 1 min usa as leituras ao vivo da última hora,
// e nos outros a média dos minutos gravados.
func (a *App) withRc(c Chart) Chart {
	n := len(c.Times)
	if n == 0 || c.StepMs <= 0 {
		return c
	}
	step := time.Duration(c.StepMs) * time.Millisecond
	from := time.UnixMilli(c.Times[0])
	to := from.Add(time.Duration(n) * step)
	sum := make([]float64, n)
	cnt := make([]int, n)
	add := func(t time.Time, v float64) {
		i := int(t.Sub(from) / step)
		if i >= 0 && i < n {
			sum[i] += v
			cnt[i]++
		}
	}
	if step < time.Minute {
		a.rc.mu.Lock()
		ring := append([]rcSample(nil), a.rc.ring...)
		a.rc.mu.Unlock()
		for _, s := range ring {
			add(s.t, s.usv)
		}
		// mais antigo que a memória: o valor do minuto em cada ponto
		lim := to
		if len(ring) > 0 {
			lim = ring[0].t
		}
		if lim.After(from) {
			for _, p := range a.rcRange(from.Truncate(time.Minute), lim) {
				for t := p.T; t.Before(p.T.Add(time.Minute)) && t.Before(lim); t = t.Add(step) {
					add(t, p.USvh)
				}
			}
		}
	} else {
		for _, p := range a.rcRange(from, to) {
			add(p.T, p.USvh)
		}
	}
	var tot float64
	k := 0
	rc := make([]*float64, n)
	for i := range sum {
		if cnt[i] > 0 {
			v := sum[i] / float64(cnt[i])
			rc[i] = &v
			tot += v
			k++
		}
	}
	if k == 0 {
		return c
	}
	avg := tot / float64(k)
	c.RC, c.RCAvg = rc, &avg
	return c
}

// RcSpectrum é o espectro do aparelho para a tela.
type RcSpectrum struct {
	Seconds float64    `json:"seconds"`
	A       [3]float64 `json:"a"`
	Counts  []uint32   `json:"counts"`
	Accum   bool       `json:"accum"`
}

// GetRcSpectrum lê o espectro atual (ou o acumulado) do aparelho.
func (a *App) GetRcSpectrum(accum bool) (RcSpectrum, error) {
	a.rc.mu.Lock()
	dev := a.rc.dev
	a.rc.mu.Unlock()
	if dev == nil {
		return RcSpectrum{}, errors.New("RadiaCode não conectado")
	}
	var s radiacode.Spectrum
	var err error
	if accum {
		s, err = dev.SpectrumAccum()
	} else {
		s, err = dev.Spectrum()
	}
	if err != nil {
		return RcSpectrum{}, err
	}
	return RcSpectrum{Seconds: s.Duration.Seconds(), A: [3]float64{s.A0, s.A1, s.A2}, Counts: s.Counts, Accum: accum}, nil
}
