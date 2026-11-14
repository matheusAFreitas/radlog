package app

import (
	"testing"
	"time"
)

// pela mescla da nuvem e da rede: o pacote "comum" de a juntado em b
func recSync(t *testing.T, from, to *App) MergeResult {
	t.Helper()
	z, err := from.buildDelta(pullReq{Rc: true, cloud: true})
	if err != nil {
		t.Fatal(err)
	}
	r, err := to.mergeDelta(z, "teste")
	if err != nil {
		t.Fatal(err)
	}
	return r
}

// N4: editar e apagar uma anotação chega no outro aparelho e não volta.
func TestRecordsNoteEditDelete(t *testing.T) {
	a, _ := testApp(t)
	b, _ := testApp(t)
	t0 := time.Now().Add(-3 * time.Hour).UnixMilli()
	n, err := a.SaveNote(Note{Time: t0, Text: "abri a janela"})
	if err != nil || a.GetNotes(0, 0)[0].UID == "" {
		t.Fatalf("nota: %+v %v", a.GetNotes(0, 0), err)
	}
	if r := recSync(t, a, b); r.Notes != 1 {
		t.Fatalf("chegou: %+v", r)
	}
	time.Sleep(2 * time.Millisecond)
	n = a.GetNotes(0, 0)[0]
	n.Text, n.UID = "fechei a janela", "" // a tela não manda o UID
	if _, err := a.SaveNote(n); err != nil {
		t.Fatal(err)
	}
	recSync(t, a, b)
	if got := b.GetNotes(0, 0); len(got) != 1 || got[0].Text != "fechei a janela" {
		t.Fatalf("edição: %+v", got)
	}
	// a cópia antiga de b não desfaz a edição de a
	recSync(t, b, a)
	if got := a.GetNotes(0, 0); len(got) != 1 || got[0].Text != "fechei a janela" {
		t.Fatalf("volta: %+v", got)
	}
	if err := a.DeleteNote(a.GetNotes(0, 0)[0].ID); err != nil {
		t.Fatal(err)
	}
	if r := recSync(t, a, b); r.Deleted != 1 || len(b.GetNotes(0, 0)) != 0 {
		t.Fatalf("apagar: %+v %+v", r, b.GetNotes(0, 0))
	}
	recSync(t, b, a)
	if len(a.GetNotes(0, 0)) != 0 {
		t.Fatal("a nota apagada voltou")
	}
}

func mustDelta(t *testing.T, a *App) []byte {
	t.Helper()
	z, err := a.buildDelta(pullReq{Rc: true, cloud: true})
	if err != nil {
		t.Fatal(err)
	}
	return z
}

// N4: medição renomeada e depois apagada (com a pasta dos anexos).
func TestRecordsMeasureEditDelete(t *testing.T) {
	a, _ := testApp(t)
	b, _ := testApp(t)
	t0 := time.Now().Add(-time.Hour)
	addTestMeasure(a, 1, "Eletrodo", "", t0)
	a.mu.Lock()
	_ = a.saveMeasurementsLocked()
	a.mu.Unlock()
	addTestMeasure(b, 4, "Outra", "", t0.Add(-48*time.Hour))
	if r := recSync(t, a, b); r.Measurements != 1 {
		t.Fatalf("chegou: %+v", r)
	}
	time.Sleep(2 * time.Millisecond)
	if err := a.UpdateMeasurement(1, "Eletrodo de solda", "toriado"); err != nil {
		t.Fatal(err)
	}
	recSync(t, a, b)
	var id int64
	for _, m := range b.GetMeasurements() {
		if m.Time == t0.UnixMilli() {
			if m.Name != "Eletrodo de solda" || m.Note != "toriado" {
				t.Fatalf("edição: %+v", m)
			}
			id = m.ID
		}
	}
	if id == 0 || len(b.GetMeasurements()) != 2 {
		t.Fatalf("medições em b: %+v", b.GetMeasurements())
	}
	if err := a.DeleteMeasurement(1); err != nil {
		t.Fatal(err)
	}
	recSync(t, a, b)
	if ms := b.GetMeasurements(); len(ms) != 1 || ms[0].Name != "Outra" {
		t.Fatalf("apagar: %+v", ms)
	}
	recSync(t, b, a)
	if len(a.GetMeasurements()) != 1 {
		t.Fatalf("a: %+v", a.GetMeasurements())
	}
}

// N4: saída de casa editada (fim mudado) e apagada.
func TestRecordsPeriodEditDelete(t *testing.T) {
	a, _ := testApp(t)
	b, _ := testApp(t)
	from := time.Now().Add(-10 * time.Hour).Truncate(time.Minute)
	p, err := a.SavePeriod(Period{From: from.UnixMilli(), To: from.Add(2 * time.Hour).UnixMilli(), Name: "passeio"})
	if err != nil {
		t.Fatal(err)
	}
	recSync(t, a, b)
	time.Sleep(2 * time.Millisecond)
	uid := a.GetPeriods()[0].UID
	p.To = from.Add(3 * time.Hour).UnixMilli()
	if _, err := a.SavePeriod(p); err != nil {
		t.Fatal(err)
	}
	recSync(t, a, b)
	if ps := b.GetPeriods(); len(ps) != 1 || ps[0].To != p.To || ps[0].UID != uid {
		t.Fatalf("edição: %+v", ps)
	}
	if err := a.DeletePeriod(p.ID); err != nil {
		t.Fatal(err)
	}
	recSync(t, a, b)
	if ps := b.GetPeriods(); len(ps) != 0 {
		t.Fatalf("apagar: %+v", ps)
	}
}

// itens de antes da 2.80 ganham o mesmo UID nos dois aparelhos
func TestRecordsLegacyUID(t *testing.T) {
	a, _ := testApp(t)
	n := Note{ID: 3, Time: 1_700_000_000_000, Text: "velha"}
	a.notes = []Note{n}
	a.notesTrackLocked(true)
	b, _ := testApp(t)
	n.ID = 9
	b.notes = []Note{n}
	b.notesTrackLocked(true)
	if a.notes[0].UID == "" || a.notes[0].UID != b.notes[0].UID {
		t.Fatalf("uid: %q %q", a.notes[0].UID, b.notes[0].UID)
	}
	if r := recSync(t, a, b); r.Notes != 0 || len(b.notes) != 1 {
		t.Fatalf("duplicou: %+v", b.notes)
	}
}
