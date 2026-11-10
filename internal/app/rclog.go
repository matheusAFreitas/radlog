package app

import (
	"bufio"
	"bytes"
	"encoding/csv"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	wruntime "github.com/wailsapp/wails/v2/pkg/runtime"

	"radlog/internal/radiacode"
)

// ---------- log do RadiaCode (como o do app do celular) ----------
//
// O aparelho manda, além da taxa ao vivo, três tipos de registro que o app do
// celular mostra na aba Log: a taxa de dose de ~96 em ~96 s (com a contagem),
// os "raros" (temperatura, bateria e dose acumulada) e os eventos (ligou,
// começou a carregar, alarme…). Sem ninguém conectado o RadiaCode guarda
// esses registros na memória e manda tudo na próxima conexão, apagando o
// que mandou (rcmemory.go): quem conectar primeiro, o PC ou o celular, leva.
// Por isso o log do celular pode ser importado (o .csv que ele exporta) e
// junta com o do PC.
//
// radiacode/log-AAAA-MM.csv, uma linha por registro, com a hora do PC.

const (
	rcLogHeader = "hora,tipo,usvh,usvh_erro_pct,cps,cps_erro_pct,contagens,temp_c,bateria_pct,fonte,dose_usv,dose_seg,evento,codigo,param,origem"
	rcLogMark   = "log-recuperado"
)

// RcLogItem é uma linha do log.
type RcLogItem struct {
	T       int64   `json:"t"`
	Kind    string  `json:"kind"` // "taxa" | "raro" | "evento"
	USv     float64 `json:"usv,omitempty"`
	USvErr  float64 `json:"usvErr,omitempty"` // %
	CPS     float64 `json:"cps,omitempty"`
	CPSErr  float64 `json:"cpsErr,omitempty"` // % (95%, como o app do celular)
	Count   float64 `json:"count,omitempty"`
	Temp    float64 `json:"temp,omitempty"`
	HasTemp bool    `json:"hasTemp,omitempty"`
	Battery float64 `json:"battery,omitempty"`
	HasBat  bool    `json:"hasBat,omitempty"`
	Power   string  `json:"power,omitempty"` // "" | "externa" | "carregando" (só do log do celular: os bits do aparelho não batem com a tomada)
	Dose    float64 `json:"dose,omitempty"`  // µSv
	HasDose bool    `json:"hasDose,omitempty"`
	DoseSec float64 `json:"doseSec,omitempty"`
	Event   string  `json:"event,omitempty"`
	Code    int     `json:"code,omitempty"`
	Param   int     `json:"param,omitempty"`
	Src     string  `json:"src"` // "pc" | "celular"
}

func rcLogFileName(t time.Time) string { return "log-" + t.Format("2006-01") + ".csv" }

func fmtF(v float64, d int) string { return strconv.FormatFloat(v, 'f', d, 64) }

func (it RcLogItem) csvLine() string {
	f := make([]string, 16)
	f[0] = time.UnixMilli(it.T).Format(time.RFC3339)
	f[1] = it.Kind
	switch it.Kind {
	case "taxa":
		f[2], f[3], f[4], f[5] = fmtF(it.USv, 4), fmtF(it.USvErr, 1), fmtF(it.CPS, 3), fmtF(it.CPSErr, 1)
		if it.Count > 0 {
			f[6] = fmtF(it.Count, 0)
		}
	case "raro":
		if it.HasTemp {
			f[7] = fmtF(it.Temp, 1)
		}
		if it.HasBat {
			f[8] = fmtF(it.Battery, 0)
		}
		f[9] = it.Power
		if it.HasDose {
			f[10], f[11] = fmtF(it.Dose, 3), fmtF(it.DoseSec, 0)
		}
	case "evento":
		f[12], f[13], f[14] = strings.ReplaceAll(it.Event, ",", ";"), strconv.Itoa(it.Code), strconv.Itoa(it.Param)
	}
	f[15] = it.Src
	return strings.Join(f, ",")
}

