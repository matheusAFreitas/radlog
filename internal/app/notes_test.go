package app

import (
	"path/filepath"
	"testing"
	"time"
)

func TestNotes(t *testing.T) {
	a, _ := testApp(t)
	t0 := time.Now().Add(-2 * time.Hour).UnixMilli()
	n, err := a.SaveNote(Note{Time: t0, Text: "  abri a janela  "})
	if err != nil || n.ID != 1 || n.Text != "abri a janela" {
		t.Fatalf("criar: %+v %v", n, err)
	}
	if _, err := a.SaveNote(Note{Text: "   "}); err == nil {
		t.Fatal("aceitou nota vazia")
	}
	if _, err := a.SaveNote(Note{Time: time.Now().Add(time.Hour).UnixMilli(), Text: "futuro"}); err == nil {
		t.Fatal("aceitou nota no futuro")
	}
	n.Text = "liguei o ar"
	if n2, err := a.SaveNote(n); err != nil || n2.Text != "liguei o ar" || len(a.GetNotes(0, 0)) != 1 {
		t.Fatal("editar falhou")
	}
	if len(a.GetNotes(t0-1, t0+1)) != 1 || len(a.GetNotes(t0+1, t0+2)) != 0 {
		t.Fatal("filtro por período")
	}
	// recarrega do arquivo
	b := NewApp()
	b.dataDir = a.dataDir
	b.loadNotes()
	if len(b.notes) != 1 || b.notes[0].Text != "liguei o ar" {
		t.Fatalf("arquivo: %+v", b.notes)
	}
	// backup leva as notas e não duplica
	c, _ := testApp(t)
	zip := filepath.Join(t.TempDir(), "n.zip")
	if err := a.writeBackupFile(zip); err != nil {
		t.Fatal(err)
	}
	if r, _ := c.mergeBackup(zip); r.Notes != 1 {
		t.Fatalf("backup: %+v", r)
	}
	if r, _ := c.mergeBackup(zip); r.Notes != 0 {
		t.Fatal("duplicou nota")
	}
	// trecho
	r, err := a.SaveNote(Note{Time: t0 - 3600e3, End: t0 - 1800e3, Text: "visita"})
	if err != nil || r.End == 0 {
		t.Fatalf("trecho: %v", err)
	}
	if len(a.GetNotes(t0-2000e3, t0-1900e3)) != 1 { // dentro do trecho
		t.Fatal("trecho não aparece no meio")
	}
	if _, err := a.SaveNote(Note{Time: t0, End: t0 - 1, Text: "x"}); err == nil {
		t.Fatal("aceitou fim antes do começo")
	}
	_ = a.DeleteNote(r.ID)
	if err := a.DeleteNote(n.ID); err != nil || len(a.GetNotes(0, 0)) != 0 {
		t.Fatal("apagar falhou")
	}
}
