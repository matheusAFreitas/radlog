package app

import (
	"regexp"
	"sort"
	"strings"
	"time"

	"encoding/json"
	"fmt"
	"math"
	"os"
	"radlog/internal/pdf"
	"testing"
)

// QA (config): a tela manda state.settings (lido ao abrir) + os campos da
// tela. O que o Go mudou por fora depois disso volta ao valor velho?
func TestQAPortUSBStale(t *testing.T) {
	a := newTestApp(t)
	a.settings.Port, a.settings.PortUSB = "COM3", "USB\\VID_1A86&PID_7523\\OLD"
	a.saveSettingsLocked()
	stale := a.GetSettings() // a tela abriu aqui
	// conectou o contador em outra porta (Connect grava Port e PortUSB)
	a.mu.Lock()
	a.settings.Port, a.settings.PortUSB = "COM7", "USB\\VID_1A86&PID_7523\\NEW"
	a.saveSettingsLocked()
	a.mu.Unlock()
	// a tela salva qualquer coisa (ex.: o som), com o state.settings velho
	stale.AlarmSound = "bipes"
	a.SaveSettings(stale)
	a.flushWrites()
	b, _ := os.ReadFile(a.settingsPath())
	var disk Settings
	_ = json.Unmarshal(b, &disk)
	fmt.Printf("depois de salvar: Port=%q PortUSB=%q (disco: %q / %q)\n", a.settings.Port, a.settings.PortUSB, disk.Port, disk.PortUSB)
	if a.settings.PortUSB != "USB\\VID_1A86&PID_7523\\NEW" {
		t.Errorf("PortUSB voltou para %q (Port ficou %q)", a.settings.PortUSB, a.settings.Port)
	}
}

// QA (config): o que a tela recebe de volta para os níveis de atenção e
// perigo, e o que ela mostra (fillLevels(lv()) usa attentionCPM × calK).
func TestQALevelsRoundTrip(t *testing.T) {
	a := newTestApp(t)
	s := a.GetSettings()
	s.Levels.AttentionUSv, s.Levels.DangerUSv, s.AlarmUSv = 0.3, 2, 0.8
	got := a.SaveSettings(s)
	k := a.GetStatus().USvPerCPM
	usv3 := func(v float64) float64 {
		if v <= 0 {
			return 0
		}
		d := 3 - int(math.Ceil(math.Log10(v)))
		p := math.Pow(10, float64(d))
		return math.Round(v*p) / p
	}
	fmt.Printf("k=%.6f\n", k)
	fmt.Printf("atenção: salvo %.4g µSv/h -> attentionCPM=%d -> tela mostra %.4g\n", got.Levels.AttentionUSv, got.Levels.AttentionCPM, usv3(float64(got.Levels.AttentionCPM)*k))
	fmt.Printf("perigo:  salvo %.4g µSv/h -> dangerCPM=%d -> tela mostra %.4g\n", got.Levels.DangerUSv, got.Levels.DangerCPM, usv3(float64(got.Levels.DangerCPM)*k))
	fmt.Printf("alarme:  salvo %.4g µSv/h (a tela usa alarmUSv direto)\n", got.AlarmUSv)
	// a tela salva de novo (outro campo) com o que mostra
	s2 := got
	s2.Levels.AttentionUSv = usv3(float64(got.Levels.AttentionCPM) * k)
	s2.Levels.DangerUSv = usv3(float64(got.Levels.DangerCPM) * k)
	s2.AlarmSound = "bipes"
	a.SaveSettings(s2)
	for _, c := range a.readCfgHist() {
		if c.Field == "Aviso de atenção" || c.Field == "Alarme de perigo" {
			fmt.Printf("histórico: %s: %q -> %q (%s)\n", c.Field, c.From, c.To, c.Source)
		}
	}
}

// QA (config): unidades e formato da data no backup
func TestQABackupUnits2(t *testing.T) {
	a := newTestApp(t)
	s := a.GetSettings()
	s.RateUnit, s.CountUnit, s.DateFormat, s.Lang = "mrh", "cpm", "iso", "en"
	a.SaveSettings(s)
	var p Settings
	_ = json.Unmarshal(a.backupSettingsJSON(), &p)
	fmt.Printf("backup: lang=%q rateUnit=%q countUnit=%q dateFormat=%q\n", p.Lang, p.RateUnit, p.CountUnit, p.DateFormat)
	// outro PC (padrão) importa o backup
	b := newTestApp(t)
	n := b.stashBackupSettings(a.backupSettingsJSON())
	got := b.ApplyBackupSettings()
	fmt.Printf("outro PC: %d campos diferentes; depois de aplicar: lang=%q rateUnit=%q countUnit=%q dateFormat=%q\n", n, got.Lang, got.RateUnit, got.CountUnit, got.DateFormat)
}

