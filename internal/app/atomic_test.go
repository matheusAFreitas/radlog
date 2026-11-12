package app

import (
	"os"
	"strings"
	"testing"
)

// Um JSON de estado estragado não pode virar lista vazia em silêncio e ser
// sobrescrito: guarda .ilegivel e avisa no log (QA, 12/11).
func TestStateJSONBroken(t *testing.T) {
	a := newTestApp(t)
	cut := []byte(`[{"id":1,"from":1700000000000,"to":17`)
	for _, p := range []string{a.periodsPath(), a.notesPath(), a.measurementsPath()} {
		if err := os.WriteFile(p, cut, 0o644); err != nil {
			t.Fatal(err)
		}
	}
	a.loadPeriods()
	a.loadNotes()
	a.loadMeasurements()
	for _, p := range []string{a.periodsPath(), a.notesPath(), a.measurementsPath()} {
		if b, err := os.ReadFile(p + ".ilegivel"); err != nil || string(b) != string(cut) {
			t.Fatalf("%s: sem cópia do arquivo estragado (%v)", p, err)
		}
	}
	log, _ := os.ReadFile(a.logPath())
	if n := strings.Count(string(log), "comecei vazio"); n != 3 {
		t.Fatalf("avisos no log: %d\n%s", n, log)
	}
	// a próxima gravação continua funcionando
	if err := a.savePeriodsLocked(); err != nil {
		t.Fatal(err)
	}
	if b, _ := os.ReadFile(a.periodsPath()); strings.TrimSpace(string(b)) != "null" && strings.TrimSpace(string(b)) != "[]" {
		t.Fatalf("gravou %q", b)
	}
}