func parseRcLogLine(s string) (RcLogItem, bool) {
	f := strings.Split(s, ",")
	if len(f) < 16 || f[0] == "hora" {
		return RcLogItem{}, false
	}
	t, err := time.Parse(time.RFC3339, f[0])
	if err != nil {
		return RcLogItem{}, false
	}
	num := func(i int) (float64, bool) {
		v, err := strconv.ParseFloat(f[i], 64)
		return v, err == nil
	}
	it := RcLogItem{T: t.UnixMilli(), Kind: f[1], Src: f[15]}
	switch it.Kind {
	case "taxa":
		it.USv, _ = num(2)
		it.USvErr, _ = num(3)
		it.CPS, _ = num(4)
		it.CPSErr, _ = num(5)
		it.Count, _ = num(6)
	case "raro":
		it.Temp, it.HasTemp = num(7)
		it.Battery, it.HasBat = num(8)
		it.Power = f[9]
		it.Dose, it.HasDose = num(10)
		it.DoseSec, _ = num(11)
	case "evento":
		it.Event = f[12]
		it.Code, _ = strconv.Atoi(f[13])
		it.Param, _ = strconv.Atoi(f[14])
	default:
		return RcLogItem{}, false
	}
	return it, true
}

// rcLogFromRecord converte um registro do aparelho (nil = não vai no log).
func rcLogFromRecord(r radiacode.Record, at time.Time) *RcLogItem {
	switch v := r.(type) {
	case radiacode.DoseRateDB:
		it := &RcLogItem{T: at.UnixMilli(), Kind: "taxa", USv: v.DoseRate, USvErr: v.DoseErr, CPS: v.CPS, Count: float64(v.Count), Src: "pc"}
		if v.Count > 0 {
			it.CPSErr = 200 / math.Sqrt(float64(v.Count))
		}
		return it
	case radiacode.Rare:
		return &RcLogItem{T: at.UnixMilli(), Kind: "raro", Temp: v.TempC, HasTemp: true, Battery: v.Battery, HasBat: true,
			Dose: v.Dose, HasDose: true, DoseSec: float64(v.Duration), Src: "pc"}
	case radiacode.Event:
		if v.ID == 17 { // mensagem de texto: vai para o log.txt
			return nil
		}
		return &RcLogItem{T: at.UnixMilli(), Kind: "evento", Event: radiacode.EventName(v.ID), Code: v.ID, Param: v.Param, Src: "pc"}
	}
	return nil
}

// rcLogAppend grava as linhas (cada uma no arquivo do mês dela).
func (a *App) rcLogAppend(items []RcLogItem) {
	if len(items) == 0 || a.dataDir == "" {
		return
	}
	a.rc.fileM.Lock()
	defer a.rc.fileM.Unlock()
	by := map[string][]string{}
	for _, it := range items {
		n := rcLogFileName(time.UnixMilli(it.T))
		by[n] = append(by[n], it.csvLine())
	}
	os.MkdirAll(a.rcDir(), 0o755)
	for n, lines := range by {
		p := filepath.Join(a.rcDir(), n)
		_, err := os.Stat(p)
		f, ferr := os.OpenFile(p, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o644)
		if ferr != nil {
			continue
		}
		if err != nil {
			f.WriteString(rcLogHeader + "\n")
		}
		f.WriteString(strings.Join(lines, "\n") + "\n")
		f.Close()
	}
}

