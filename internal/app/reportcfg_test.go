package app

import (
	"encoding/json"
	"math"
	"os"
	"strings"
	"testing"
	"time"
)

func TestReportConfigPeriodAndMaturity(t *testing.T) {
	base := time.Now().Truncate(time.Hour).Add(-48 * time.Hour)
	a, _ := testApp(t)
	fillApp(t, a, base, 0, 600, "A")
	// histórico de configurações: atenção 40 antes do período, muda para 60 no meio
	h := []ConfigChange{
		{T: base.Add(-24 * time.Hour).UnixMilli(), Field: "Aviso de atenção", To: "a partir de 40 CPM", Source: "início do registro"},
		{T: base.Add(-24 * time.Hour).UnixMilli(), Field: "Tempo morto", To: "desligado", Source: "início do registro"},
		{T: base.Add(5 * time.Hour).UnixMilli(), Field: "Aviso de atenção", From: "a partir de 40 CPM", To: "a partir de 60 CPM", Source: "você"},
		{T: base.Add(6 * time.Hour).UnixMilli(), Field: "Gravação na memória do GMC", To: "a cada minuto", Source: "contador"},
		{T: base.Add(30 * time.Hour).UnixMilli(), Field: "Alarme", To: "a partir de 100 CPM", Source: "início do registro"},
	}
	var sb strings.Builder
	for _, c := range h {
		b, _ := json.Marshal(c)
		sb.Write(append(b, '\n'))
	}
	os.WriteFile(a.cfgHistPath(), []byte(sb.String()), 0o644)
	T := base.Add(2 * time.Hour).UnixMilli()
	T2 := base.Add(8 * time.Hour).UnixMilli()
	a.events = append(a.events,
		Event{ID: 1, Time: T, End: T + 600000, Kind: kindAlert, Level: "atencao", Peak: 70},
		Event{ID: 2, Time: T2, End: T2 + 300000, Kind: kindAlert, Level: "atencao", Peak: 75, EndReason: endDisconnected},
		Event{ID: 3, Time: T2, End: T2 + 120000, Kind: kindAlert, Level: "alarme", Peak: 120},
	)
	r, err := a.buildReport(base.UnixMilli(), base.Add(24*time.Hour).UnixMilli(), "teste")
	if err != nil {
		t.Fatal(err)
	}
	// linhas: atenção (40 no início, mudou para 60), tempo morto, gravação (registrada no meio)
	if len(r.CfgRows) != 3 || r.CfgRows[0].Field != "Aviso de atenção" || r.CfgRows[0].AtStart != "a partir de 40 CPM" || len(r.CfgRows[0].Changes) != 1 {
		t.Fatalf("configurações: %+v", r.CfgRows)
	}
	if g := r.CfgRows[2]; g.Field != "Gravação na memória do GMC" || g.Known || len(g.Changes) != 1 {
		t.Fatalf("gravação: %+v", g)
	}
	// limites de cada episódio pelo valor da época
	if len(r.Episodes) != 2 || r.Episodes[0].Limit != 40 || r.Episodes[0].LimitNow {
		t.Fatalf("episódio 1: %+v", r.Episodes)
	}
	if e := r.Episodes[1]; !e.LimitNow || e.LevelCls != "alarme" {
		t.Fatalf("episódio 2 (alarme sem registro na época): %+v", e)
	}
	// o que está na tabela do período sai das "Configurações e método"; o resto fica com o valor atual
	var cfg string
	for _, kv := range r.Config {
		cfg += kv[0] + "=" + kv[1] + "\n"
	}
	if strings.Contains(cfg, "Aviso de atenção=") || strings.Contains(cfg, "Tempo morto=") || !strings.Contains(cfg, "Alarme=") || !strings.Contains(cfg, "valor atual") {
		t.Fatalf("método:\n%s", cfg)
	}
	// limites em CPM da época viram µSv/h pela calibração de então (2.36)
	k := r.uAt(base, 100) / 100
	near := func(a, b float64) bool { return math.Abs(a-b) < 1e-9 }
	if a0 := r.Above[0]; len(a0.Limits) != 2 || !near(a0.Limits[0], 40*k) || !near(a0.Limits[1], 60*k) || a0.UsedNow {
		t.Fatalf("tempo acima (atenção): %+v", a0)
	}
	if len(r.Mature.GMC) != 5 || r.Mature.GMC[0].State == matWaiting {
		t.Fatalf("maturidade: %+v", r.Mature)
	}
	b := renderReportPDF(r)
	if os.Getenv("RADLOG_PDF") != "" {
		os.WriteFile(os.Getenv("RADLOG_PDF"), b, 0o644)
	}
	if !strings.HasPrefix(string(b), "%PDF-1.4") {
		t.Fatal("não parece um PDF")
	}
}

func TestCfgLimitAt(t *testing.T) {
	h := []ConfigChange{
		{T: 100, Field: "Alarme", To: "a partir de 100 CPM"},
		{T: 200, Field: "Alarme", To: "desligado"},
		{T: 300, Field: "Alarme de perigo", To: "a partir de 1000 CPM"},
	}
	if l, ok := cfgLimitAt(h, "alarme", 150); !ok || l != 100 {
		t.Fatalf("alarme em 150: %d %v", l, ok)
	}
	if _, ok := cfgLimitAt(h, "alarme", 250); ok {
		t.Fatal("alarme desligado deu limite")
	}
	if _, ok := cfgLimitAt(h, "perigo", 250); ok {
		t.Fatal("perigo antes do registro")
	}
	if l, _ := cfgLimitAt(h, "perigo", 400); l != 1000 {
		t.Fatalf("perigo: %d", l)
	}
}

// desde a 2.29 o histórico guarda o limite em µSv/h
func TestCfgLimitUSvAt(t *testing.T) {
	base := time.Date(2026, 11, 1, 0, 0, 0, 0, time.Local)
	h := []ConfigChange{
		{T: base.UnixMilli(), Field: "Aviso de atenção", To: "a partir de 50 CPM"},
		{T: base.Add(time.Hour).UnixMilli(), Field: "Aviso de atenção", From: "a partir de 50 CPM", To: "a partir de 0,564 µSv/h"},
	}
	var tl calTL
	if u, cpm, ok := cfgLimitUSvAt(h, "atencao", base.Add(time.Minute).UnixMilli(), tl); !ok || cpm != 50 || math.Abs(u-tl.USvAt(base, 50)) > 1e-12 {
		t.Fatalf("em CPM: %v %v %v", u, cpm, ok)
	}
	if u, cpm, ok := cfgLimitUSvAt(h, "atencao", base.Add(2*time.Hour).UnixMilli(), tl); !ok || cpm != 0 || u != 0.564 {
		t.Fatalf("em µSv/h: %v %v %v", u, cpm, ok)
	}
}
