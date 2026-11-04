package app

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"radlog/internal/store"
)

// Teste de ponta a ponta contra um contador simulado numa porta serial.
// Só roda quando GMC_TEST_PORT aponta para a porta (no Linux, uma pty do
// simulador). Leva pouco mais de um minuto, para pegar a virada do minuto.
func TestEndToEnd(t *testing.T) {
	port := os.Getenv("GMC_TEST_PORT")
	if port == "" {
		t.Skip("GMC_TEST_PORT não definido")
	}
	dir := t.TempDir()
	a := NewApp()
	a.dataDir = dir
	a.settings = defaultSettings()
	st, err := store.Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	a.store = st

	info, err := a.Connect(port)
	if err != nil {
		t.Fatal(err)
	}
	if info.Version != "GMC-800Re1.08" || info.Calibration.Source != "contador" || info.ClockDrift < 5 || info.ClockDrift > 9 {
		t.Fatalf("info = %+v", info)
	}

	// Sincroniza o relógio (o simulador devolve +7 s, então continua ~7).
	if _, err := a.SyncClock(); err != nil {
		t.Fatal(err)
	}

	// Histórico com leitura ao vivo rodando ao mesmo tempo.
	res, err := a.DownloadHistory()
	if err != nil {
		t.Fatal(err)
	}
	if res.Records != 300 || res.Added != 300 || res.Alerts != 1 {
		t.Fatalf("histórico = %+v", res)
	}
	// o pico do histórico vira um alerta com a hora em que aconteceu
	retro := a.GetEvents(kindAlert)
	if len(retro) != 1 || !retro[0].Retro || retro[0].Level != "alarme" || retro[0].Peak != 130 ||
		time.Since(time.UnixMilli(retro[0].Time)) < 4*time.Hour {
		t.Fatalf("alerta retroativo = %+v", retro)
	}
	if _, err := os.Stat(res.File); err != nil {
		t.Fatal(err)
	}
	if m := a.GetMemoryInfo(); !m.Known || m.UsedBytes != 312 || m.TotalBytes != 2<<20 {
		t.Fatalf("memória = %+v", m)
	}

	// Lê e grava a configuração do contador.
	cfg, err := a.GetDeviceConfig()
	if err != nil || !cfg.Editable || cfg.Values["saveMode"] != 1 || cfg.Values["alarmCPM"] != 100 {
		t.Fatalf("config = %+v (%v)", cfg, err)
	}
	if err := a.SnapshotDeviceConfig(); err != nil {
		t.Fatal(err)
	}
	cfg.Values["saveMode"] = 2
	cfg.Values["alarmCPM"] = 300
	cfg.Values["clickSound"] = 1
	for i := range cfg.Calibration.Points {
		cfg.Calibration.Points[i].CPM = 132 * cfg.Calibration.Points[i].USv
	}
	got, err := a.SetDeviceConfig(cfg)
	if err != nil || got.Values["saveMode"] != 2 || got.Values["alarmCPM"] != 300 || got.Values["clickSound"] != 1 {
		t.Fatalf("depois de gravar = %+v (%v)", got, err)
	}
	again, err := a.GetDeviceConfig()
	if err != nil || again.Values["alarmCPM"] != 300 || again.Calibration.Points[0].CPM != 1320 {
		t.Fatalf("relido = %+v (%v)", again, err)
	}
	if u := a.cal.USv(132); u < 0.999 || u > 1.001 {
		t.Fatalf("o app não passou a usar a calibração nova: %v", u)
	}
	diff, err := a.DiffDeviceConfig()
	if err != nil || len(diff) == 0 {
		t.Fatalf("diff = %+v (%v)", diff, err)
	}
	backups, _ := filepath.Glob(filepath.Join(dir, "configuracao", "*.bin"))
	if len(backups) != 2 { // a da primeira leitura e a de antes de gravar
		t.Fatalf("cópias: %v", backups)
	}

	// Liga o demo: a tela vê 999 CPM, mas o que vai para o disco é o real (120).
	a.StartDemo(999, false)
	// Espera passar uma virada de minuto completa com leitura ao vivo.
	deadline := time.Now().Add(130 * time.Second)
	for time.Now().Before(deadline) {
		pts, _ := a.store.Range(time.Now().Add(-5*time.Minute), time.Now().Add(time.Minute))
		live := 0
		for _, p := range pts {
			if p.Src == store.SrcLive {
				live++
			}
		}
		if live > 0 {
			break
		}
		time.Sleep(2 * time.Second)
	}
	real, _ := a.store.Range(time.Now().Add(-5*time.Minute), time.Now().Add(time.Minute))
	for _, p := range real {
		if p.Src == store.SrcLive && p.CPM != 120 {
			t.Fatalf("o demo vazou para o disco: %+v", p)
		}
	}
	a.StopDemo()
	stats, err := a.GetStats()
	if err != nil || stats.Avg1h == nil || *stats.Avg1h != 120 {
		t.Fatalf("stats = %+v (%v)", stats, err)
	}
	c, err := a.GetChart("24h")
	if err != nil || c.Avg == nil || len(c.Values) != 288 {
		t.Fatalf("chart 24h = %d valores, avg %v (%v)", len(c.Values), c.Avg, err)
	}
	days, _ := a.GetDaily(7)
	if len(days) == 0 {
		t.Fatal("sem fundo diário")
	}

	evs := a.GetEvents("")
	var texts []string
	for _, e := range evs {
		texts = append(texts, e.Text)
	}
	joined := strings.Join(texts, " | ")
	for _, want := range []string{"Contador conectado", "Relógio", "Histórico baixado: 300 registros", "Alarme: 120 CPM", "gravação na memória a cada minuto, som de clique ligado, limite do alarme 300 CPM, calibração"} {
		if !strings.Contains(joined, want) {
			t.Fatalf("faltou evento %q em: %s", want, joined)
		}
	}

	a.Disconnect()
	if a.GetStatus().State != "desconectado" {
		t.Fatalf("estado = %+v", a.GetStatus())
	}
	t.Logf("eventos: %s", joined)
}