// rcLogMergeLines junta linhas num arquivo do log, sem repetir, em ordem.
// Devolve quantas entraram. Chame com fileM trancado.
func (a *App) rcLogMergeLocked(name string, lines []string) int {
	dst := filepath.Join(a.rcDir(), name)
	have := map[string]bool{}
	var all []RcLogItem
	key := func(it RcLogItem) string { // a origem não conta: o mesmo registro pode vir dos dois
		it.Src = ""
		return it.csvLine()
	}
	if b, err := os.ReadFile(dst); err == nil {
		sc := bufio.NewScanner(bytes.NewReader(b))
		for sc.Scan() {
			if it, ok := parseRcLogLine(sc.Text()); ok && !have[key(it)] {
				have[key(it)] = true
				all = append(all, it)
			}
		}
	}
	added := 0
	for _, l := range lines {
		it, ok := parseRcLogLine(l)
		if !ok || have[key(it)] {
			continue
		}
		have[key(it)] = true
		all = append(all, it)
		added++
	}
	if added == 0 {
		return 0
	}
	sort.SliceStable(all, func(i, j int) bool { return all[i].T < all[j].T })
	var out bytes.Buffer
	out.WriteString(rcLogHeader + "\n")
	for _, it := range all {
		out.WriteString(it.csvLine() + "\n")
	}
	os.MkdirAll(a.rcDir(), 0o755)
	tmp := dst + ".tmp"
	if os.WriteFile(tmp, out.Bytes(), 0o644) == nil {
		os.Rename(tmp, dst)
	}
	return added
}

func (a *App) rcLogMerge(items []RcLogItem) int {
	by := map[string][]string{}
	for _, it := range items {
		n := rcLogFileName(time.UnixMilli(it.T))
		by[n] = append(by[n], it.csvLine())
	}
	a.rc.fileM.Lock()
	defer a.rc.fileM.Unlock()
	n := 0
	for name, lines := range by {
		n += a.rcLogMergeLocked(name, lines)
	}
	return n
}

// rcLogRecords recebe os registros de uma leitura do DATA_BUF.
func (a *App) rcLogRecords(recs []radiacode.Record) {
	now := time.Now()
	var items []RcLogItem
	for _, r := range recs {
		if e, ok := r.(radiacode.Event); ok && e.ID == 3 {
			// "configuração alterada": vem a cada toque no menu; uma por minuto
			a.rc.mu.Lock()
			recent := now.Sub(a.rc.logCfgAt) < time.Minute
			if !recent {
				a.rc.logCfgAt = now
			}
			a.rc.mu.Unlock()
			if recent {
				continue
			}
		}
		if it := rcLogFromRecord(r, now); it != nil {
			items = append(items, *it)
		}
	}
	a.rcLogAppend(items)
}

// ---------- recupera dos dados brutos (uma vez) ----------

func rcLogFromRaw(lines [][]byte) []RcLogItem {
	var out []RcLogItem
	var cur int64 = -1
	var buf []byte
	seen := map[int64]bool{}
	var cfgAt int64
	flush := func() {
		if len(buf) == 0 || seen[cur] {
			buf = nil
			return
		}
		seen[cur] = true
		live, _ := splitOldRecords(radiacode.DecodeDataBuf(buf, time.Unix(0, 0)), time.Time{}) // os da memória não têm a hora do "pc"
		for _, r := range live {
			if e, ok := r.(radiacode.Event); ok && e.ID == 3 {
				if cur-cfgAt < 60e3 {
					continue
				}
				cfgAt = cur
			}
			if it := rcLogFromRecord(r, time.UnixMilli(cur)); it != nil {
				out = append(out, *it)
			}
		}
		buf = nil
	}
	for _, ln := range lines {
		var r struct {
			PC  int64  `json:"pc"`
			Hex string `json:"hex"`
		}
		if json.Unmarshal(ln, &r) != nil || r.Hex == "" || r.PC == 0 {
			continue
		}
		if r.PC != cur {
			flush()
			cur = r.PC
		}
		if h, err := hex.DecodeString(r.Hex); err == nil {
			buf = append(buf, h...)
		}
	}
	flush()
	return out
}

