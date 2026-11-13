package app

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"

	"radlog/internal/gmc"
	"radlog/internal/store"
)

// um ano de minutos dos dois aparelhos (dados sintéticos)
func yearApp(tb testing.TB) (*App, time.Time) {
	tb.Helper()
	dir := tb.TempDir()
	now := time.Date(2026, 11, 13, 12, 0, 0, 0, time.Local)
	start := now.AddDate(-1, 0, 0)
	_ = os.MkdirAll(filepath.Join(dir, "leituras"), 0o755)
	_ = os.MkdirAll(filepath.Join(dir, rcDirName), 0o755)
	files := map[string]*bufio.Writer{}
	var fs []*os.File
	w := func(path, head string) *bufio.Writer {
		if b, ok := files[path]; ok {
			return b
		}
		f, _ := os.Create(path)
		fs = append(fs, f)
		b := bufio.NewWriterSize(f, 1<<20)
		b.WriteString(head + "\n")
		files[path] = b
		return b
	}
	for t := start; t.Before(now); t = t.Add(time.Minute) {
		m := t.Format("2006-01")
		g := w(filepath.Join(dir, "leituras", m+".csv"), "data_hora,cpm,origem,cpm_bruto,cpm_corrigido,tau_us,tm,obs")
		fmt.Fprintf(g, "%s,%d,ao vivo\n", t.Format(time.RFC3339), 18+t.Minute()%7)
		r := w(filepath.Join(dir, rcDirName, m+".csv"), rcHeader)
		fmt.Fprintf(r, "%s,%.2f,%.4f,60\n", t.Format(time.RFC3339), 3.5, 0.1)
	}
	for _, b := range files {
		b.Flush()
	}
	for _, f := range fs {
		f.Close()
	}
	st, err := store.Open(dir)
	if err != nil {
		tb.Fatal(err)
	}
	a := NewApp()
	a.dataDir, a.store, a.cal = dir, st, gmc.DefaultCalibration()
	a.settings = defaultSettings()
	tb.Cleanup(a.rcSpecClose)
	tb.Cleanup(a.flushWrites)
	return a, now
}

func BenchmarkMyDoseYear(b *testing.B) {
	a, now := yearApp(b)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if _, err := a.myDose(now, "publico"); err != nil {
			b.Fatal(err)
		}
	}
}