// QA (config): grava, relê do disco (como ao abrir o app) e compara campo a campo
func TestQAPersist(t *testing.T) {
	a := newTestApp(t)
	s := a.GetSettings()
	s.AutoConnect = false
	s.AlarmUSv = 0.8
	s.AlarmSound = "bipes"
	s.AlarmRaise = false
	s.AttentionEnabled = false
	s.Levels.AttentionUSv, s.Levels.DangerUSv, s.Levels.SnoozeMin = 0.3, 2, 10
	s.Levels.SatMaxUSv, s.Levels.SatDropUSv, s.Levels.SatHoldSec, s.Levels.RcSatUSv = 1500, 800, 90, 900
	s.Levels.DeadTimeOn, s.Levels.DeadTimeUs = true, 150
	s.AwayAskMin = 45
	s.TrayOnClose, s.TrayPlain, s.TrayDevice = false, true, "radiacode"
	s.UIScale, s.UIDensity = 1.3, "compacto"
	s.RateUnit, s.CountUnit, s.DateFormat = "mrh", "cpm", "iso"
	s.PulseOff, s.PulseStrip, s.PulseLogo = true, true, "gmc"
	s.NoUpdateCheck, s.KeepOldVersions = true, true
	s.Lang = "en"
	s.NoMeasureReport, s.NoAutoHistory = true, true
	saved := a.SaveSettings(s)
	a.flushWrites()
	re := a.loadSettings()
	sj, _ := json.Marshal(saved)
	rj, _ := json.Marshal(re)
	var m1, m2 map[string]any
	_ = json.Unmarshal(sj, &m1)
	_ = json.Unmarshal(rj, &m2)
	for k, v := range m1 {
		if fmt.Sprint(v) != fmt.Sprint(m2[k]) {
			fmt.Printf("difere ao reler: %s: salvo %v, relido %v\n", k, v, m2[k])
		}
	}
	fmt.Println("persistência conferida")
}

// QA (config): relatório com a taxa em mrem/h e data ISO — o que sai em µSv e dd/mm?
func TestQAReportUnits(t *testing.T) {
	base := time.Now().Truncate(time.Hour).Add(-48 * time.Hour)
	a, _ := testApp(t)
	fillApp(t, a, base, 0, 600, "A")
	T := base.Add(2 * time.Hour).UnixMilli()
	a.events = append(a.events,
		Event{ID: 1, Time: T, End: T + 600000, Kind: kindAlert, Level: "atencao", Peak: 70},
		Event{ID: 2, Time: T + 300000, End: T + 500000, Kind: kindAlert, Level: "alarme", Peak: 134},
	)
	a.mu.Lock()
	a.settings.RateUnit, a.settings.DateFormat, a.lang = "mrh", "iso", "pt"
	a.mu.Unlock()
	r, err := a.buildReport(base.UnixMilli(), base.Add(24*time.Hour).UnixMilli(), "teste")
	if err != nil {
		t.Fatal(err)
	}
	seen := map[string]bool{}
	var mrem, all []string
	ddmm := regexp.MustCompile(`(^|[^\d/])\d{2}/\d{2}([^\d/]|$)`)
	pdf.SetTextHook(func(s string) {
		if strings.Contains(s, "µSv") || ddmm.MatchString(s) {
			seen[s] = true
		}
		if strings.Contains(s, "mrem") {
			mrem = append(mrem, s)
		}
		all = append(all, s)
	})
	defer pdf.SetTextHook(nil)
	_ = a.inLang(func() []byte { return renderReportPDF(r) })
	var l []string
	for s := range seen {
		l = append(l, s)
	}
	sort.Strings(l)
	fmt.Printf("com mrem (%d): %q\n", len(mrem), mrem[:min(len(mrem), 12)])
	for i, x := range all {
		if strings.HasPrefix(x, "Máx µSv/h") || strings.HasPrefix(x, "Pico µSv/h") {
			fmt.Printf("depois de %q: %q\n", x, all[i+1:min(len(all), i+12)])
		}
	}
	fmt.Printf("PDF (mrem/h, iso): %d textos com µSv ou dd/mm:\n  %s\n", len(l), strings.Join(l, "\n  "))
}

// QA (config): o que o Go muda ao salvar (limites fora da faixa)
func TestQAClamp(t *testing.T) {
	a := newTestApp(t)
	s := a.GetSettings()
	s.Levels.SnoozeMin, s.Levels.SatHoldSec, s.Levels.DeadTimeUs = 120, 7200, 900
	s.AlarmUSv, s.Levels.AttentionUSv, s.Levels.DangerUSv = 0.001, 0.001, 0.01
	got := a.SaveSettings(s)
	fmt.Printf("snooze 120 -> %d; satHold 7200 -> %d; τ 900 -> %v; alarme 0.001 -> %v; atenção 0.001 -> %v; perigo 0.01 -> %v\n",
		got.Levels.SnoozeMin, got.Levels.SatHoldSec, got.Levels.DeadTimeUs, got.AlarmUSv, got.Levels.AttentionUSv, got.Levels.DangerUSv)
}
