package app

import (
	"bytes"
	"math"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"radlog/internal/radiacode"
	"radlog/internal/store"
)

func TestRcMinutesAndCompare(t *testing.T) {
	a := newTestApp(t)
	now := time.Now().Truncate(time.Minute)
	var pts []store.Point
	for i := 30; i >= 2; i-- {
		m := now.Add(-time.Duration(i) * time.Minute)
		pts = append(pts, store.Point{T: m, CPM: 20 + float64(i%2)*2, Src: "live"})
		// Radiacode: 2 leituras por minuto, µSv/h acompanhando o GMC
		usv := 0.10 + float64(i%2)*0.01
		a.rcAddMinute(m.Add(5*time.Second), 3.0, usv)
		a.rcAddMinute(m.Add(35*time.Second), 3.2, usv)
	}
	a.rcFlushMinute()
	if _, err := a.store.Import(pts); err != nil {
		t.Fatal(err)
	}
	got := a.rcRange(now.Add(-time.Hour), now)
	if len(got) != 29 || got[0].N != 2 || math.Abs(got[0].CPS-3.1) > 1e-9 {
		t.Fatalf("minutos gravados: %d %+v", len(got), got[0])
	}
	c, err := a.GetRcCompare(60)
	if err != nil {
		t.Fatal(err)
	}
	if c.Minutes != 29 || c.Ratio <= 0 || c.Corr < 0.99 {
		t.Fatalf("comparação: %+v", c)
	}
}

func TestRcRecordsState(t *testing.T) {
	a := newTestApp(t)
	a.rcRecords([]radiacode.Record{
		radiacode.RealTime{CPS: 5, DoseRate: 0.12, CPSErr: 10, DoseErr: 12},
		radiacode.Rare{Dose: 3.4, Duration: 7200, Battery: 87, TempC: 24.5},
	})
	st := a.GetRadiacode()
	if st.CPS != 5 || st.USvh != 0.12 || st.Dose != 3.4 || st.Battery != 87 || !st.HasRare {
		t.Fatalf("estado: %+v", st)
	}
}

func TestRcTodayAndChart(t *testing.T) {
	a := newTestApp(t)
	now := time.Now()
	start := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())
	if now.Sub(start) < 15*time.Minute {
		t.Skip("logo depois da meia-noite")
	}
	// 10 minutos a 0,6 µSv/h = 0,1 µSv
	for i := 12; i > 2; i-- {
		m := now.Truncate(time.Minute).Add(-time.Duration(i) * time.Minute)
		a.rcAddMinute(m.Add(time.Second), 3, 0.6)
	}
	a.rcFlushMinute()
	st := a.GetRadiacode()
	if math.Abs(st.Today-0.1) > 1e-6 || st.LastUSv != 0.6 || st.LastT == 0 {
		t.Fatalf("hoje: %+v", st)
	}
	c, err := a.GetChart("1h")
	if err != nil {
		t.Fatal(err)
	}
	k := 0
	for _, v := range c.RC {
		if v != nil {
			k++
			if *v != 0.6 {
				t.Fatalf("valor %v", *v)
			}
		}
	}
	if k != 10 || c.RCAvg == nil {
		t.Fatalf("curva: %d pontos", k)
	}
	// a mesma curva em CPM do aparelho (3 cps = 180 CPM)
	for i, v := range c.RCCPM {
		if (v == nil) != (c.RC[i] == nil) || (v != nil && math.Abs(*v-180) > 1e-9) {
			t.Fatalf("CPM do RC102 no ponto %d: %v", i, v)
		}
	}
	if c.RCAvgCPM == nil || math.Abs(*c.RCAvgCPM-180) > 1e-9 {
		t.Fatalf("média em CPM: %v", c.RCAvgCPM)
	}
	// período curto (passo de 5 s): minutos gravados viram pontos também
	c, _ = a.GetChart("15m")
	k = 0
	for _, v := range c.RC {
		if v != nil {
			k++
		}
	}
	if k < 100 {
		t.Fatalf("curva curta: %d pontos", k)
	}
}

