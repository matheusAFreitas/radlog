package app

import (
	"bytes"
	"encoding/binary"
	"math"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"radlog/internal/gmc"
	"radlog/internal/store"
)

func near(a, b float64) bool { return math.Abs(a-b) <= 1e-9*math.Max(1, math.Abs(b)) }

func TestCalVersions(t *testing.T) {
	a := newTestApp(t)
	a.loadCalHist()
	old := store.Point{T: time.Now().Add(-48 * time.Hour).Truncate(time.Minute), CPM: 100, Src: store.SrcDevice}
	if _, err := a.store.Import([]store.Point{old}); err != nil {
		t.Fatal(err)
	}
	k0 := 10.0 / 986
	a.mu.Lock()
	tl := a.calTLLocked()
	a.mu.Unlock()
	if !near(tl.USv(old), 100*k0) {
		t.Fatalf("v1: %v", tl.USv(old))
	}

	// fator manual: vale a partir de agora, o passado continua igual
	if err := a.SetGMCFactor(0.0055, "lado a lado com o RC102"); err != nil {
		t.Fatal(err)
	}
	if err := a.SetGMCFactor(0.0055, ""); err == nil {
		t.Fatal("aceitou o mesmo fator duas vezes")
	}
	if err := a.SetGMCFactor(5, ""); err == nil {
		t.Fatal("aceitou fator absurdo")
	}
	a.mu.Lock()
	tl = a.calTLLocked()
	now := a.convCalLocked()
	a.mu.Unlock()
	fut := store.Point{T: time.Now().Add(time.Minute), CPM: 100}
	if !near(tl.USv(old), 100*k0) || !near(tl.USv(fut), 0.55) || !near(now.USv(100), 0.55) {
		t.Fatalf("depois do fator: antigo %v, novo %v, agora %v", tl.USv(old), tl.USv(fut), now.USv(100))
	}
	// a curva continua com a forma do contador (só escalada)
	if c := tl.calAt(fut.T.UnixMilli(), false); len(c.Points) != 6 || !near(c.Points[5].USv, 2000*0.0055/k0) {
		t.Fatalf("curva: %+v", c.Points)
	}
	// com a correção de tempo morto naquele minuto: só a reta
	dt := store.Point{T: fut.T, CPM: 100000, Has: true, DTOn: true}
	if !near(tl.USv(dt), 100000*0.0055) {
		t.Fatalf("tempo morto: %v", tl.USv(dt))
	}
	if !strings.Contains(tl.Tag(fut), ":manual") || strings.Contains(tl.Tag(old), "manual") {
		t.Fatalf("tags: %q %q", tl.Tag(old), tl.Tag(fut))
	}

	h := a.GetCalHistory()
	if len(h.Versions) != 2 || !h.Versions[0].Current || h.Versions[0].Factor != 0.0055 || h.Versions[1].Minutes != 1 || h.Versions[1].Until == 0 {
		t.Fatalf("histórico: %+v", h)
	}
	v2 := h.Versions[0].ID

	// comparação antes de aplicar: não muda nada
	cmp, err := a.CompareCal(v2)
	if err != nil {
		t.Fatal(err)
	}
	if cmp.Minutes != 1 || !near(cmp.DoseOrig, 100*k0/60) || !near(cmp.DoseNew, 0.55/60) || cmp.ChangePct > -40 {
		t.Fatalf("comparação: %+v", cmp)
	}
	s, _ := a.GetSummary(old.T.UnixMilli(), old.T.Add(time.Minute).UnixMilli())
	if !near(s.Dose, 100*k0/60) {
		t.Fatalf("resumo antes do recálculo: %v", s.Dose)
	}

	// recálculo retroativo: o passado usa a v2; os minutos gravados não mudam
	if err := a.SetCalRetro(v2); err != nil {
		t.Fatal(err)
	}
	s, _ = a.GetSummary(old.T.UnixMilli(), old.T.Add(time.Minute).UnixMilli())
	if !near(s.Dose, 0.55/60) {
		t.Fatalf("resumo recalculado: %v", s.Dose)
	}
	a.mu.Lock()
	tl = a.calTLLocked()
	a.mu.Unlock()
	if !strings.Contains(tl.Tag(old), ":recalculado") || !strings.Contains(tl.periodDesc(old.T.UnixMilli(), old.T.Add(time.Hour).UnixMilli()), "recalculando") {
		t.Fatalf("marca do recálculo: %q", tl.Tag(old))
	}
	pts, _ := a.store.Range(old.T, old.T.Add(time.Minute))
	if len(pts) != 1 || pts[0].CPM != 100 {
		t.Fatalf("mexeu no gravado: %+v", pts)
	}

	// abre de novo: as versões e o recálculo continuam
	b := newTestApp(t)
	b.dataDir = a.dataDir
	b.loadCalHist()
	if len(b.calHist.Versions) != 2 || b.calHist.Retro != v2 {
		t.Fatalf("recarregado: %+v", b.calHist)
	}

	// desfaz: volta ao original
	if err := a.SetCalRetro(0); err != nil {
		t.Fatal(err)
	}
	s, _ = a.GetSummary(old.T.UnixMilli(), old.T.Add(time.Minute).UnixMilli())
	if !near(s.Dose, 100*k0/60) {
		t.Fatalf("desfazer: %v", s.Dose)
	}
	if a.SetCalRetro(999) == nil {
		t.Fatal("aceitou versão inexistente")
	}
}

