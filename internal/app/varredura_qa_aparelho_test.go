package app

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"radlog/internal/store"
	"testing"
	"time"

	"radlog/internal/gmc"
)

// QA: a tela carrega as configurações uma vez (GetSettings no init) e manda
// de volta o objeto inteiro em cada SaveSettings. O connect grava PortUSB no
// meio disso; o saveSettingsFrom protege Port mas não PortUSB.
func TestQAPortUSBLostOnUISave(t *testing.T) {
	a, _ := testApp(t)
	// 1. tela abre: GetSettings (ainda sem portUsb) -> JSON como no JS
	b, _ := json.Marshal(a.GetSettings())
	var ui map[string]any
	_ = json.Unmarshal(b, &ui)
	t.Logf("tela carregou portUsb presente? %v", ui["portUsb"] != nil)

	// 2. conectou o contador (o que connect() faz em app.go:1120-1122)
	a.mu.Lock()
	a.settings.Port = "COM3"
	a.settings.PortUSB = "USB\\VID_1A86&PID_7523\\5&2A3B"
	a.saveSettingsLocked()
	a.mu.Unlock()

	// 3. a pessoa liga/desliga o alarme (saveSettings({alarmEnabled}) = state.settings + extra)
	ui["alarmEnabled"] = false
	b, _ = json.Marshal(ui)
	var s Settings
	_ = json.Unmarshal(b, &s)
	a.SaveSettings(s)

	a.mu.Lock()
	got, port := a.settings.PortUSB, a.settings.Port
	a.mu.Unlock()
	t.Logf("depois do SaveSettings da tela: Port=%q PortUSB=%q", port, got)
	a.flushWrites()
	raw, _ := os.ReadFile(a.settingsPath())
	var disk map[string]any
	_ = json.Unmarshal(raw, &disk)
	t.Logf("config.json: port=%v portUsb=%v", disk["port"], disk["portUsb"])

	// 4. reconexão: com o USBID apagado, abre o que estiver na COM3
	ports := []gmc.PortInfo{{Name: "COM3", USBID: "USB\\VID_2341&PID_0043\\OUTRO", IsUSB: true}, {Name: "COM7", USBID: "USB\\VID_1A86&PID_7523\\5&2A3B", IsUSB: true}}
	p1, ok1 := resolvePort(ports, "COM3", "USB\\VID_1A86&PID_7523\\5&2A3B")
	p2, ok2 := resolvePort(ports, "COM3", got)
	t.Logf("resolvePort com USBID guardado: %q %v ; com o que ficou salvo: %q %v", p1, ok1, p2, ok2)
	if got == "" {
		t.Errorf("PortUSB apagado pelo SaveSettings da tela (Port foi preservado = %q)", port)
	}
}

// QA: sessão anterior guardou o USBID da porta USB A; hoje o contador foi
// ligado noutra porta USB (CH340 sem número de série: o USBID muda com a
// porta física, e o Windows dá outro COM). O connect grava o novo; a tela
// manda de volta o velho.
func TestQAPortUSBStaleOnUISave(t *testing.T) {
	a, _ := testApp(t)
	oldUSB, newUSB := "USB\\VID_1A86&PID_7523\\5&AAAA&0&1", "USB\\VID_1A86&PID_7523\\5&BBBB&0&2"
	a.settings.Port, a.settings.PortUSB = "COM3", oldUSB
	b, _ := json.Marshal(a.GetSettings()) // init da tela
	var ui map[string]any
	_ = json.Unmarshal(b, &ui)
	a.mu.Lock() // connect() na porta nova
	a.settings.Port, a.settings.PortUSB = "COM5", newUSB
	a.saveSettingsLocked()
	a.mu.Unlock()
	ui["uiScale"] = 1.25 // qualquer saveSettings da tela
	b, _ = json.Marshal(ui)
	var s Settings
	_ = json.Unmarshal(b, &s)
	a.SaveSettings(s)
	a.mu.Lock()
	port, usb := a.settings.Port, a.settings.PortUSB
	a.mu.Unlock()
	ports := []gmc.PortInfo{{Name: "COM5", USBID: newUSB, IsUSB: true}}
	p, ok := resolvePort(ports, port, usb)
	t.Logf("salvo: Port=%q PortUSB=%q ; reconexão com o próprio contador em COM5: resolvePort=%q ok=%v", port, usb, p, ok)
	if !ok {
		t.Errorf("a reconexão recusa o próprio contador (\"COM5 agora é outro aparelho\")")
	}
}

