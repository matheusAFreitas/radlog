package app

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"radlog/internal/gmc"
	"radlog/internal/store"
)

func addTestMeasure(a *App, id int64, name, dist string, t time.Time) {
	a.mu.Lock()
	a.measurements = append(a.measurements, SavedMeasure{ID: id, Name: name, Distance: dist, Time: t.UnixMilli(),
		Bg: MeasurePhase{Start: t.UnixMilli(), End: t.Add(time.Minute).UnixMilli()}, Obj: MeasurePhase{Start: t.Add(time.Minute).UnixMilli(), End: t.Add(2 * time.Minute).UnixMilli()},
		Result: MeasureResult{NetCPM: 30, NetUSv: 0.3, NetErr: 3, Level: "sim"}})
	a.mu.Unlock()
}

func TestMeasureAttachments(t *testing.T) {
	a := newTestApp(t)
	addTestMeasure(a, 1, "Eletrodo", "encostado", time.Now().Add(-time.Hour))
	src := filepath.Join(t.TempDir(), "foto do eletrodo.png")
	os.WriteFile(src, []byte("\\x89PNG fake"), 0o644)
	bad := filepath.Join(t.TempDir(), "x/../../etc")
	list, err := a.attachFiles(1, []string{src, src, bad})
	if err == nil || !strings.Contains(err.Error(), "não encontrado") {
		t.Fatalf("erro esperado do arquivo que não existe: %v", err)
	}
	if len(list) != 2 || list[0].File != "foto do eletrodo.png" || list[1].File != "foto do eletrodo-2.png" || list[0].Type != "image/png" {
		t.Fatalf("anexos: %+v", list)
	}
	if _, err := os.Stat(filepath.Join(a.attachDir(1), list[1].File)); err != nil {
		t.Fatal("não copiou")
	}
	u, err := a.GetMeasurementAttachment(1, list[0].File)
	if err != nil || !strings.HasPrefix(u, "data:image/png;base64,") {
		t.Fatalf("data URL: %v %q", err, u[:min(40, len(u))])
	}
	if _, err := a.GetMeasurementAttachment(1, "../medicoes.json"); err == nil {
		t.Fatal("leu fora da lista")
	}
	if err := a.RemoveMeasurementAttachment(1, list[1].File); err != nil || len(a.measurementAttachments(1)) != 1 {
		t.Fatalf("remover: %v", err)
	}
	if safeFileName("../../a\\b:c?.png") != "b_c_.png" && safeFileName("../../a\\b:c?.png") == "" {
		t.Fatal("nome")
	}

	// backup: o outro PC tem uma medição com o mesmo id; o anexo vai para o id novo
	var buf bytes.Buffer
	if err := a.writeBackup(&buf); err != nil {
		t.Fatal(err)
	}
	b := newTestApp(t)
	addTestMeasure(b, 1, "Outra coisa", "", time.Now().Add(-48*time.Hour))
	zp := filepath.Join(t.TempDir(), "b.zip")
	os.WriteFile(zp, buf.Bytes(), 0o644)
	if _, err := b.mergeZip(zp, true); err != nil {
		t.Fatal(err)
	}
	var got *SavedMeasure
	for i := range b.measurements {
		if b.measurements[i].Name == "Eletrodo" {
			got = &b.measurements[i]
		}
	}
	if got == nil || got.ID == 1 || len(got.Attachments) != 1 {
		t.Fatalf("medição no outro PC: %+v", got)
	}
	if _, err := os.Stat(filepath.Join(b.attachDir(got.ID), "foto do eletrodo.png")); err != nil {
		t.Fatalf("anexo no outro PC: %v", err)
	}
	if _, err := os.Stat(filepath.Join(b.attachDir(1), "foto do eletrodo.png")); err == nil {
		t.Fatal("anexo foi para a medição errada")
	}

	// apagar a medição apaga os anexos
	if err := a.DeleteMeasurement(1); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(a.attachDir(1)); err == nil {
		t.Fatal("pasta de anexos ficou")
	}
}

