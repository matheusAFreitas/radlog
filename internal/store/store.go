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
type Point struct {
	T   time.Time
	CPM float64
	Src string
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

const header = "data_hora,cpm,origem"

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
	return p.T.Format(time.RFC3339) + "," + strconv.FormatFloat(p.CPM, 'f', -1, 64) + "," + p.Src
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
		out = append(out, Point{T: t.Local(), CPM: v, Src: src})
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

// ExportCSV escreve todos os pontos em [from, to) num único CSV.
func (s *Store) ExportCSV(w io.Writer, from, to time.Time, usv func(float64) float64) error {
	pts, err := s.Range(from, to)
	if err != nil {
		return err
	}
	bw := bufio.NewWriter(w)
	fmt.Fprintln(bw, "data_hora,cpm,usv_h,origem")
	for _, p := range pts {
		fmt.Fprintf(bw, "%s,%s,%s,%s\n", p.T.Format("2006-01-02 15:04"),
			strconv.FormatFloat(p.CPM, 'f', -1, 64),
			strconv.FormatFloat(usv(p.CPM), 'f', 4, 64), p.Src)
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