// QA: Resumo da saída: a média do RC102 fora é comparada com "casa", que
// vem do GMC-800 (mediana em CPM × calibração do GMC), não do RC102.
func TestQATripHomeFromOtherDevice(t *testing.T) {
	a := newTestApp(t)
	t0 := time.Now().Add(-30 * time.Hour).Truncate(time.Hour)
	// GMC-800 em casa o dia todo: 20 CPM (~0,20 µSv/h pela calibração padrão)
	var pts []store.Point
	for i := 0; i < 26*60; i++ {
		pts = append(pts, store.Point{T: t0.Add(time.Duration(i) * time.Minute), CPM: 20, Src: store.SrcLive})
	}
	if _, err := a.store.Import(pts); err != nil {
		t.Fatal(err)
	}
	// RC102: 0,10 µSv/h o dia todo, em casa e fora (o fundo dele)
	writeRcMinutes(t, a, t0, 26*60, 0.10)
	leave := t0.Add(20 * time.Hour)
	p, err := a.SavePeriod(Period{From: leave.UnixMilli(), To: leave.Add(2 * time.Hour).UnixMilli(), GMC: WhereHome, RC: WhereMe, NotHome: true})
	if err != nil {
		t.Fatal(err)
	}
	s, err := a.GetTripSummary(p.ID)
	if err != nil {
		t.Fatal(err)
	}
	if s.RC == nil {
		t.Fatal("sem RC")
	}
	t.Logf("RC102 na saída: média %.3f µSv/h; fundo do RC102 em casa (RC.BgUSv) %.3f; 'casa' usado: %.3f (%s) → a tela mostra %.1f× casa",
		s.RC.Avg, s.RC.BgUSv, s.HomeUSv, s.HomeSrc, s.RC.Avg/s.HomeUSv)
	if s.HomeSrc != "rc" && s.RC.BgUSv > 0 {
		t.Errorf("o resumo da saída do RC102 usa o fundo de casa do GMC-800 (%.3f) e não o do próprio RC102 (%.3f)", s.HomeUSv, s.RC.BgUSv)
	}
}

// QA: "Lado a lado com o RC102 ... faria o GMC bater com ele" (Configurações
// › GMC › Calibração) usa as últimas 6 h sem olhar onde estava cada um.
func TestQARcCompareIgnoresWhere(t *testing.T) {
	a := newTestApp(t)
	now := time.Now().Truncate(time.Minute)
	t0 := now.Add(-360 * time.Minute)
	var pts []store.Point
	for i := 0; i <= 360; i++ {
		pts = append(pts, store.Point{T: t0.Add(time.Duration(i) * time.Minute), CPM: 20, Src: store.SrcLive})
	}
	if _, err := a.store.Import(pts); err != nil {
		t.Fatal(err)
	}
	// 5 h juntos em casa (0,10 µSv/h no RC102) e 1 h fora de casa com o RC102 (0,60)
	writeRcMinutes(t, a, t0, 300, 0.10)
	away := t0.Add(300 * time.Minute)
	writeRcMinutes2 := func(from time.Time, n int, usv float64) {
		f := filepath.Join(a.rcDir(), from.Format("2006-01")+".csv")
		b, _ := os.ReadFile(f)
		s := string(b)
		for i := 0; i < n; i++ {
			s += fmt.Sprintf("%s,%.2f,%.4f,%d\n", from.Add(time.Duration(i)*time.Minute).Format(time.RFC3339), 2.0, usv, 30)
		}
		_ = os.WriteFile(f, []byte(s), 0o644)
	}
	writeRcMinutes2(away, 60, 0.60)
	if _, err := a.SavePeriod(Period{From: away.UnixMilli(), To: away.Add(60 * time.Minute).UnixMilli(), Name: "Praia", GMC: WhereHome, RC: WhereMe, NotHome: true}); err != nil {
		t.Fatal(err)
	}
	c, err := a.GetRcCompare(360)
	if err != nil {
		t.Fatal(err)
	}
	t.Logf("GetRcCompare(6 h): %d min 'juntos', sugere K = %.5f µSv/h por CPM (só os minutos em casa dariam %.5f)", c.Minutes, c.SuggestK, 0.10/20)
	if c.SuggestK > 0.10/20*1.2 {
		t.Errorf("a sugestão de calibração do GMC-800 usa minutos em que o RC102 estava fora de casa")
	}
}

// QA: o backup leva "as preferências que valem em qualquer PC" (idioma,
// alarmes...), mas não as unidades nem o formato da data (2.84 / 2.97).
func TestQABackupUnits(t *testing.T) {
	here := defaultSettings()
	bk := defaultSettings()
	bk.RateUnit, bk.CountUnit, bk.DateFormat, bk.Lang = "mrh", "cps", "iso", "pt"
	p := portableSettings(bk)
	t.Logf("no backup: rateUnit=%q countUnit=%q dateFormat=%q lang=%q; diferenças contadas: %d", p.RateUnit, p.CountUnit, p.DateFormat, p.Lang, portableDiff(here, bk))
	dst := here
	portableCopy(&dst, bk)
	if dst.RateUnit != "mrh" || dst.DateFormat != "iso" {
		t.Errorf("importar o backup não traz mrem/h nem a data ISO (rateUnit=%q dateFormat=%q)", dst.RateUnit, dst.DateFormat)
	}
}

// QA: PDF de uma medição salva exportado depois de mudar a calibração do GMC:
// a linha "Calibração" sai com o fator de agora, os números com o de antes.
func TestQAMeasurePDFCalLater(t *testing.T) {
	a, _ := testApp(t)
	dir := t.TempDir()
	a.settings.ReportDir = dir
	m := fakeMeasure(120, 25, 300) // fator padrão 0,0101 (calibração do contador)
	id, err := a.addMeasurement(m)
	if err != nil {
		t.Fatal(err)
	}
	k0 := m.Obj.USv / m.Obj.CPM
	if err := a.setGMCFactor(0.0065, "você", "intercalibração"); err != nil {
		t.Fatal(err)
	}
	path, err := a.saveMeasureReport(id)
	if err != nil {
		t.Fatal(err)
	}
	out := "/tmp/qa/sweep/aparelho/medicao-depois.pdf"
	b, _ := os.ReadFile(path)
	_ = os.WriteFile(out, b, 0o644)
	t.Logf("fator usado nos números da medição: %.4f; PDF salvo em %s", k0, out)
}
