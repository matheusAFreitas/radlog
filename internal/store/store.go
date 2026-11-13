// Package store guarda as leituras por minuto em arquivos CSV mensais
// (um arquivo por mês, fácil de abrir no Excel).
package store

import (
	"bufio"
	"encoding/csv"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"
)

// Origem de cada ponto.
const (
	SrcLive   = "ao vivo"  // lido pelo app enquanto conectado
	SrcDevice = "contador" // importado da memória do contador
)

// Point é a contagem de um minuto (ou de uma hora, se veio do contador
// gravando por hora).
//
// Desde a 2.1 cada minuto guarda também de onde o CPM veio: o CPM bruto (o
// que o contador mandou), o corrigido do tempo morto, o τ usado e se a
// correção estava ligada, e marcas (Obs). CPM continua sendo o valor usado
// em tudo (o corrigido, com a correção ligada). Minutos gravados antes não
// têm essas colunas (Has = false) e não são reescritos com elas.
type Point struct {
	T   time.Time
	CPM float64
	Src string

	Has   bool    // tem as colunas da 2.1
	Raw   float64 // CPM bruto
	Corr  float64 // CPM corrigido do tempo morto (igual ao bruto com a correção desligada)
	TauUs float64 // τ usado (µs); 0 com a correção desligada
	DTOn  bool    // correção de tempo morto ligada
	Obs   string  // marcas separadas por "+": seg, sat, interf
}

// Marcas de um minuto (Point.Obs).
const (
	ObsSeconds      = "seg"    // veio de gravações por segundo da memória do contador
	ObsSaturation   = "sat"    // o contador saturou nesse minuto
	ObsInterference = "interf" // segundo suspeito de interferência elétrica nesse minuto
)

// HasObs diz se o minuto tem a marca o.
func (p Point) HasObs(o string) bool {
	for _, x := range strings.Split(p.Obs, "+") {
		if x == o {
			return true
		}
	}
	return false
}

// RawCPM é o CPM bruto (nos minutos antigos, o próprio CPM: até a 2.0 a
// correção de tempo morto nunca foi usada).
func (p Point) RawCPM() float64 {
	if p.Has {
		return p.Raw
	}
	return p.CPM
}

// Store mantém os últimos dias em memória e grava tudo em disco.
type Store struct {
	dir string
	mu  sync.Mutex
	// cache ordenado por tempo com os dias recentes
	cache    []Point
	cacheMin time.Time
	// minutos ao vivo que não gravaram (arquivo do mês aberto no Excel, por
	// exemplo): ficam no cache e são gravados no próximo Add ou no Flush (B2)
	pending []Point
}

// ErrNotSaved diz que o minuto não foi para o disco agora: ficou na memória
// e o app tenta de novo no minuto seguinte.
var ErrNotSaved = errors.New("leitura não gravada no disco")

const keepDays = 8

// Open abre (ou cria) a pasta de dados e carrega os dias recentes.
func Open(dir string) (*Store, error) {
	if err := os.MkdirAll(filepath.Join(dir, "leituras"), 0o755); err != nil {
		return nil, err
	}
	s := &Store{dir: dir}
	s.cacheMin = startOfDay(time.Now()).AddDate(0, 0, -keepDays)
	pts, err := s.load(s.cacheMin, time.Now().Add(24*time.Hour))
	if err != nil {
		return nil, err
	}
	s.cache = pts
	s.mu.Lock()
	s.loadPendingLocked()
	s.mu.Unlock()
	return s, nil
}

// Dir devolve a pasta de dados.
func (s *Store) Dir() string { return s.dir }

func startOfDay(t time.Time) time.Time {
	y, m, d := t.Date()
	return time.Date(y, m, d, 0, 0, 0, 0, t.Location())
}

func (s *Store) monthFile(t time.Time) string {
	return filepath.Join(s.dir, "leituras", t.Format("2006-01")+".csv")
}

const header = "data_hora,cpm,origem,cpm_bruto,cpm_corrigido,tau_us,tm,obs"