func TestRcStats(t *testing.T) {
	a := newTestApp(t)
	now := time.Now().Truncate(time.Minute)
	for i := 90; i >= 1; i-- {
		v := 0.10
		if i%10 == 0 {
			v = 0.12
		}
		a.rcAddMinute(now.Add(-time.Duration(i)*time.Minute+time.Second), 3, v)
	}
	a.rcFlushMinute()
	st := a.GetRadiacode()
	s := st.Stats
	if s == nil || s.BgMinutes != 90 || s.BgUSv != 0.10 || s.BgHi != 0.10 || s.N1h < 59 || s.Max24h != 0.12 || s.Min24h != 0.10 {
		t.Fatalf("medidas: %+v", s)
	}
	if st.AttnUSv <= 0 || st.AlarmUSv <= st.AttnUSv {
		t.Fatalf("faixas: %v %v", st.AttnUSv, st.AlarmUSv)
	}
}

func TestRcRawAndBackup(t *testing.T) {
	a := newTestApp(t)
	now := time.Now().Truncate(time.Minute)
	a.rcAddMinute(now.Add(-3*time.Minute+time.Second), 3, 0.1)
	a.rcAddMinute(now.Add(-2*time.Minute+time.Second), 3, 0.1)
	a.rcFlushMinute()
	// dados brutos de uma versão antiga: ficam no disco, mas não vão mais no
	// backup (a gravação saiu na 2.70.3)
	os.MkdirAll(a.rcRawPath(""), 0o755)
	os.WriteFile(a.rcRawPath("2026-01-01-120000.jsonl.gz"), []byte("x"), 0o644)
	files := a.rcBackupFiles()
	if len(files) != 1 || strings.Contains(files[0], "bruto") {
		t.Fatalf("arquivos: %v", files)
	}
	var buf bytes.Buffer
	if err := a.writeBackup(&buf); err != nil {
		t.Fatal(err)
	}
	// outro PC sem nada do Radiacode recebe o backup
	b := newTestApp(t)
	zp := filepath.Join(t.TempDir(), "b.zip")
	os.WriteFile(zp, buf.Bytes(), 0o644)
	res, err := b.mergeZip(zp, true)
	if err != nil {
		t.Fatal(err)
	}
	if res.RcMinutes != 2 || len(b.rcRange(now.Add(-time.Hour), now)) != 2 {
		t.Fatalf("minutos: %+v", res)
	}
	if raws, _ := filepath.Glob(filepath.Join(b.rcDir(), "bruto", "*")); len(raws) != 0 {
		t.Fatalf("brutos: %v", raws)
	}
	// de novo: nada repete
	if res, _ := b.mergeZip(zp, true); res.RcMinutes != 0 {
		t.Fatalf("repetiu: %+v", res)
	}
}

func TestDoseRadiacode(t *testing.T) {
	a := newTestApp(t)
	now := time.Now().Truncate(time.Minute)
	var pts []store.Point
	for i := 120; i >= 1; i-- {
		m := now.Add(-time.Duration(i) * time.Minute)
		a.rcAddMinute(m.Add(time.Second), 3, 0.12)
		pts = append(pts, store.Point{T: m, CPM: 20, Src: "live"})
	}
	a.rcFlushMinute()
	a.store.Import(pts)
	c, err := a.GetDoseSrc("3h", "publico", "radiacode")
	if err != nil {
		t.Fatal(err)
	}
	if c.Device != "radiacode" || c.MeasuredMinutes != 120 || math.Abs(c.Total-0.24) > 1e-6 || math.Abs(c.BackgroundRate-0.12) > 1e-9 {
		t.Fatalf("RC102: %+v", c)
	}
	if c.OtherMinutes != 120 || math.Abs(c.OtherTotal-120*a.convCalLocked().USv(20)/60) > 1e-6 || !c.HasRC {
		t.Fatalf("casa: %v %v", c.OtherMinutes, c.OtherTotal)
	}
	g, _ := a.GetDoseSrc("3h", "publico", "gmc")
	if g.Device != "gmc" || math.Abs(g.OtherTotal-0.24) > 1e-6 {
		t.Fatalf("GMC: %+v", g)
	}
}