// rcLogBackfill monta o log com os dados brutos já gravados (uma vez).
func (a *App) rcLogBackfill() {
	if a.dataDir == "" {
		return
	}
	mark := filepath.Join(a.rcDir(), rcLogMark)
	if _, err := os.Stat(mark); err == nil {
		return
	}
	files, _ := filepath.Glob(filepath.Join(a.rcDir(), rcRawDir, "*.jsonl.gz"))
	var items []RcLogItem
	for _, f := range files {
		if b, err := os.ReadFile(f); err == nil {
			items = append(items, rcLogFromRaw(gzipLinesTolerant(b))...)
		}
	}
	if n := a.rcLogMerge(items); n > 0 {
		a.logf("RadiaCode: %d registros do log recuperados dos dados brutos", n)
	}
	os.MkdirAll(a.rcDir(), 0o755)
	os.WriteFile(mark, []byte(time.Now().Format(time.RFC3339)+"\n"), 0o644)
}

// ---------- leitura ----------

// RcLogPage é um pedaço do log (do mais novo para o mais velho).
type RcLogPage struct {
	Items []RcLogItem    `json:"items"`
	Shown int            `json:"shown"` // linhas no período com os tipos escolhidos
	Total int            `json:"total"` // linhas no período, de todos os tipos
	Kinds map[string]int `json:"kinds"` // linhas no período por tipo
	More  bool           `json:"more"`
	First int64          `json:"first"` // primeira linha gravada (0 = nenhuma)
}

// GetRcLog devolve até limit linhas de fromMs a toMs (0 = sem limite), dos
// tipos pedidos, mais velhas que beforeMs (0 = do fim), da mais nova para a
// mais velha.
func (a *App) GetRcLog(fromMs, toMs int64, kinds []string, limit int, beforeMs int64) RcLogPage {
	out := RcLogPage{Items: []RcLogItem{}, Kinds: map[string]int{}}
	if a.dataDir == "" {
		return out
	}
	if limit <= 0 || limit > 2000 {
		limit = 200
	}
	want := map[string]bool{}
	for _, k := range kinds {
		want[k] = true
	}
	files, _ := filepath.Glob(filepath.Join(a.rcDir(), "log-*.csv"))
	sort.Strings(files)
	var all []RcLogItem
	a.rc.fileM.Lock()
	for i, f := range files {
		b, err := os.ReadFile(f)
		if err != nil {
			continue
		}
		sc := bufio.NewScanner(bytes.NewReader(b))
		for sc.Scan() {
			it, ok := parseRcLogLine(sc.Text())
			if !ok {
				continue
			}
			if i == 0 && (out.First == 0 || it.T < out.First) {
				out.First = it.T
			}
			if (fromMs > 0 && it.T < fromMs) || (toMs > 0 && it.T > toMs) {
				continue
			}
			out.Total++
			out.Kinds[it.Kind]++
			if want[it.Kind] {
				all = append(all, it)
			}
		}
	}
	a.rc.fileM.Unlock()
	out.Shown = len(all)
	sort.SliceStable(all, func(i, j int) bool { return all[i].T > all[j].T })
	for _, it := range all {
		if beforeMs > 0 && it.T >= beforeMs {
			continue
		}
		if len(out.Items) == limit {
			out.More = true
			break
		}
		out.Items = append(out.Items, it)
	}
	return out
}

// ---------- importar o log do app do celular ----------

// RcLogImport é o resultado de importar um .csv do app do celular.
type RcLogImport struct {
	File    string `json:"file"`
	Read    int    `json:"read"`    // linhas reconhecidas
	Added   int    `json:"added"`   // novas
	Skipped int    `json:"skipped"` // linhas que não deu para ler
	From    int64  `json:"from"`
	To      int64  `json:"to"`
}

// ImportRcLog pede o .csv que o app do celular exporta e junta no log.
func (a *App) ImportRcLog() (RcLogImport, error) {
	if a.ctx == nil {
		return RcLogImport{}, errors.New("sem janela")
	}
	src, err := wruntime.OpenFileDialog(a.ctx, wruntime.OpenDialogOptions{
		Title:   "Importar o log do app do RadiaCode",
		Filters: []wruntime.FileFilter{{DisplayName: "Log do RadiaCode (*.csv)", Pattern: "*.csv"}},
	})
	if err != nil || src == "" {
		return RcLogImport{}, err
	}
	b, err := os.ReadFile(src)
	if err != nil {
		return RcLogImport{}, err
	}
	res, err := a.importRcLog(b)
	res.File = filepath.Base(src)
	if err != nil {
		return res, err
	}
	a.logEvent(fmt.Sprintf("Log do RadiaCode importado (%s): %d registros novos de %d", res.File, res.Added, res.Read))
	return res, nil
}

