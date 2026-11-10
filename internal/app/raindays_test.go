package app

import (
	"testing"
	"time"
)

func TestRainDays(t *testing.T) {
	a := newTestApp(t)
	day := time.Date(2026, 3, 10, 0, 0, 0, 0, time.Local)
	hours := map[int64]float64{}
	for h := 0; h < 24; h++ {
		hours[day.Add(time.Duration(h)*time.Hour).Unix()] = 0.25
	}
	hours[day.Add(24*time.Hour).Unix()] = 1.04 // 1 hora do dia seguinte
	a.mu.Lock()
	if !a.mergeRainDaysLocked(hours) {
		t.Fatal("deveria mudar")
	}
	a.saveRainDaysLocked()
	a.mu.Unlock()
	if r := a.rainDays.Days["2026-03-10"]; r.MM != 6 || r.Hours != 24 {
		t.Fatalf("dia: %+v", r)
	}
	if r := a.rainDays.Days["2026-03-11"]; r.MM != 1 || r.Hours != 1 {
		t.Fatalf("dia seguinte: %+v", r)
	}
	// horas velhas apagadas: o dia não encolhe
	part := map[int64]float64{day.Add(23 * time.Hour).Unix(): 0.25}
	a.mu.Lock()
	if a.mergeRainDaysLocked(part) {
		t.Fatal("não deveria trocar um dia completo por um pedaço")
	}
	a.mu.Unlock()
	// outro app lê do disco
	b := NewApp()
	b.dataDir = a.dataDir
	b.loadRainDays()
	if r := b.rainDays.Days["2026-03-10"]; r.MM != 6 {
		t.Fatalf("disco: %+v", b.rainDays.Days)
	}
	// backup: entra o dia que falta ou com mais horas
	b.mergeRainDaysBackup([]byte(`{"v":1,"days":{"2026-03-11":{"mm":3,"h":24},"2026-03-10":{"mm":9,"h":5},"2025-01-01":{"mm":2,"h":24}}}`))
	if b.rainDays.Days["2026-03-11"].MM != 3 || b.rainDays.Days["2026-03-10"].MM != 6 || b.rainDays.Days["2025-01-01"].MM != 2 {
		t.Fatalf("backup: %+v", b.rainDays.Days)
	}
}