// Add grava a leitura de um minuto feita ao vivo. Se o arquivo não deixar
// gravar, o minuto fica na memória (e nos gráficos) e o erro é ErrNotSaved;
// o próximo Add tenta gravar os que ficaram, na ordem.
func (s *Store) Add(p Point) error {
	p.T = p.T.Truncate(time.Minute)
	s.mu.Lock()
	defer s.mu.Unlock()
	// já gravado: compara com o último minuto ao vivo, não com o histórico do
	// contador (um relógio adiantado deixava minutos no futuro e a gravação
	// ao vivo descartava tudo até a hora passar deles, B1)
	if last, ok := s.lastLiveLocked(); ok && !last.Before(p.T) {
		return nil
	}
	i := sort.Search(len(s.cache), func(i int) bool { return !s.cache[i].T.Before(p.T) })
	if i < len(s.cache) && s.cache[i].T.Equal(p.T) {
		return nil // o histórico já trouxe esse minuto
	}
	var err error
	if s.flushLocked() == nil {
		err = s.appendLine(p)
	} else {
		err = errors.New("ainda há minutos esperando")
	}
	if err != nil {
		s.pending = append(s.pending, p)
		err = fmt.Errorf("%w: %v", ErrNotSaved, err)
	}
	s.cache = append(s.cache, Point{})
	copy(s.cache[i+1:], s.cache[i:])
	s.cache[i] = p
	s.trim()
	return err
}

// withPending põe no cache relido do disco os minutos que ainda não gravaram.
func withPending(cache, pending []Point) []Point {
	for _, p := range pending {
		i := sort.Search(len(cache), func(i int) bool { return !cache[i].T.Before(p.T) })
		if i < len(cache) && cache[i].T.Equal(p.T) {
			continue
		}
		cache = append(cache, Point{})
		copy(cache[i+1:], cache[i:])
		cache[i] = p
	}
	return cache
}

// lastLiveLocked é a hora do último minuto ao vivo no cache.
func (s *Store) lastLiveLocked() (time.Time, bool) {
	for i := len(s.cache) - 1; i >= 0; i-- {
		if s.cache[i].Src == SrcLive {
			return s.cache[i].T, true
		}
	}
	return time.Time{}, false
}

// flushLocked grava os minutos que ficaram esperando, na ordem; para no
// primeiro que falhar.
func (s *Store) flushLocked() error {
	for len(s.pending) > 0 {
		if err := s.appendLine(s.pending[0]); err != nil {
			return err
		}
		s.pending = s.pending[1:]
	}
	s.pending = nil
	return nil
}

// pendingFile guarda os minutos que não deu para gravar no mês quando o
// app fechou (o CSV ainda aberto no Excel). Na próxima abertura eles voltam
// e vão para o mês (QA 13/11).
const pendingFile = "leituras-pendentes.csv"

// Flush tenta gravar os minutos que ficaram esperando (ao fechar o app e a
// cada minuto). Se não der, ao fechar (keep) eles vão para pendingFile.
func (s *Store) Flush() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.flushLocked()
}

// FlushOrKeep é o Flush do fechamento: o que não gravar no mês fica em
// pendingFile, para a próxima abertura.
func (s *Store) FlushOrKeep() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	err := s.flushLocked()
	if err == nil || len(s.pending) == 0 {
		return err
	}
	if kerr := writeFile(filepath.Join(s.dir, pendingFile), s.pending); kerr != nil {
		return fmt.Errorf("%v; e não deu para guardar em %s: %v", err, pendingFile, kerr)
	}
	return nil
}

// loadPendingLocked traz de volta os minutos guardados no último fechamento.
func (s *Store) loadPendingLocked() {
	path := filepath.Join(s.dir, pendingFile)
	pts, err := readFile(path)
	if err != nil || len(pts) == 0 {
		return
	}
	s.pending = append(s.pending, pts...)
	s.cache = withPending(s.cache, pts)
	if s.flushLocked() == nil {
		os.Remove(path)
	}
}

// Pending diz quantos minutos estão na memória esperando para gravar e o
// arquivo do primeiro.
func (s *Store) Pending() (int, string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if len(s.pending) == 0 {
		return 0, ""
	}
	return len(s.pending), filepath.Base(s.monthFile(s.pending[0].T))
}

