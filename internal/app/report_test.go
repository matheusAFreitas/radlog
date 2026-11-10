package app

import (
	"encoding/json"
	"math"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"radlog/internal/store"
)

func TestRenderReport(t *testing.T) {
	base := time.Now().Truncate(time.Hour).Add(-48 * time.Hour)
	a, _ := testApp(t)
	fillApp(t, a, base, 0, 600, "A")
	T := base.Add(2 * time.Hour).UnixMilli()
	a.events = append(a.events,
		Event{ID: 1, Time: T, End: T + 600000, Kind: kindAlert, Level: "atencao", Peak: 70},
		Event{ID: 2, Time: T + 300000, End: T + 500000, Kind: kindAlert, Level: "alarme", Peak: 134},
	)
	r, err := a.buildReport(base.UnixMilli(), base.Add(24*time.Hour).UnixMilli(), "teste")
	if err != nil {
		t.Fatal(err)
	}
	if r.Alerts != "1 alarme" || len(r.AlertList) != 1 || !strings.HasPrefix(r.AlertList[0].Peak, fmtU(r.uAt(time.UnixMilli(T), 134))+" µSv/h") {
		t.Fatalf("alertas: %q %+v", r.Alerts, r.AlertList)
	}
	b := renderReportPDF(r)
	if !strings.HasPrefix(string(b), "%PDF-1.4") || !strings.Contains(string(b), "%%EOF") {
		t.Fatal("não parece um PDF")
	}
}

func TestUniquePath(t *testing.T) {
	dir := t.TempDir()
	p := filepath.Join(dir, "r.pdf")
	if uniquePath(p) != p {
		t.Fatal("arquivo novo mudou de nome")
	}
	_ = os.WriteFile(p, []byte("x"), 0o644)
	if q := uniquePath(p); q != filepath.Join(dir, "r-2.pdf") {
		t.Fatalf("esperava r-2.pdf, veio %s", q)
	}
}

