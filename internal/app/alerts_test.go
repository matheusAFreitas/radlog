package app

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"radlog/internal/core/levels"
	"radlog/internal/gmc"
	"radlog/internal/store"
)

func TestAttentionStep(t *testing.T) {
	on := false
	steps := []struct {
		cpm     float64
		started bool
		on      bool
	}{
		{20, false, false}, {50, true, true}, {48, false, true}, // fica até cair abaixo de 45
		{60, false, true}, {44, false, false}, {55, true, true},
	}
	for i, s := range steps {
		if got := levels.AttentionStep(&on, true, s.cpm, DefaultLevels()); got != s.started || on != s.on {
			t.Fatalf("passo %d (%v CPM): começou=%v ligado=%v", i, s.cpm, got, on)
		}
	}
	if levels.AttentionStep(&on, false, 200, DefaultLevels()) || on {
		t.Fatal("desligado não deveria avisar")
	}
}

func TestAlertsSeparatedFromEvents(t *testing.T) {
	dir := t.TempDir()
	// arquivo de uma versão antiga, sem a coluna do tipo
	old := "2026-10-27T10:00:00-03:00,Contador conectado (GMC-800Re1.08 em COM5)\n" +
		"2026-10-27T10:05:00-03:00,Alarme: 120 CPM (limite 100)\n"
	if err := os.WriteFile(filepath.Join(dir, "eventos.csv"), []byte(old), 0o644); err != nil {
		t.Fatal(err)
	}
	st, _ := store.Open(dir)
	a := NewApp()
	a.dataDir, a.store, a.cal = dir, st, gmc.DefaultCalibration()
	a.settings = defaultSettings()
	a.events = a.loadEvents()

	if ev := a.GetEvents(kindEvent); len(ev) != 1 || !strings.HasPrefix(ev[0].Text, "Contador conectado") {
		t.Fatalf("eventos antigos: %+v", ev)
	}
	if al := a.GetEvents(kindAlert); len(al) != 1 || !strings.HasPrefix(al[0].Text, "Alarme") {
		t.Fatalf("alertas antigos: %+v", al)
	}

	a.StartDemo(60, false)
	time.Sleep(1500 * time.Millisecond)
	a.StartDemo(150, false)
	time.Sleep(1500 * time.Millisecond)
	al := a.GetEvents(kindAlert)
	if len(al) < 3 || al[0].Text != "[demo] Alarme: "+uT(a, 150)+" (limite "+uN(a, 100)+")" || al[1].Text != "[demo] Atenção: "+uT(a, 60)+" (a partir de "+uN(a, 50)+")" {
		t.Fatalf("alertas do demo: %+v", al)
	}
	if ev := a.GetEvents(kindEvent); ev[0].Text != "[demo] Modo demo ligado (60 CPM)" {
		t.Fatalf("eventos do demo: %+v", ev)
	}
	a.StopDemo()

	// um alerta real vai para o arquivo com o tipo
	a.logAlert("Atenção: 55 CPM (acima de 50)")
	b, _ := os.ReadFile(filepath.Join(dir, "eventos.csv"))
	if !strings.HasSuffix(strings.TrimSpace(string(b)), "Atenção: 55 CPM (acima de 50),alerta,atencao") {
		t.Fatalf("arquivo: %s", b)
	}
	if strings.Contains(string(b), "[demo]") {
		t.Fatal("demo foi para o arquivo")
	}
	again := a.loadEvents()
	if again[len(again)-1].Kind != kindAlert {
		t.Fatalf("tipo não voltou do arquivo: %+v", again[len(again)-1])
	}
}