func (s *Store) appendLine(p Point) error {
	path := s.monthFile(p.T)
	_, statErr := os.Stat(path)
	f, err := os.OpenFile(path, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o644)
	if err != nil {
		return err
	}
	defer f.Close()
	w := bufio.NewWriter(f)
	if errors.Is(statErr, fs.ErrNotExist) {
		fmt.Fprintln(w, header)
	} else if !endsWithNewline(path) {
		// a última gravação foi cortada (queda de energia): começa linha nova
		fmt.Fprintln(w)
	}
	fmt.Fprintln(w, formatLine(p))
	return w.Flush()
}

func endsWithNewline(path string) bool {
	f, err := os.Open(path)
	if err != nil {
		return true
	}
	defer f.Close()
	st, err := f.Stat()
	if err != nil || st.Size() == 0 {
		return true
	}
	b := make([]byte, 1)
	if _, err := f.ReadAt(b, st.Size()-1); err != nil {
		return true
	}
	return b[0] == '\n'
}

func formatLine(p Point) string {
	l := p.T.Format(time.RFC3339) + "," + strconv.FormatFloat(p.CPM, 'f', -1, 64) + "," + p.Src
	if !p.Has {
		return l // minuto de antes da 2.1: fica como estava
	}
	tm, tau := "desligado", ""
	if p.DTOn {
		tm, tau = "ligado", strconv.FormatFloat(p.TauUs, 'f', -1, 64)
	}
	return l + "," + strconv.FormatFloat(p.Raw, 'f', -1, 64) + "," + strconv.FormatFloat(p.Corr, 'f', -1, 64) + "," + tau + "," + tm + "," + p.Obs
}

func (s *Store) trim() {
	cut := startOfDay(time.Now()).AddDate(0, 0, -keepDays)
	if !cut.After(s.cacheMin) {
		return
	}
	i := sort.Search(len(s.cache), func(i int) bool { return !s.cache[i].T.Before(cut) })
	s.cache = append([]Point(nil), s.cache[i:]...)
	s.cacheMin = cut
}

// Import junta pontos vindos do contador. Minutos que já existem não são
// substituídos. Devolve quantos pontos novos entraram.
func (s *Store) Import(pts []Point) (int, error) {
	added, err := s.ImportNew(pts)
	return len(added), err
}

// ImportNew é o Import que devolve os pontos que entraram de fato.
func (s *Store) ImportNew(pts []Point) ([]Point, error) {
	pts, _, _ = SaneTimes(pts, time.Now())
	if len(pts) == 0 {
		return nil, nil
	}
	s.mu.Lock()
	defer s.mu.Unlock()

	byMonth := map[string][]Point{}
	for _, p := range pts {
		p.T = p.T.Truncate(time.Minute)
		byMonth[p.T.Format("2006-01")] = append(byMonth[p.T.Format("2006-01")], p)
	}
	var added []Point
	for _, list := range byMonth {
		path := s.monthFile(list[0].T)
		existing, bad, err := readFileBad(path)
		if err != nil {
			return added, err
		}
		have := make(map[int64]bool, len(existing))
		for _, p := range existing {
			have[p.T.Unix()] = true
		}
		merged := existing
		for _, p := range list {
			if !have[p.T.Unix()] {
				have[p.T.Unix()] = true
				merged = append(merged, p)
				added = append(added, p)
			}
		}
		if len(merged) == len(existing) {
			continue
		}
		sort.Slice(merged, func(i, j int) bool { return merged[i].T.Before(merged[j].T) })
		// o arquivo vai ser regravado só com o que foi lido: as linhas
		// ilegíveis vão antes para um arquivo ao lado
		if err := keepBad(path, bad); err != nil {
			return added, err
		}
		if err := writeFile(path, merged); err != nil {
			return added, err
		}
	}
	sort.Slice(added, func(i, j int) bool { return added[i].T.Before(added[j].T) })
	// recarrega o cache
	pts2, err := s.load(s.cacheMin, time.Now().Add(24*time.Hour))
	if err != nil {
		return added, err
	}
	s.cache = withPending(pts2, s.pending)
	return added, nil
}

// Earliest devolve o primeiro mês que tem arquivo (zero se não houver nada).
func (s *Store) Earliest() time.Time {
	files, _ := filepath.Glob(filepath.Join(s.dir, "leituras", "*.csv"))
	var first time.Time
	for _, f := range files {
		m, err := time.ParseInLocation("2006-01", strings.TrimSuffix(filepath.Base(f), ".csv"), time.Local)
		if err != nil {
			continue
		}
		if first.IsZero() || m.Before(first) {
			first = m
		}
	}
	return first
}

