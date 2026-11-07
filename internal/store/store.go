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
}

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

// Add grava a leitura de um minuto feita ao vivo.
func (s *Store) Add(p Point) error {
	p.T = p.T.Truncate(time.Minute)
	s.mu.Lock()
	defer s.mu.Unlock()
	if n := len(s.cache); n > 0 && !s.cache[n-1].T.Before(p.T) {
		return nil // já existe esse minuto (ou o relógio voltou)
	}
	if err := s.appendLine(p); err != nil {
		return err
	}
	s.cache = append(s.cache, p)
	s.trim()
	return nil
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
		existing, err := readFile(path)
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
	s.cache = pts2
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
		return err
	}
	if err := f.Close(); err != nil {
		return err
	}
	return os.Rename(tmp, path)
}

func readFile(path string) ([]Point, error) {
	f, err := os.Open(path)
	if errors.Is(err, fs.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	defer f.Close()
	return ParseCSV(f), nil
}

// ParseCSV lê pontos no formato dos arquivos de leituras (para importar um
// backup). Linhas quebradas são ignoradas.
func ParseCSV(rd io.Reader) []Point {
	r := csv.NewReader(rd)
	r.FieldsPerRecord = -1
	var out []Point
	for {
		rec, err := r.Read()
		if err == io.EOF {
			break
		}
		if err != nil {
			return out // linha quebrada no fim (queda de energia): ignora o resto
		}
		if len(rec) < 2 || strings.HasPrefix(rec[0], "data") {
			continue
		}
		t, err1 := time.Parse(time.RFC3339, rec[0])
		v, err2 := strconv.ParseFloat(rec[1], 64)
		if err1 != nil || err2 != nil {
			continue
		}
		src := SrcLive
		if len(rec) > 2 {
			src = rec[2]
		}
		p := Point{T: t.Local(), CPM: v, Src: src}
		if len(rec) >= 7 && rec[3] != "" {
			raw, e1 := strconv.ParseFloat(rec[3], 64)
			corr, e2 := strconv.ParseFloat(rec[4], 64)
			if e1 == nil && e2 == nil {
				p.Has, p.Raw, p.Corr = true, raw, corr
				p.DTOn = rec[6] == "ligado"
				p.TauUs, _ = strconv.ParseFloat(rec[5], 64)
				if len(rec) > 7 {
					p.Obs = rec[7]
				}
			}
		}
		out = append(out, p)
	}
	return out
}

// load lê dos arquivos os pontos em [from, to).
func (s *Store) load(from, to time.Time) ([]Point, error) {
	var out []Point
	m := time.Date(from.Year(), from.Month(), 1, 0, 0, 0, 0, from.Location())
	for !m.After(to) {
		pts, err := readFile(s.monthFile(m))
		if err != nil {
			return nil, err
		}
		for _, p := range pts {
			if !p.T.Before(from) && p.T.Before(to) {
				out = append(out, p)
			}
		}
		m = m.AddDate(0, 1, 0)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].T.Before(out[j].T) })
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

// Range devolve os pontos em [from, to).
func (s *Store) Range(from, to time.Time) ([]Point, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if !from.Before(s.cacheMin) {
		i := sort.Search(len(s.cache), func(i int) bool { return !s.cache[i].T.Before(from) })
		j := sort.Search(len(s.cache), func(i int) bool { return !s.cache[i].T.Before(to) })
		return append([]Point(nil), s.cache[i:j]...), nil
	}
	return s.load(from, to)
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