func (a *App) importRcLog(b []byte) (RcLogImport, error) {
	items, skipped, err := parseMobileLog(b)
	res := RcLogImport{Read: len(items), Skipped: skipped}
	if err != nil {
		return res, err
	}
	for _, it := range items {
		if res.From == 0 || it.T < res.From {
			res.From = it.T
		}
		if it.T > res.To {
			res.To = it.T
		}
	}
	res.Added = a.rcLogMerge(items)
	return res, nil
}

var (
	reNumUnit = regexp.MustCompile(`^\s*([-+]?[0-9]*[.,]?[0-9]+(?:[eE][-+]?[0-9]+)?)\s*(\S*)`)
	reRealT   = regexp.MustCompile(`^(\d{4})[.\-/](\d{2})[.\-/](\d{2})\s+(\d{1,2})[.:](\d{2})[.:](\d{2})`)
)

// numUnit lê "15.3 µR/h", "1.06 µSv", "28.1°C", "100%".
func numUnit(s string) (float64, string, bool) {
	m := reNumUnit.FindStringSubmatch(s)
	if m == nil {
		return 0, "", false
	}
	v, err := strconv.ParseFloat(strings.ReplaceAll(m[1], ",", "."), 64)
	if err != nil {
		return 0, "", false
	}
	return v, strings.TrimSpace(m[2]), true
}

// toUSv converte dose (µSv) ou taxa (µSv/h) para µSv; R vale 0,01 Sv, como
// no app do RadiaCode. ok=false se a unidade não é conhecida.
func toUSv(v float64, unit string) (float64, bool) {
	u := strings.TrimSuffix(strings.TrimSuffix(strings.ReplaceAll(unit, "μ", "µ"), "/h"), "/hr")
	mult := map[string]float64{"Sv": 1e6, "mSv": 1e3, "µSv": 1, "uSv": 1, "nSv": 1e-3,
		"R": 1e4, "mR": 10, "µR": 0.01, "uR": 0.01, "Gy": 1e6, "mGy": 1e3, "µGy": 1, "uGy": 1, "nGy": 1e-3, "": 1}
	k, ok := mult[u]
	return v * k, ok
}

// durSec lê "7:11:46", "1:02:03:04" ou segundos.
func durSec(s string) (float64, bool) {
	s = strings.TrimSpace(s)
	if s == "" {
		return 0, false
	}
	parts := strings.Split(s, ":")
	t := 0.0
	for _, p := range parts {
		v, err := strconv.ParseFloat(strings.TrimSpace(p), 64)
		if err != nil {
			return 0, false
		}
		t = t*60 + v
	}
	if len(parts) == 4 { // d:h:m:s
		d, _ := strconv.ParseFloat(parts[0], 64)
		h, _ := strconv.ParseFloat(parts[1], 64)
		m, _ := strconv.ParseFloat(parts[2], 64)
		ss, _ := strconv.ParseFloat(parts[3], 64)
		t = ((d*24+h)*60+m)*60 + ss
	}
	return t, true
}

// eventos do app do celular (em inglês) para os nomes do app
var mobileEvents = map[string]int{
	"device power off": 0, "device power on": 1, "low battery power off": 2, "device parameters changed": 3,
	"dose reset": 4, "user event": 5, "battery empty alarm": 6, "battery charge begin": 7, "battery charge end": 8,
	"dose rate alarm 1": 9, "dose rate alarm 2": 10, "dose rate out of scale": 11, "dose alarm 1": 12, "dose alarm 2": 13,
	"dose out of scale": 14, "temperature too low": 15, "temperature too high": 16, "text message": 17,
	"memory snapshot": 18, "spectrum reset": 19, "count rate alarm 1": 20, "count rate alarm 2": 21, "count rate out of scale": 22,
}