func writeFile(path string, pts []Point) error {
	tmp := path + ".tmp"
	f, err := os.Create(tmp)
	if err != nil {
		return err
	}
	w := bufio.NewWriter(f)
	fmt.Fprintln(w, header)
	for _, p := range pts {
		fmt.Fprintln(w, formatLine(p))
	}
	if err := w.Flush(); err != nil {
		f.Close()
		os.Remove(tmp)
		return err
	}
	// os dados no disco antes do rename: sem isso, numa queda de energia o
	// rename pode chegar primeiro e o mês ficar vazio (QA, 12/11)
	if err := f.Sync(); err != nil {
		f.Close()
		os.Remove(tmp)
		return err
	}
	if err := f.Close(); err != nil {
		os.Remove(tmp)
		return err
	}
	if err := RenameRetry(tmp, path); err != nil {
		os.Remove(tmp)
		return err
	}
	return nil
}

func readFile(path string) ([]Point, error) {
	pts, _, err := readFileBad(path)
	return pts, err
}

// readFileBad é o readFile que devolve também as linhas que não deu para ler.
func readFileBad(path string) ([]Point, []string, error) {
	f, err := os.Open(path)
	if errors.Is(err, fs.ErrNotExist) {
		return nil, nil, nil
	}
	if err != nil {
		return nil, nil, err
	}
	defer f.Close()
	hint := 0
	if st, err := f.Stat(); err == nil {
		hint = int(st.Size() / 60) // ~60 bytes por linha
	}
	pts, bad := parseCSVHint(f, hint)
	return pts, bad, nil
}

// ParseCSV lê pontos no formato dos arquivos de leituras (para importar um
// backup). Linhas quebradas são ignoradas, uma a uma.
func ParseCSV(rd io.Reader) []Point {
	pts, _ := parseCSV(rd)
	return pts
}

// maxLine é o maior tamanho de linha aceito; o resto de uma linha maior
// (lixo de uma gravação cortada) conta como linha ilegível.
const maxLine = 64 << 10

// parseCSV lê linha por linha e devolve os pontos e as linhas ilegíveis.
//
// Até a 2.70.0 o arquivo inteiro passava por um leitor de CSV só, que parava
// na primeira linha com erro (uma aspa solta, de uma gravação cortada por
// queda de energia): o resto do mês sumia da tela e, na importação seguinte,
// o arquivo era regravado sem ele. Agora cada linha é lida sozinha e uma
// linha ruim só perde a si mesma.
func parseCSV(rd io.Reader) (out []Point, bad []string) { return parseCSVHint(rd, 0) }

// parseCSVHint é o parseCSV com uma estimativa de quantas linhas vêm.
func parseCSVHint(rd io.Reader, hint int) (out []Point, bad []string) {
	out = make([]Point, 0, hint)
	br := bufio.NewReaderSize(rd, 64<<10)
	for {
		line, err := readLine(br)
		if t := strings.TrimSpace(strings.Trim(line, "\x00")); t != "" {
			if p, ok, skip := parseLine(t); ok {
				out = append(out, p)
			} else if !skip {
				bad = append(bad, t)
			}
		}
		if err != nil {
			return out, bad
		}
	}
}

// readLine lê uma linha de qualquer tamanho, guardando no máximo maxLine.
// O caso comum (linha que cabe no buffer) sai com uma alocação só.
func readLine(br *bufio.Reader) (string, error) {
	b, err := br.ReadSlice('\n')
	if err != bufio.ErrBufferFull {
		return strings.TrimRight(string(b), "\r\n"), err
	}
	var sb strings.Builder
	sb.Write(b)
	for {
		b, err = br.ReadSlice('\n')
		if sb.Len() < maxLine {
			sb.Write(b)
		}
		if err != bufio.ErrBufferFull {
			line := sb.String()
			if len(line) > maxLine {
				line = line[:maxLine]
			}
			return strings.TrimRight(line, "\r\n"), err
		}
	}
}