func TestSummaryAndReportRadiacode(t *testing.T) {
	a := newTestApp(t)
	now := time.Now().Truncate(time.Minute)
	var pts []store.Point
	for i := 90; i >= 1; i-- {
		m := now.Add(-time.Duration(i) * time.Minute)
		v := 0.12
		if i <= 30 {
			v = 0.30
		}
		a.rcAddMinute(m.Add(time.Second), 3, v)
		pts = append(pts, store.Point{T: m, CPM: 20, Src: "live"})
	}
	a.rcFlushMinute()
	a.store.Import(pts)
	// os últimos 30 min fora de casa
	a.mu.Lock()
	a.periods = append(a.periods, Period{ID: 1, From: now.Add(-30 * time.Minute).UnixMilli(), To: now.Add(time.Hour).UnixMilli()})
	a.mu.Unlock()
	s, err := a.GetSummary(now.Add(-2*time.Hour).UnixMilli(), now.UnixMilli())
	if err != nil {
		t.Fatal(err)
	}
	rc := s.RC
	if rc == nil || rc.Minutes != 90 || rc.AwayMinutes != 30 || math.Abs(rc.AwayDose-0.15) > 1e-6 || math.Abs(rc.HomeDose-0.12) > 1e-6 || rc.Max != 0.30 {
		t.Fatalf("resumo do RC102: %+v", rc)
	}
	r, err := a.buildReport(now.Add(-2*time.Hour).UnixMilli(), now.UnixMilli(), "teste")
	if err != nil {
		t.Fatal(err)
	}
	if b := renderReportPDF(r); len(b) < 2000 {
		t.Fatal("PDF vazio")
	}
}

func TestRcHomeBackground(t *testing.T) {
	a := newTestApp(t)
	now := time.Now().Truncate(time.Minute)
	for i := 50; i >= 1; i-- {
		for k := 0; k < 25; k++ { // ~1 leitura a cada 2 s
			a.rcAddMinute(now.Add(-time.Duration(i)*time.Minute+time.Duration(k*2)*time.Second), 10, 0.08)
		}
	}
	a.rcAddMinute(now.Add(-30*time.Second), 10, 0.08) // minuto incompleto: fica de fora
	a.rcFlushMinute()
	hb := a.GetHomeBackgroundDev("radiacode")
	if !hb.OK || hb.Device != "radiacode" || hb.Minutes != 50 || math.Abs(hb.CPM-600) > 1e-6 || math.Abs(hb.USv-0.08) > 1e-9 {
		t.Fatalf("fundo do RC102: %+v", hb)
	}
	// Poisson: 30 000 contagens em 50 min → ±3,46 CPM
	if math.Abs(hb.ErrCPM-math.Sqrt(30000)/50) > 0.01 {
		t.Fatalf("erro: %v", hb.ErrCPM)
	}
}

func TestRcDuplicateMinute(t *testing.T) {
	a := newTestApp(t)
	m := time.Now().Truncate(time.Minute).Add(-5 * time.Minute)
	for k := 0; k < 30; k++ {
		a.rcAddMinute(m.Add(time.Duration(k)*time.Second), 10, 0.10)
	}
	a.rcFlushMinute() // caiu no meio do minuto
	for k := 40; k < 50; k++ {
		a.rcAddMinute(m.Add(time.Duration(k)*time.Second), 14, 0.14)
	}
	a.rcFlushMinute()
	pts := a.rcRange(m.Add(-time.Hour), time.Now())
	if len(pts) != 1 || pts[0].N != 40 || math.Abs(pts[0].USvh-0.11) > 1e-9 {
		t.Fatalf("minuto repetido: %+v", pts)
	}
	st := a.GetRadiacode()
	if m.Day() == time.Now().Day() && math.Abs(st.Today-0.11/60) > 1e-6 {
		t.Fatalf("dose de hoje contou o minuto duas vezes: %v", st.Today)
	}
}