func TestMeasureContext(t *testing.T) {
	a := newTestApp(t)
	a.loadCalHist()
	t0 := time.Now().Add(-10 * 24 * time.Hour)
	h := []ConfigChange{
		{T: t0.Add(-time.Hour).UnixMilli(), Field: "Calibração em uso", To: "0,01014 µSv/h por CPM no fundo (curva do contador)"},
		{T: t0.Add(-time.Hour).UnixMilli(), Field: "Tempo morto", To: "desligado"},
		{T: t0.Add(5 * 24 * time.Hour).UnixMilli(), Field: "Calibração em uso", From: "0,01014 µSv/h por CPM no fundo (curva do contador)", To: "0,00550 µSv/h por CPM no fundo (informado à mão)"},
	}
	var sb strings.Builder
	for _, c := range h {
		b, _ := json.Marshal(c)
		sb.Write(append(b, '\n'))
	}
	os.WriteFile(a.cfgHistPath(), []byte(sb.String()), 0o644)
	addTestMeasure(a, 1, "Castanha-do-pará", "encostado", t0)
	addTestMeasure(a, 2, "castanha-do-pará ", "encostado", t0.Add(24*time.Hour))
	addTestMeasure(a, 3, "Castanha-do-Pará", "5 cm", t0.Add(7*24*time.Hour))
	addTestMeasure(a, 4, "Outra", "encostado", t0)
	if err := a.SetMeasurementComparable(2, false, "outro pote"); err != nil {
		t.Fatal(err)
	}
	ctx, err := a.GetMeasurementContext(1)
	if err != nil {
		t.Fatal(err)
	}
	if !ctx.CfgKnown || len(ctx.Changed) != 0 || len(ctx.Related) != 2 {
		t.Fatalf("contexto: %+v", ctx)
	}
	r3, r2 := ctx.Related[0], ctx.Related[1]
	if r3.ID != 3 || !strings.Contains(strings.Join(r3.Diff, ";"), "outra distância") || !strings.Contains(strings.Join(r3.Diff, ";"), "calibração em uso") {
		t.Fatalf("medição 3: %+v", r3)
	}
	if r2.ID != 2 || !r2.NotComparable || r2.NCReason != "outro pote" || len(r2.Diff) != 1 {
		t.Fatalf("medição 2: %+v", r2)
	}
	// a configuração atual (sem calibração manual) difere da 3, não da 1? a atual vem do app
	ctx3, _ := a.GetMeasurementContext(3)
	if len(ctx3.Related) != 2 || len(ctx3.Related[0].Diff) == 0 {
		t.Fatalf("contexto 3: %+v", ctx3)
	}
	if err := a.SetMeasurementComparable(2, true, "x"); err != nil || a.measurements[1].NotComparable || a.measurements[1].NCReason != "" {
		t.Fatal("desmarcar")
	}
	rows := a.measureExtraRows(a.measurements[0])
	if len(rows) == 0 || rows[0][0] != "Configuração na época" {
		t.Fatalf("linhas do PDF: %+v", rows)
	}
}

func TestCounterSyncState(t *testing.T) {
	a := newTestApp(t)
	now := time.Now().Truncate(time.Minute)
	if cs := a.GetCounterSync(); cs.State != "nunca" {
		t.Fatalf("sem download: %+v", cs)
	}
	// download há 1 dia; memória até 20 h atrás; o app tem dado até 6 h atrás e depois nada
	h := gmc.History{}
	for m := 24 * 60; m >= 20*60; m-- {
		h.Records = append(h.Records, gmc.Record{Time: now.Add(-time.Duration(m) * time.Minute), Count: 20, Unit: "CPM", Interval: time.Minute})
	}
	a.mu.Lock()
	a.devSaveMode = "a cada minuto"
	a.mu.Unlock()
	a.recordCounterSync(h, ImportResult{Records: len(h.Records), Added: 10}, true)
	var pts []store.Point
	for m := 20 * 60; m >= 6*60; m-- {
		pts = append(pts, store.Point{T: now.Add(-time.Duration(m) * time.Minute), CPM: 20, Src: store.SrcLive})
	}
	a.store.Import(pts)
	cs := a.counterSyncAt(now)
	if cs.State != "pendente" || cs.PendingMinutes < 6*60-5 || cs.PendingMinutes > 6*60+2 || !strings.Contains(cs.Text, "baixe o histórico") {
		t.Fatalf("pendente: %+v", cs)
	}
	// com dado até agora: em dia
	var more []store.Point
	for m := 6*60 - 1; m >= 1; m-- {
		more = append(more, store.Point{T: now.Add(-time.Duration(m) * time.Minute), CPM: 20, Src: store.SrcLive})
	}
	a.store.Import(more)
	if cs = a.counterSyncAt(now); cs.State != "em-dia" {
		t.Fatalf("em dia: %+v", cs)
	}
	// gravação desligada
	a.mu.Lock()
	a.devSaveMode = "desligada"
	a.mu.Unlock()
	if cs = a.counterSyncAt(now); cs.State != "sem-gravacao" {
		t.Fatalf("sem gravação: %+v", cs)
	}
	// memória: 85% no download, a cada segundo → passa de 95% logo
	a.mu.Lock()
	a.devSaveMode = "a cada segundo"
	a.mu.Unlock()
	mi := MemoryInfo{Known: true, UsedBytes: gmc.FlashSize * 85 / 100, At: now.Add(-time.Hour).UnixMilli()}
	b, _ := json.Marshal(mi)
	os.WriteFile(a.memoryPath(), b, 0o644)
	cs = a.counterSyncAt(now)
	if cs.MemLevel == "" || cs.EstPct <= 85 || cs.FullIn <= 0 {
		t.Fatalf("memória: %+v", cs)
	}
	a.checkMemoryWarning()
	n := countEvents(a, "Memória do GMC")
	a.checkMemoryWarning() // não repete
	if n != 1 || countEvents(a, "Memória do GMC") != 1 {
		t.Fatalf("avisos: %d", countEvents(a, "Memória do GMC"))
	}
}

func countEvents(a *App, prefix string) int {
	a.mu.Lock()
	defer a.mu.Unlock()
	n := 0
	for _, e := range a.events {
		if strings.HasPrefix(e.Text, prefix) {
			n++
		}
	}
	return n
}