// splitLine separa os campos de uma linha. Os arquivos do app nunca têm
// aspas nem vírgula dentro de um campo; uma linha com aspas (editada no
// Excel, por exemplo) passa pelo leitor de CSV.
func splitLine(line string) ([]string, bool) {
	if !strings.Contains(line, `"`) {
		return strings.Split(line, ","), true
	}
	r := csv.NewReader(strings.NewReader(line))
	r.FieldsPerRecord = -1
	rec, err := r.Read()
	return rec, err == nil
}

// parseLine lê uma linha. skip = cabeçalho (não é ponto nem linha ruim).
func parseLine(line string) (p Point, ok, skip bool) {
	var rec [8]string
	n := 0
	if strings.Contains(line, `"`) {
		r, good := splitLine(line)
		if !good {
			return p, false, false
		}
		n = copy(rec[:], r)
	} else {
		// sem aspas: separa à mão, sem alocar (é o caminho de quase todas)
		rest := line
		for n < len(rec) {
			f, tail, more := strings.Cut(rest, ",")
			rec[n] = f
			n++
			if !more {
				break
			}
			rest = tail
		}
	}
	if n < 2 {
		return p, false, false
	}
	if strings.HasPrefix(rec[0], "data") {
		return p, false, true
	}
	t, ok1 := parseTime(rec[0])
	v, err2 := strconv.ParseFloat(rec[1], 64)
	if !ok1 || err2 != nil || !SaneCPM(v) {
		return p, false, false // NaN, infinito ou negativo vai para o .ilegivel
	}
	src := SrcLive
	if n > 2 {
		src = intern(rec[2])
	}
	p = Point{T: t, CPM: v, Src: src}
	if n >= 7 && rec[3] != "" {
		raw, e1 := strconv.ParseFloat(rec[3], 64)
		corr, e2 := strconv.ParseFloat(rec[4], 64)
		if e1 == nil && e2 == nil && SaneCPM(raw) && SaneCPM(corr) {
			p.Has, p.Raw, p.Corr = true, raw, corr
			p.DTOn = rec[6] == "ligado"
			if rec[5] != "" {
				p.TauUs, _ = strconv.ParseFloat(rec[5], 64)
			}
			if n > 7 {
				p.Obs = intern(rec[7])
			}
		}
	}
	return p, true, false
}

// intern devolve uma cópia compartilhada das origens e marcas mais comuns,
// para cada ponto não segurar a linha inteira na memória.
func intern(s string) string {
	switch s {
	case "":
		return ""
	case SrcLive:
		return SrcLive
	}
	internMu.Lock()
	defer internMu.Unlock()
	if v, ok := internMap[s]; ok {
		return v
	}
	v := strings.Clone(s)
	if len(internMap) < 256 {
		internMap[v] = v
	}
	return v
}

var (
	internMu  sync.Mutex
	internMap = map[string]string{}
)

// parseTime lê o horário das linhas (RFC 3339, com ou sem fuso) já no fuso
// do PC. O formato que o app grava é lido à mão, bem mais rápido que o
// time.Parse; qualquer outro passa por ele.
func parseTime(v string) (time.Time, bool) {
	if len(v) == 25 || (len(v) == 20 && v[19] == 'Z') {
		num := func(i, n int) (int, bool) {
			x := 0
			for _, c := range []byte(v[i : i+n]) {
				if c < '0' || c > '9' {
					return 0, false
				}
				x = x*10 + int(c-'0')
			}
			return x, true
		}
		y, a := num(0, 4)
		mo, b := num(5, 2)
		d, c := num(8, 2)
		h, e := num(11, 2)
		mi, f := num(14, 2)
		se, g := num(17, 2)
		if a && b && c && e && f && g && v[4] == '-' && v[7] == '-' && v[10] == 'T' && v[13] == ':' && v[16] == ':' &&
			mo >= 1 && mo <= 12 && d >= 1 && d <= 31 && h < 24 && mi < 60 && se < 60 {
			// 31/04 ou 29/02 fora de bissexto: o time.Date normalizaria para o
			// mês seguinte; o time.Parse recusa (a linha vai para o .ilegivel)
			if d > 28 && time.Date(y, time.Month(mo), d, 0, 0, 0, 0, time.UTC).Day() != d {
				goto slow
			}
			off := 0
			if len(v) == 25 {
				oh, ok1 := num(20, 2)
				om, ok2 := num(23, 2)
				if !ok1 || !ok2 || oh > 23 || om > 59 || v[22] != ':' || (v[19] != '+' && v[19] != '-') {
					goto slow
				}
				off = (oh*60 + om) * 60
				if v[19] == '-' {
					off = -off
				}
			}
			t := time.Date(y, time.Month(mo), d, h, mi, se, 0, time.UTC).Add(-time.Duration(off) * time.Second)
			return t.Local(), true
		}
	}
slow:
	t, err := time.Parse(time.RFC3339, v)
	if err != nil {
		return time.Time{}, false
	}
	return t.Local(), true
}

