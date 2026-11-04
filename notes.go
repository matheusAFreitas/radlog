package main

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

// ---------- anotações no gráfico ----------
//
// Uma nota marca um momento ("abri a janela", "liguei o ar") para explicar
// depois uma subida que não foi chuva nem alerta. Ficam em anotacoes.json.

type Note struct {
	ID      int64  `json:"id"`
	Time    int64  `json:"time"` // ms (começo, se for um trecho)
	End     int64  `json:"end"`  // ms; 0 = um momento só
	Text    string `json:"text"`
	Created int64  `json:"created"`
}

const noteMaxLen = 200

func (a *App) notesPath() string { return filepath.Join(a.dataDir, "anotacoes.json") }

func (a *App) loadNotes() {
	b, err := os.ReadFile(a.notesPath())
	if err != nil {
		return
	}
	var list []Note
	if json.Unmarshal(b, &list) == nil {
		sort.Slice(list, func(i, j int) bool { return list[i].Time < list[j].Time })
		a.notes = list
	}
}

func (a *App) saveNotesLocked() error {
	if a.dataDir == "" {
		return nil
	}
	b, _ := json.MarshalIndent(a.notes, "", "  ")
	tmp := a.notesPath() + ".tmp"
	if err := os.WriteFile(tmp, b, 0o644); err != nil {
		return err
	}
	return os.Rename(tmp, a.notesPath())
}

// GetNotes devolve as notas em [from, to) (ms); 0 e 0 = todas.
func (a *App) GetNotes(fromMs, toMs int64) []Note {
	a.mu.Lock()
	defer a.mu.Unlock()
	out := []Note{}
	for _, n := range a.notes {
		end := max(n.End, n.Time)
		if (fromMs == 0 && toMs == 0) || (end >= fromMs && n.Time < toMs) {
			out = append(out, n)
		}
	}
	return out
}

// SaveNote cria (ID 0) ou edita uma nota.
func (a *App) SaveNote(n Note) (Note, error) {
	n.Text = strings.TrimSpace(n.Text)
	if n.Text == "" {
		return n, errors.New("escreva o texto da anotação")
	}
	if r := []rune(n.Text); len(r) > noteMaxLen {
		n.Text = string(r[:noteMaxLen])
	}
	if n.Time <= 0 {
		n.Time = time.Now().UnixMilli()
	}
	if n.Time > time.Now().Add(time.Minute).UnixMilli() {
		return n, errors.New("a anotação não pode ficar no futuro")
	}
	if n.End != 0 {
		if n.End <= n.Time {
			return n, errors.New("o fim do trecho tem que ser depois do começo")
		}
		if n.End > time.Now().Add(time.Minute).UnixMilli() {
			n.End = time.Now().UnixMilli()
		}
	}
	a.mu.Lock()
	if n.ID == 0 {
		var max int64
		for _, q := range a.notes {
			max = maxInt64(max, q.ID)
		}
		n.ID, n.Created = max+1, time.Now().UnixMilli()
		a.notes = append(a.notes, n)
	} else {
		found := false
		for i := range a.notes {
			if a.notes[i].ID == n.ID {
				n.Created = a.notes[i].Created
				a.notes[i] = n
				found = true
			}
		}
		if !found {
			a.mu.Unlock()
			return n, errors.New("anotação não encontrada")
		}
	}
	sort.Slice(a.notes, func(i, j int) bool { return a.notes[i].Time < a.notes[j].Time })
	err := a.saveNotesLocked()
	a.mu.Unlock()
	a.emit("notes", a.GetNotes(0, 0))
	return n, err
}

// DeleteNote apaga uma nota.
func (a *App) DeleteNote(id int64) error {
	a.mu.Lock()
	kept := a.notes[:0]
	for _, n := range a.notes {
		if n.ID != id {
			kept = append(kept, n)
		}
	}
	a.notes = kept
	err := a.saveNotesLocked()
	a.mu.Unlock()
	a.emit("notes", a.GetNotes(0, 0))
	return err
}

// mergeNotes junta notas de um backup (mesma hora e texto = a mesma).
func (a *App) mergeNotes(list []Note) int {
	a.mu.Lock()
	defer a.mu.Unlock()
	have := map[string]bool{}
	var max int64
	for _, n := range a.notes {
		have[noteKey(n)] = true
		max = maxInt64(max, n.ID)
	}
	added := 0
	for _, n := range list {
		if strings.TrimSpace(n.Text) == "" || have[noteKey(n)] {
			continue
		}
		have[noteKey(n)] = true
		max++
		n.ID = max
		a.notes = append(a.notes, n)
		added++
	}
	if added > 0 {
		sort.Slice(a.notes, func(i, j int) bool { return a.notes[i].Time < a.notes[j].Time })
		_ = a.saveNotesLocked()
	}
	return added
}

func noteKey(n Note) string {
	k := strings.TrimSpace(n.Text) + "|" + time.UnixMilli(n.Time).Format("20060102150405")
	if n.End != 0 {
		k += "|" + time.UnixMilli(n.End).Format("20060102150405")
	}
	return k
}
