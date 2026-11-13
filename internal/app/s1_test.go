package app

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

// S1: anexo com nome que sai da pasta (vindo de um backup de terceiros) não
// entra, e "remover" não apaga nada fora da pasta do anexo
func TestBackupAttachmentNames(t *testing.T) {
	base := time.Now().Truncate(time.Hour).Add(-48 * time.Hour)
	a, _ := testApp(t)
	b, _ := testApp(t)
	a.measurements = []SavedMeasure{{ID: 1, Name: "pedra", Time: base.UnixMilli(), Attachments: []MeasureAttachment{
		{File: "../../config.json", Type: "text/plain"}, {File: "foto.jpg", Type: "image/jpeg"}, {File: `..\x.exe`},
	}}}
	_ = a.saveMeasurementsLocked()
	zip := filepath.Join(t.TempDir(), "a.zip")
	if err := a.writeBackupFile(zip); err != nil {
		t.Fatal(err)
	}
	if _, err := b.mergeBackup(zip); err != nil {
		t.Fatal(err)
	}
	if len(b.measurements) != 1 {
		t.Fatalf("%d medições", len(b.measurements))
	}
	for _, at := range b.measurements[0].Attachments {
		if at.File != "foto.jpg" {
			t.Fatalf("anexo perigoso entrou: %q", at.File)
		}
	}
	// mesmo que já esteja gravado em medicoes.json (versão antiga), o
	// "remover" com esse nome não alcança o config.json
	cfg := b.settingsPath()
	b.flushWrites()
	_ = os.WriteFile(cfg, []byte("{}"), 0o644)
	b.mu.Lock()
	b.measurements[0].Attachments = append(b.measurements[0].Attachments, MeasureAttachment{File: "../../config.json"})
	b.mu.Unlock()
	if err := b.RemoveMeasurementAttachment(b.measurements[0].ID, "../../config.json"); err == nil {
		t.Fatal("remover aceitou o nome")
	}
	if _, err := os.Stat(cfg); err != nil {
		t.Fatal("config.json sumiu")
	}
	_ = b.saveMeasurementsLocked()
	b.loadMeasurements()
	for _, at := range b.measurements[0].Attachments {
		if at.File != "foto.jpg" {
			t.Fatalf("ao carregar ficou %q", at.File)
		}
	}
}