// O caso que o Codex achou: maior minuto gravado 114, pico ao vivo do alarme
// 124. O máximo do resumo e o da tabela por dia têm de ser o mesmo número.
func TestReportMaxConsistent(t *testing.T) {
	base := time.Now().Truncate(time.Hour).Add(-30 * time.Hour)
	a, _ := testApp(t)
	var pts []store.Point
	for m := 0; m < 24*60; m++ {
		v := 20.0 + float64(m%7)
		if m >= 300 && m < 305 {
			v = []float64{60, 98, 114, 90, 40}[m-300]
		}
		pts = append(pts, store.Point{T: base.Add(time.Duration(m) * time.Minute), CPM: v, Src: store.SrcLive})
	}
	if _, err := a.store.ImportNew(pts); err != nil {
		t.Fatal(err)
	}
	T := base.Add(300 * time.Minute).Add(25 * time.Second)
	peakAt := T.Add(150 * time.Second)
	a.events = append(a.events,
		Event{ID: 1, Time: T.UnixMilli(), End: T.Add(277 * time.Second).UnixMilli(), Kind: kindAlert, Level: "atencao", Peak: 124, PeakAt: peakAt.UnixMilli()},
		Event{ID: 2, Time: T.Add(70 * time.Second).UnixMilli(), End: T.Add(200 * time.Second).UnixMilli(), Kind: kindAlert, Level: "alarme", Peak: 124, PeakAt: peakAt.UnixMilli()},
	)
	// segundos gravados do alerta
	var secs []AlertPoint
	for s := -120; s < 400; s++ {
		v := 25.0
		if s >= 0 && s < 280 {
			v = 30 + 94*math.Sin(math.Pi*float64(s)/280)
		}
		secs = append(secs, AlertPoint{T: T.Add(time.Duration(s) * time.Second).UnixMilli(), CPM: math.Round(v)})
	}
	_ = os.MkdirAll(a.alertDir(), 0o755)
	b, _ := json.Marshal(secs)
	_ = os.WriteFile(filepath.Join(a.alertDir(), alertKey(a.events[0])), b, 0o644)
	a.measurements = []SavedMeasure{{ID: 1, Name: "Eletrodo de tungstênio toriado", Time: base.Add(600 * time.Minute).UnixMilli(),
		Bg:     MeasurePhase{Counts: 1540, Seconds: 60, CPM: 25.7, ErrCPM: 0.7},
		Obj:    MeasurePhase{Counts: 6200, Seconds: 60, CPM: 103.3, ErrCPM: 1.3},
		Result: MeasureResult{NetCPM: 77.6, NetErr: 1.5, NetUSv: 0.78, Ratio: 4.02, Sigma: 52, Verdict: "Radioativo: bem acima do fundo"}}}

	r, err := a.buildReport(base.UnixMilli(), base.Add(24*time.Hour).UnixMilli(), "teste")
	if err != nil {
		t.Fatal(err)
	}
	if r.St.Max != 114 || r.AlertPeakMax != 124 || r.Max != "124" || !r.MaxLive {
		t.Fatalf("máx %v pico %v resumo %q live %v", r.St.Max, r.AlertPeakMax, r.Max, r.MaxLive)
	}
	var day reportDay
	for _, d := range r.Days {
		if d.Max >= 114 {
			day = d
		}
	}
	if day.Max != 114 || day.LivePeak != 124 {
		t.Fatalf("dia: %+v", day)
	}
	if len(r.Episodes) != 1 || r.Episodes[0].DurSec != 277 || !r.Episodes[0].Chart || len(r.Episodes[0].Secs) == 0 || len(r.Episodes[0].Around) < 60 {
		t.Fatalf("episódio: %+v", r.Episodes)
	}
	if len(r.BgRows) != 3 {
		t.Fatalf("fundo: %+v", r.BgRows)
	}
	pdfb := renderReportPDF(r)
	if out := os.Getenv("REPORT_OUT"); out != "" {
		_ = os.WriteFile(out, pdfb, 0o644)
	}
	a.SetUILang("en")
	enb := a.inLang(func() []byte { return renderReportPDF(r) })
	if !strings.HasPrefix(string(enb), "%PDF") || string(enb) == string(pdfb) {
		t.Fatal("PDF em inglês igual ao português")
	}
	if out := os.Getenv("REPORT_OUT"); out != "" {
		_ = os.WriteFile(strings.TrimSuffix(out, ".pdf")+"-en.pdf", enb, 0o644)
	}
}

func TestFmtSeconds(t *testing.T) {
	for in, want := range map[int]string{0: "0 s", 45: "45 s", 277: "4 min 37 s", 3725: "1 h 02 min 05 s"} {
		if got := fmtSeconds(in); got != want {
			t.Fatalf("%d: %q", in, got)
		}
	}
}

// Relatórios gravados ao mesmo tempo com o mesmo nome não se sobrescrevem:
// cada um fica com o seu arquivo (r.pdf, r-2.pdf, ...).
func TestWriteReportFileConcurrent(t *testing.T) {
	a := newTestApp(t)
	want := filepath.Join(t.TempDir(), "r.pdf")
	const n = 8
	got := make(chan string, n)
	for i := 0; i < n; i++ {
		go func(i int) {
			p, err := a.writeReportFile(want, []byte{byte('a' + i)})
			if err != nil {
				t.Error(err)
			}
			got <- p
		}(i)
	}
	seen := map[string]bool{}
	for i := 0; i < n; i++ {
		seen[<-got] = true
	}
	if len(seen) != n {
		t.Fatalf("esperava %d arquivos diferentes, vieram %d: %v", n, len(seen), seen)
	}
}

// Um segundo pedido de relatório enquanto o primeiro gera volta na hora.
func TestExportReportBusy(t *testing.T) {
	a := newTestApp(t)
	a.reportBusy.Lock()
	defer a.reportBusy.Unlock()
	if _, err := a.ExportReport(0, 1, "x"); err != errReportBusy {
		t.Fatalf("esperava errReportBusy, veio %v", err)
	}
}