// keepBad guarda as linhas ilegíveis de um mês em <mês>.csv.ilegivel antes
// de o arquivo ser regravado, para nada ser jogado fora.
func keepBad(path string, bad []string) error {
	if len(bad) == 0 {
		return nil
	}
	f, err := os.OpenFile(path+".ilegivel", os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o644)
	if err != nil {
		return err
	}
	w := bufio.NewWriter(f)
	fmt.Fprintf(w, "# linhas que o RadLog não conseguiu ler, tiradas em %s\n", time.Now().Format(time.RFC3339))
	for _, l := range bad {
		fmt.Fprintln(w, l)
	}
	if err := w.Flush(); err != nil {
		f.Close()
		return err
	}
	return f.Close()
}

// load lê dos arquivos os pontos em [from, to).
func (s *Store) load(from, to time.Time) ([]Point, error) {
	var months [][]Point
	total := 0
	m := time.Date(from.Year(), from.Month(), 1, 0, 0, 0, 0, from.Location())
	for !m.After(to) {
		pts, err := readFile(s.monthFile(m))
		if err != nil {
			return nil, err
		}
		// só o pedaço do mês dentro de [from, to), sem copiar
		if sortedPts(pts) {
			i := sort.Search(len(pts), func(k int) bool { return !pts[k].T.Before(from) })
			j := sort.Search(len(pts), func(k int) bool { return !pts[k].T.Before(to) })
			pts = pts[i:j]
		} else {
			keep := pts[:0]
			for _, p := range pts {
				if !p.T.Before(from) && p.T.Before(to) {
					keep = append(keep, p)
				}
			}
			pts = keep
		}
		months = append(months, pts)
		total += len(pts)
		m = m.AddDate(0, 1, 0)
	}
	out := make([]Point, 0, total)
	for _, pts := range months {
		out = append(out, pts...)
	}
	if !sortedPts(out) {
		sort.SliceStable(out, func(i, j int) bool { return out[i].T.Before(out[j].T) })
	}
	// remove minutos repetidos (fica o primeiro)
	dedup := out[:0]
	for i, p := range out {
		if i > 0 && p.T.Equal(out[i-1].T) {
			continue
		}
		dedup = append(dedup, p)
	}
	return dedup, nil
}

func sortedPts(p []Point) bool {
	for i := 1; i < len(p); i++ {
		if p[i].T.Before(p[i-1].T) {
			return false
		}
	}
	return true
}

// Range devolve os pontos em [from, to). Os dias recentes vêm da memória;
// os de antes, dos arquivos, lidos sem segurar a trava (a gravação do minuto
// não espera uma consulta longa).
func (s *Store) Range(from, to time.Time) ([]Point, error) {
	var old []Point
	for {
		s.mu.Lock()
		cm := s.cacheMin
		if !from.Before(cm) {
			i := sort.Search(len(s.cache), func(i int) bool { return !s.cache[i].T.Before(from) })
			j := sort.Search(len(s.cache), func(i int) bool { return !s.cache[i].T.Before(to) })
			out := make([]Point, 0, len(old)+j-i)
			out = append(append(out, old...), s.cache[i:j]...)
			s.mu.Unlock()
			return out, nil
		}
		s.mu.Unlock()
		// os meses de antes: lidos fora da trava (os arquivos só são trocados
		// inteiros, por rename)
		end := to
		if cm.Before(end) {
			end = cm
		}
		pts, err := s.load(from, end)
		if err != nil {
			return nil, err
		}
		if old == nil {
			old = pts
		} else {
			old = append(old, pts...)
		}
		if !to.After(end) {
			return old, nil
		}
		from = end // o resto vem da memória (se a virada do dia mexeu nela, lê o que faltou)
	}
}