func TestCalVersionFromDevice(t *testing.T) {
	a := newTestApp(t)
	a.loadCalHist()
	if v := a.calHist.Versions; len(v) != 1 || v[0].Source != "padrão" {
		t.Fatalf("inicial: %+v", v)
	}
	// primeira leitura do contador: substitui o palpite de fábrica
	dev := gmc.Calibration{Points: []gmc.CalPoint{{CPM: 1500, USv: 10}, {CPM: 12000, USv: 100}}, Source: "contador"}
	a.saveLastCal(dev)
	if v := a.calHist.Versions; len(v) != 1 || v[0].Since != 0 || v[0].Points[0].CPM != 1500 {
		t.Fatalf("primeira leitura: %+v", v)
	}
	a.saveLastCal(dev) // a mesma: nada novo
	if len(a.calHist.Versions) != 1 {
		t.Fatal("repetiu a versão")
	}
	if err := a.SetGMCFactor(0.0055, ""); err != nil {
		t.Fatal(err)
	}
	// a curva do contador mudou: versão nova, o fator manual fica
	dev2 := gmc.Calibration{Points: []gmc.CalPoint{{CPM: 1600, USv: 10}}, Source: "contador"}
	a.saveLastCal(dev2)
	vs := a.calHist.Versions
	if len(vs) != 3 || vs[2].Source != "contador" || vs[2].Factor != 0.0055 || !strings.Contains(vs[2].Note, "fator manual") {
		t.Fatalf("curva nova: %+v", vs)
	}

	// backup: outro PC recebe as versões sem repetir
	var buf bytes.Buffer
	if err := a.writeBackup(&buf); err != nil {
		t.Fatal(err)
	}
	b := newTestApp(t)
	b.loadCalHist()
	zp := filepath.Join(t.TempDir(), "b.zip")
	os.WriteFile(zp, buf.Bytes(), 0o644)
	for i := 0; i < 2; i++ {
		if _, err := b.mergeZip(zp, true); err != nil {
			t.Fatal(err)
		}
	}
	if n := len(b.calHist.Versions); n != 3 {
		t.Fatalf("no outro PC: %d versões", n)
	}
}

func TestGMCDiagReport(t *testing.T) {
	cfg := make([]byte, 512)
	cfg2 := append([]byte(nil), cfg...)
	binary.BigEndian.PutUint32(cfg2[200:], math.Float32bits(1.25))
	r1 := gmcDiagRound{cfg: cfg, ans: map[string][]byte{"GETVER": []byte("GMC-800Re1.23")}, err: map[string]string{}}
	r2 := gmcDiagRound{cfg: cfg, ans: map[string][]byte{"GETVER": []byte("GMC-800Re1.23")}, err: map[string]string{}}
	out, txt := gmcDiagReport(r1, r2)
	if out.CfgChanged != 0 || len(out.Unknown) != 0 || !strings.Contains(out.Verdict, "continua calculando") {
		t.Fatalf("sem nada: %+v", out)
	}
	r2.cfg = cfg2
	r2.ans["GETDOSE"] = []byte{0, 0, 1, 2}
	out, txt = gmcDiagReport(r1, r2)
	if out.CfgChanged == 0 || len(out.Unknown) != 1 || !strings.Contains(txt, "float BE 0 → 1.25") || !strings.Contains(txt, "GETDOSE") {
		t.Fatalf("com mudança: %+v\n%s", out, txt)
	}
}

func TestRcDeviceDose(t *testing.T) {
	a := newTestApp(t)
	now := time.Now()
	y, m, d := now.Date()
	day := time.Date(y, m, d, 0, 0, 0, 0, now.Location())
	if now.Sub(day) < 2*time.Hour {
		t.Skip("perto da meia-noite")
	}
	if _, ok := a.rcTodayDevice(now); ok {
		t.Fatal("sem registro não pode valer")
	}
	// ontem 23:00: 10 µSv; hoje 01:00: 10,2 (longe do PC, o aparelho somou); agora: 10,5
	a.rcDoseLogAppend(day.Add(-time.Hour), 10, 1000)
	a.rcDoseLogAppend(day.Add(time.Hour), 10.2, 8200)
	a.rcDoseLogAppend(now.Add(-time.Minute), 10.5, 9000)
	dose, up, ok := a.rcDeviceDose(day, now)
	want := 10.5 - (10 + 0.2*0.5) // a meia-noite fica no meio da reta 23:00–01:00
	if !ok || math.Abs(dose-want) > 1e-3 || up.After(now) {
		t.Fatalf("dose de hoje: %v %v (quero %v)", dose, ok, want)
	}
	a.rcDevT.at = time.Time{}
	if v, ok := a.rcTodayDevice(now); !ok || math.Abs(v-want) > 1e-3 {
		t.Fatalf("cache: %v %v", v, ok)
	}
	// período que começa antes do registro: não vale
	if _, _, ok := a.rcDeviceDose(day.AddDate(0, 0, -3), now); ok {
		t.Fatal("aceitou período sem o total do começo")
	}
}