// parseMobileLog lê o .csv do app do RadiaCode (Android/iOS/Windows): uma
// linha por registro, com "Real time", "Count rate", "Dose rate", "Dose",
// "Temp.", "Charge", "Event"… (as colunas são achadas pelo nome).
func parseMobileLog(b []byte) ([]RcLogItem, int, error) {
	b = bytes.TrimPrefix(b, []byte{0xEF, 0xBB, 0xBF})
	if !utf8.Valid(b) { // o app grava em Latin-1 (µ, °)
		r := make([]rune, len(b))
		for i, c := range b {
			r[i] = rune(c)
		}
		b = []byte(string(r))
	}
	cr := csv.NewReader(bytes.NewReader(b))
	cr.FieldsPerRecord = -1
	cr.LazyQuotes = true
	if first, _, _ := bufio.NewReader(bytes.NewReader(b)).ReadLine(); strings.Count(string(first), ";") > strings.Count(string(first), ",") {
		cr.Comma = ';'
	}
	rows, err := cr.ReadAll()
	if err != nil && len(rows) == 0 {
		return nil, 0, fmt.Errorf("não consegui ler o arquivo: %v", err)
	}
	if len(rows) < 2 {
		return nil, 0, errors.New("o arquivo não tem registros")
	}
	// colunas pelo nome; nas exportações em que a unidade vem no cabeçalho
	// ("Dose rate, Sv/h"; valores só com o número), a unidade fica guardada
	col := map[string]int{}
	hdrUnit := map[string]string{}
	for i, h := range rows[0] {
		h = strings.TrimSpace(h)
		low := strings.ToLower(h)
		col[low] = i
		if base, unit, ok := strings.Cut(h, ","); ok {
			b := strings.ToLower(strings.TrimSpace(base))
			if _, dup := col[b]; !dup {
				col[b] = i
				hdrUnit[b] = strings.TrimSpace(unit)
			}
		}
	}
	// nomes alternativos (app do celular × exportação do app de PC)
	alias := map[string][]string{
		"real time": {"time"}, "dr error": {"dose rate error"}, "temp.": {"temperature"},
		"duration of d.a.": {"duration of dose accumulation"},
	}
	colOf := func(name string) (int, string, bool) {
		for _, n := range append([]string{name}, alias[name]...) {
			if i, ok := col[n]; ok {
				return i, hdrUnit[n], true
			}
		}
		return 0, "", false
	}
	_, _, hasT := colOf("real time")
	_, _, hasTS := colOf("timestamp")
	if !hasT && !hasTS {
		return nil, 0, errors.New("não parece o log do app do RadiaCode (falta a coluna \"Real time\" ou \"Time\")")
	}
	get := func(r []string, name string) string {
		if i, _, ok := colOf(name); ok && i < len(r) {
			return strings.TrimSpace(r[i])
		}
		return ""
	}
	// número com unidade; sem unidade no valor, vale a do cabeçalho
	numU := func(r []string, name string) (float64, string, bool) {
		v, u, ok := numUnit(get(r, name))
		if ok && u == "" {
			_, hu, _ := colOf(name)
			u = hu
		}
		return v, u, ok
	}
	var out []RcLogItem
	skipped := 0
	for _, r := range rows[1:] {
		t, ok := mobileTime(get(r, "real time"), get(r, "timestamp"))
		if !ok {
			skipped++
			continue
		}
		base := RcLogItem{T: t.UnixMilli(), Src: "celular"}
		n := 0
		// taxa de dose
		if v, u, ok := numU(r, "dose rate"); ok {
			if usv, ok := toUSv(v, u); ok {
				it := base
				it.Kind, it.USv = "taxa", usv
				it.USvErr, _, _ = numUnit(get(r, "dr error"))
				it.CPS, _, _ = numUnit(get(r, "count rate"))
				it.CPSErr, _, _ = numUnit(get(r, "cr error"))
				it.Count, _, _ = numUnit(get(r, "pulses"))
				out = append(out, it)
				n++
			}
		}
		// evento (o "Current device parameters" é a foto dos raros)
		ev := get(r, "event")
		code, codeErr := strconv.Atoi(get(r, "event id"))
		isParams := strings.Contains(strings.ToLower(ev), "current device parameters")
		if n == 0 && !isParams && (ev != "" || codeErr == nil) {
			it := base
			it.Kind = "evento"
			if c, ok := mobileEvents[strings.ToLower(ev)]; ok {
				code, codeErr = c, nil
			}
			if codeErr == nil && code >= 0 && code <= 22 && ev != "" {
				if _, known := mobileEvents[strings.ToLower(ev)]; known {
					it.Event, it.Code = radiacode.EventName(code), code
				} else {
					it.Event, it.Code = ev, code
				}
			} else {
				it.Event, it.Code = ev, code
			}
			it.Param, _ = strconv.Atoi(get(r, "event param. 1"))
			if it.Event != "" {
				out = append(out, it)
				n++
			}
		}
		// raros: temperatura, bateria, dose acumulada
		if n == 0 {
			it := base
			it.Kind = "raro"
			if v, _, ok := numUnit(get(r, "temp.")); ok {
				it.Temp, it.HasTemp = v, true
			}
			ch := get(r, "charge")
			if v, _, ok := numUnit(ch); ok {
				it.Battery, it.HasBat = v, true
			}
			if strings.Contains(strings.ToLower(ch), "chrg") || strings.Contains(strings.ToLower(ch), "charg") {
				it.Power = "carregando"
			} else if strings.Contains(strings.ToLower(ch), "ext") {
				it.Power = "externa"
			}
			if v, u, ok := numU(r, "dose"); ok {
				if usv, ok := toUSv(v, u); ok {
					it.Dose, it.HasDose = usv, true
				}
			}
			it.DoseSec, _ = durSec(get(r, "duration of d.a."))
			if it.HasTemp || it.HasBat || it.HasDose || it.Power != "" {
				out = append(out, it)
				n++
			}
		}
		if n == 0 {
			skipped++
		}
	}
	if len(out) == 0 {
		return nil, skipped, errors.New("nenhum registro reconhecido no arquivo")
	}
	return out, skipped, nil
}