// ExportCSV escreve todos os pontos em [from, to) num único CSV. usv devolve
// o µSv/h do minuto e a calibração usada (vai na coluna "calibracao").
func (s *Store) ExportCSV(w io.Writer, from, to time.Time, usv func(Point) (float64, string)) error {
	pts, err := s.Range(from, to)
	if err != nil {
		return err
	}
	bw := bufio.NewWriter(w)
	fmt.Fprintln(bw, "data_hora,cpm,usv_h,origem,cpm_bruto,cpm_corrigido,tau_us,tm,obs,calibracao")
	for _, p := range pts {
		u, cal := usv(p)
		fmt.Fprintf(bw, "%s,%s,%s,%s", p.T.Format("2006-01-02 15:04"),
			strconv.FormatFloat(p.CPM, 'f', -1, 64),
			strconv.FormatFloat(u, 'f', 4, 64), p.Src)
		if p.Has {
			tm, tau := "desligado", ""
			if p.DTOn {
				tm, tau = "ligado", strconv.FormatFloat(p.TauUs, 'f', -1, 64)
			}
			fmt.Fprintf(bw, ",%s,%s,%s,%s,%s", strconv.FormatFloat(p.Raw, 'f', -1, 64), strconv.FormatFloat(p.Corr, 'f', -1, 64), tau, tm, p.Obs)
		} else {
			fmt.Fprint(bw, ",,,,sem registro (antes da 2.1),")
		}
		fmt.Fprintf(bw, ",%s\n", cal)
	}
	return bw.Flush()
}

// FirstTime devolve a data da leitura mais antiga guardada.
func (s *Store) FirstTime() (time.Time, bool) {
	entries, err := os.ReadDir(filepath.Join(s.dir, "leituras"))
	if err != nil {
		return time.Time{}, false
	}
	var names []string
	for _, e := range entries {
		if strings.HasSuffix(e.Name(), ".csv") {
			names = append(names, e.Name())
		}
	}
	sort.Strings(names)
	for _, n := range names {
		pts, err := readFile(filepath.Join(s.dir, "leituras", n))
		if err == nil && len(pts) > 0 {
			return pts[0].T, true
		}
	}
	return time.Time{}, false
}

// WriteCSV escreve pontos no mesmo formato dos arquivos (ParseCSV lê de volta).
func WriteCSV(w io.Writer, pts []Point) error {
	bw := bufio.NewWriter(w)
	fmt.Fprintln(bw, header)
	for _, p := range pts {
		fmt.Fprintln(bw, formatLine(p))
	}
	return bw.Flush()
}

// MonthFiles lista os arquivos de leituras (um por mês), com o tamanho e a
// hora da última mudança, para quem precisa saber se algo mudou.
func (s *Store) MonthFiles() ([]os.FileInfo, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	ents, err := os.ReadDir(filepath.Join(s.dir, "leituras"))
	if errors.Is(err, fs.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	var out []os.FileInfo
	for _, e := range ents {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".csv") {
			continue
		}
		if fi, err := e.Info(); err == nil {
			out = append(out, fi)
		}
	}
	return out, nil
}

// ReadMonthFile lê um arquivo de mês pelo nome (de MonthFiles).
func (s *Store) ReadMonthFile(name string) ([]Point, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return readFile(filepath.Join(s.dir, "leituras", filepath.Base(name)))
}

// Unopened é um Store para quando nem a pasta de dados nem a temporária
// abriram: as consultas voltam vazias e cada minuto fica esperando na
// memória (ErrNotSaved), em vez de o app quebrar (auditoria 2.76.0, D3).
func Unopened(dir string) *Store {
	return &Store{dir: dir, cacheMin: startOfDay(time.Now()).AddDate(0, 0, -keepDays)}
}

// ReadFileTolerant lê um arquivo de leituras de outra pasta (a temporária
// de uma sessão anterior); linhas estragadas ficam de fora.
func ReadFileTolerant(path string) ([]Point, error) { return readFile(path) }