func TestRcDoseLive(t *testing.T) {
	a := newTestApp(t)
	a.rc.st.State = "conectado"
	t0 := time.Now().Add(-10 * time.Second)
	a.rc.mu.Lock()
	a.rcDoseRareLocked(0.70, 3600, t0)
	for k := 0; k <= 10; k++ { // 10 s a 36 µSv/h = 0,1 µSv
		a.rcDoseRTLocked(36, t0.Add(time.Duration(k)*time.Second))
	}
	a.rc.mu.Unlock()
	st := a.GetRadiacode()
	if !st.DoseLiveOn || math.Abs(st.DoseLive-0.80) > 1e-6 || st.DoseLiveSec < 3609 {
		t.Fatalf("ao vivo: %+v", st)
	}
	// desconectado e reaberto: vale o último total gravado
	b := newTestApp(t)
	b.dataDir = a.dataDir
	st = b.GetRadiacode()
	if st.DoseLiveOn || st.DoseLive != 0.70 || st.DoseLiveAt != t0.UnixMilli() {
		t.Fatalf("gravado: %+v", st)
	}
}

// 2.13.1: reabrir (atualização) ou reconectar logo depois continua do total
// estimado, em vez de ficar "desconectado" até o próximo envio do aparelho
func TestRcDoseResume(t *testing.T) {
	a := newTestApp(t)
	t0 := time.Now().Add(-10 * time.Minute)
	a.rc.mu.Lock()
	a.rcDoseRareLocked(1.00, 3600, t0)
	a.rc.dose.acc = 0.05
	a.rcDoseSaveLiveLocked(t0.Add(5 * time.Minute))
	// "outra abertura": esquece o que estava na memória
	a.rc.dose = rcDoseLive{}
	a.rcDoseResumeLocked(t0.Add(6 * time.Minute))
	d := a.rc.dose
	a.rc.mu.Unlock()
	if !d.has || math.Abs(d.base-1.05) > 1e-9 || d.baseSec < 3600+5*60 {
		t.Fatalf("não continuou: %+v", d)
	}
	// muito tempo depois: espera o aparelho
	a.rc.mu.Lock()
	a.rc.dose = rcDoseLive{}
	a.rcDoseResumeLocked(t0.Add(30 * time.Minute))
	has := a.rc.dose.has
	a.rc.mu.Unlock()
	if has {
		t.Fatal("continuou de um total velho")
	}
}

// "fora da escala" avisado pelo aparelho: vale até a leitura cair à metade
func TestRcOffscale(t *testing.T) {
	a := newTestApp(t)
	a.rcRecords([]radiacode.Record{radiacode.RealTime{CPS: 9000, DoseRate: 900}})
	a.rcRecords([]radiacode.Record{radiacode.Event{ID: radiacode.EvRateOffscale, Rate: 1000}})
	if !a.GetRadiacode().Offscale {
		t.Fatal("aviso não marcou")
	}
	a.rcRecords([]radiacode.Record{radiacode.RealTime{CPS: 8000, DoseRate: 800}})
	if !a.GetRadiacode().Offscale {
		t.Fatal("desmarcou cedo")
	}
	a.rcRecords([]radiacode.Record{radiacode.RealTime{CPS: 100, DoseRate: 10}})
	if a.GetRadiacode().Offscale {
		t.Fatal("não desmarcou quando a leitura caiu")
	}
}