// mobileTime lê "2026.11.07  01.25.27" (hora local) ou, se não der, o
// Timestamp em hex (FILETIME do Windows, UTC).
func mobileTime(real, ts string) (time.Time, bool) {
	if m := reRealT.FindStringSubmatch(real); m != nil {
		v := make([]int, 6)
		for i := range v {
			v[i], _ = strconv.Atoi(m[i+1])
		}
		ms := 0
		if rest := real[len(m[0]):]; len(rest) > 1 && rest[0] == '.' { // "18:13:41.491"
			frac := rest[1:]
			for len(frac) < 3 {
				frac += "0"
			}
			ms, _ = strconv.Atoi(frac[:3])
		}
		return time.Date(v[0], time.Month(v[1]), v[2], v[3], v[4], v[5], ms*1e6, time.Local), true
	}
	// FILETIME (intervalos de 100 ns desde 1601): em decimal (exportação do app
	// de PC) ou em hexadecimal (app do celular)
	const epoch, maxFT = 116444736000000000, 200000000000000000
	if ft, err := strconv.ParseUint(ts, 10, 64); err == nil && ft > epoch && ft < maxFT {
		// nessa exportação o número é a hora local (bate com a coluna Time)
		u := time.Unix(0, int64(ft-epoch)*100).UTC()
		return time.Date(u.Year(), u.Month(), u.Day(), u.Hour(), u.Minute(), u.Second(), u.Nanosecond(), time.Local), true
	}
	if ft, err := strconv.ParseUint(ts, 16, 64); err == nil && ft > epoch && ft < maxFT {
		return time.Unix(0, int64(ft-epoch)*100), true
	}
	return time.Time{}, false
}
