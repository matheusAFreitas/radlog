package app

import (
	"math"
	"os"
	"strings"
	"testing"
	"time"

	"radlog/internal/store"
)

func TestReportNewPages(t *testing.T) {
	a := newTestApp(t)
	now := time.Now().Truncate(time.Minute)
	start := now.Add(-2 * 24 * time.Hour)
	// 2 dias: GMC em casa e o RC102 com você; das 13 às 17 h de ontem, fora
	leave := time.Date(now.Year(), now.Month(), now.Day(), 13, 0, 0, 0, now.Location()).AddDate(0, 0, -1)
	back := leave.Add(4 * time.Hour)
	var pts []store.Point
	for m := start; m.Before(now); m = m.Add(time.Minute) {
		v := 0.12 + 0.01*math.Sin(float64(m.Unix())/3000)
		if !m.Before(leave) && m.Before(back) {
			v = 0.17
		}
		a.rcAddMinute(m.Add(time.Second), 3, v)
		pts = append(pts, store.Point{T: m, CPM: 20 + float64(m.Minute()%5), Src: "live"})
	}
	a.rcFlushMinute()
	a.store.Import(pts)
	// espectro: em casa de minuto em minuto, e a linha da volta cobrindo a saída
	acc := make([]float64, 1024)
	c := make([]uint32, 1024)
	sec := 0.0
	add := func(min, k40 float64) {
		for i := range acc {
			acc[i] += 40 / (1 + float64(i)/40) * min
		}
		for d := -8; d <= 8; d++ {
			acc[487+d] += k40 * min * math.Exp(-float64(d*d)/18)
		}
		for i := range c {
			c[i] = uint32(acc[i])
		}
		sec += min * 60
	}
	t0 := leave.Add(-6 * time.Hour)
	a.rcSpecAdd("RC-1", t0, fakeSpec(sec, c))
	for i := 1; i <= 360; i++ {
		add(1, 3)
		a.rcSpecAdd("RC-1", t0.Add(time.Duration(i)*time.Minute), fakeSpec(sec, c))
	}
	add(240, 6)
	a.rcSpecAdd("RC-1", back.Add(3*time.Minute), fakeSpec(sec, c))
	if _, err := a.SavePeriod(Period{From: leave.UnixMilli(), To: back.UnixMilli(), Name: "Centro", GMC: WhereHome, RC: WhereMe, NotHome: true}); err != nil {
		t.Fatal(err)
	}
	a.mu.Lock()
	a.settings.ReportNotes = true
	a.notes = append(a.notes, Note{ID: 1, Time: leave.Add(-time.Hour).UnixMilli(), Text: "abri a janela"})
	a.mu.Unlock()

	r, err := a.buildReport(start.UnixMilli(), now.UnixMilli(), "teste")
	if err != nil {
		t.Fatal(err)
	}
	if len(r.Trips) != 1 || r.Trips[0].Title != "Centro" || r.Trips[0].Spec == nil || !r.Trips[0].HasDose {
		t.Fatalf("saídas: %+v", r.Trips)
	}
	if len(r.Days2) < 2 || !r.Days2[1].HasDose || !r.Days2[1].HasHome {
		t.Fatalf("por dia: %+v", r.Days2)
	}
	if !r.Verdict.OK || r.Verdict.Level != "normal" {
		t.Fatalf("veredito: %+v", r.Verdict)
	}
	if len(r.Notes) != 1 || !r.Opts.Cfg || r.Opts.Summary {
		t.Fatalf("opções: %+v notas %d", r.Opts, len(r.Notes))
	}
	full := renderReportPDF(r)
	if !strings.Contains(string(full), "Anexo") && !strings.HasPrefix(string(full), "%PDF") {
		t.Fatal("PDF")
	}
	r.Opts.Summary = true
	r.pages = 0
	short := renderReportPDF(r)
	if len(short) >= len(full) {
		t.Fatalf("só o resumo (%d) não é menor que o completo (%d)", len(short), len(full))
	}
	if out := os.Getenv("REPORT2_OUT"); out != "" {
		_ = os.WriteFile(out, full, 0o644)
		_ = os.WriteFile(strings.TrimSuffix(out, ".pdf")+"-resumo.pdf", short, 0o644)
	}
}

func TestDoseVerdict(t *testing.T) {
	rc := &RcSum{Minutes: 600, Dose: 1.0, BgUSv: 0.1}
	for _, c := range []struct {
		dose float64
		lv   string
	}{{1.2, "normal"}, {1.5, "acima"}, {2.5, "alto"}} {
		rc.Dose = c.dose
		if v := doseVerdict(rc, 10*time.Hour); !v.OK || v.Level != c.lv || math.Abs(v.Expected-1) > 1e-9 {
			t.Fatalf("%v: %+v", c.dose, v)
		}
	}
	if v := doseVerdict(&RcSum{Minutes: 10, Dose: 1}, time.Hour); v.OK {
		t.Fatal("sem fundo de casa não tem veredito")
	}
}
