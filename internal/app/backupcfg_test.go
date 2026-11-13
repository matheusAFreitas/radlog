package app

import (
	"archive/zip"
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// o backup leva as configurações sem o lugar e sem o que é só do PC; ao
// importar, só mudam se a pessoa escolher
func TestBackupSettings(t *testing.T) {
	a, _ := testApp(t)
	b, _ := testApp(t)
	a.settings.AlarmUSv = 0.42
	a.settings.AlarmSound = "bipes"
	a.settings.Lang = "en"
	a.settings.Levels.SnoozeMin = 15
	a.settings.RainEnabled, a.settings.RainLat, a.settings.RainLon, a.settings.RainPlace = true, -23.55, -46.63, "Lugar Secreto"
	a.settings.Port, a.settings.RcBLE, a.settings.ReportDir = "COM7", "AA:BB:CC:DD:EE:FF", "/pasta/secreta"
	a.settings.UIScale, a.settings.StartWithWindows, a.settings.UsageOn = 1.25, true, true

	b.settings.Port, b.settings.UIScale = "/dev/ttyUSB0", 1.1
	a.migrateThrLocked() // como ao abrir o app: limites em µSv/h
	b.migrateThrLocked()

	var buf bytes.Buffer
	if err := a.writeBackup(&buf); err != nil {
		t.Fatal(err)
	}
	zr, err := zip.NewReader(bytes.NewReader(buf.Bytes()), int64(buf.Len()))
	if err != nil {
		t.Fatal(err)
	}
	var cfg string
	for _, f := range zr.File {
		if f.Name == backupSettingsFile {
			raw, _ := readZipFile(f)
			cfg = string(raw)
		}
	}
	if cfg == "" {
		t.Fatal("backup sem settings.json")
	}
	for _, bad := range []string{"-23.55", "-46.63", "Lugar Secreto", "COM7", "AA:BB", "secreta", "1.25"} {
		if strings.Contains(cfg, bad) {
			t.Fatalf("configurações do backup levaram %q:\n%s", bad, cfg)
		}
	}

	zp := filepath.Join(t.TempDir(), "b.zip")
	_ = os.WriteFile(zp, buf.Bytes(), 0o644)
	res, err := b.mergeZip(zp, true)
	if err != nil {
		t.Fatal(err)
	}
	if res.Settings != 4 { // alarme, som, idioma, silenciar
		t.Fatalf("ajustes diferentes: %d", res.Settings)
	}
	if b.settings.AlarmSound != "sirene" {
		t.Fatal("importar mudou as configurações sem perguntar")
	}
	s := b.ApplyBackupSettings()
	if s.AlarmUSv != 0.42 || s.AlarmSound != "bipes" || s.Lang != "en" || s.Levels.SnoozeMin != 15 {
		t.Fatalf("não aplicou: %+v", s)
	}
	if s.Port != "/dev/ttyUSB0" || s.UIScale != 1.1 || s.RainEnabled || s.RainPlace != "" || s.StartWithWindows || s.UsageOn || s.RcBLE != "" {
		t.Fatalf("mexeu no que é só deste PC: %+v", s)
	}
	// de novo: já é igual, nada a perguntar
	if res, _ := b.mergeZip(zp, true); res.Settings != 0 || b.bkSettings != nil {
		t.Fatalf("segunda vez: %d", res.Settings)
	}
	// a sincronização não traz configurações
	b.settings.AlarmSound = "mudo"
	if res, _ := b.mergeZip(zp, false); res.Settings != 0 || b.bkSettings != nil {
		t.Fatal("sincronização guardou configurações")
	}
	// sem nada guardado, aplicar não muda nada
	if s := b.ApplyBackupSettings(); s.AlarmSound != "mudo" {
		t.Fatal("aplicou sem backup")
	}
}
