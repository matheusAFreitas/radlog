package app

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"time"
)

// ---------- editar e apagar entre aparelhos (auditoria 2.79.0, N4) ----------
//
// Anotações, medições e saídas de casa ganham um UID (igual em todos os
// aparelhos) e a hora da última edição. Ninguém precisa lembrar de marcar:
// a cada gravação, o app compara cada item com a gravação anterior; o que
// mudou ganha a hora de agora e o que sumiu vira uma marca de apagado em
// apagados.json. Na mescla (nuvem, rede, backup), a edição mais nova vale e
// o apagado vence: o item não volta.
//
// Itens de antes da 2.80 ganham um UID tirado deles mesmos (o mesmo em
// todos os aparelhos que já tinham a cópia); os novos ganham um sorteado.

const (
	recNote    = "n"
	recMeasure = "m"
	recPeriod  = "p"
	tombsFile  = "apagados.json"
)

// tomb é a marca de um item apagado.
type tomb struct {
	K  string `json:"k"`  // n, m ou p
	ID string `json:"id"` // UID do item
	T  int64  `json:"t"`  // ms
}

func legacyNoteUID(n Note) string {
	h := sha256.Sum256([]byte(n.Text + "|" + strconv.FormatInt(n.Time, 10) + "|" + strconv.FormatInt(n.End, 10)))
	return "n-" + hex.EncodeToString(h[:8])
}

func legacyMeasureUID(m SavedMeasure) string { return "m-" + strconv.FormatInt(m.Time/1000, 10) }

func legacyPeriodUID(p Period) string { return "p-" + strconv.FormatInt(p.From, 10) }

func recHashOf(v any) string {
	b, _ := json.Marshal(v)
	h := sha256.Sum256(b)
	return hex.EncodeToString(h[:8])
}

// recTrackLocked guarda o resumo de cada item e devolve os que mudaram desde
// a gravação anterior; os que sumiram viram marca de apagado. Na carga
// (init) e na mescla (a.recMerge) só guarda; sem carga antes (testes), tudo
// conta como novo.
func (a *App) recTrackLocked(k string, cur map[string]string, init bool) map[string]bool {
	if a.recHash == nil {
		a.recHash = map[string]map[string]string{}
	}
	prev := a.recHash[k]
	a.recHash[k] = cur
	if init || a.recMerge {
		return nil
	}
	changed := map[string]bool{}
	for uid, h := range cur {
		if prev[uid] != h {
			changed[uid] = true
		}
	}
	now := time.Now().UnixMilli()
	gone := false
	for uid := range prev {
		if _, ok := cur[uid]; !ok {
			a.tombsLoadLocked()
			if _, ok := a.tombs[k+"|"+uid]; !ok {
				a.tombs[k+"|"+uid] = now
				gone = true
			}
		}
	}
	if gone {
		a.saveTombsLocked()
	}
	return changed
}

// newUID: itens já gravados (init) ganham o UID tirado deles; os novos, um
// sorteado.
func newUID(init bool, legacy func() string) string {
	if init {
		return legacy()
	}
	return randHex(8)
}

func (a *App) notesTrackLocked(init bool) {
	cur := map[string]string{}
	for i := range a.notes {
		n := &a.notes[i]
		if n.UID == "" {
			c := *n
			n.UID = newUID(init, func() string { return legacyNoteUID(c) })
		}
		c := *n
		c.ID, c.Edited = 0, 0
		cur[n.UID] = recHashOf(c)
	}
	ch := a.recTrackLocked(recNote, cur, init)
	now := time.Now().UnixMilli()
	for i := range a.notes {
		if ch[a.notes[i].UID] {
			a.notes[i].Edited = now
		}
	}
}

func (a *App) measurementsTrackLocked(init bool) {
	cur := map[string]string{}
	for i := range a.measurements {
		m := &a.measurements[i]
		if m.UID == "" {
			c := *m
			m.UID = newUID(init, func() string { return legacyMeasureUID(c) })
		}
		c := *m
		c.ID, c.Edited = 0, 0
		cur[m.UID] = recHashOf(c)
	}
	ch := a.recTrackLocked(recMeasure, cur, init)
	now := time.Now().UnixMilli()
	for i := range a.measurements {
		if ch[a.measurements[i].UID] {
			a.measurements[i].Edited = now
		}
	}
}

func (a *App) periodsTrackLocked(init bool) {
	cur := map[string]string{}
	for i := range a.periods {
		p := &a.periods[i]
		if p.UID == "" {
			c := *p
			p.UID = newUID(init, func() string { return legacyPeriodUID(c) })
		}
		c := *p
		c.ID, c.Edited = 0, 0
		cur[p.UID] = recHashOf(c)
	}
	ch := a.recTrackLocked(recPeriod, cur, init)
	now := time.Now().UnixMilli()
	for i := range a.periods {
		if ch[a.periods[i].UID] {
			a.periods[i].Edited = now
		}
	}
}

// ---------- marcas de apagado ----------

func (a *App) tombsPath() string { return filepath.Join(a.dataDir, tombsFile) }

func (a *App) tombsLoadLocked() {
	if a.tombs != nil {
		return
	}
	a.tombs = map[string]int64{}
	if a.dataDir == "" {
		return
	}
	var list []tomb
	if a.readStateJSON(a.tombsPath(), "apagados", &list) {
		for _, t := range list {
			if t.K != "" && t.ID != "" {
				a.tombs[t.K+"|"+t.ID] = t.T
			}
		}
	}
}

func (a *App) tombList() []tomb {
	out := make([]tomb, 0, len(a.tombs))
	for key, t := range a.tombs {
		k, id := key[:1], key[2:]
		out = append(out, tomb{K: k, ID: id, T: t})
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].K != out[j].K {
			return out[i].K < out[j].K
		}
		return out[i].ID < out[j].ID
	})
	return out
}

func (a *App) saveTombsLocked() {
	if a.dataDir == "" {
		return
	}
	b, _ := json.MarshalIndent(a.tombList(), "", "  ")
	if err := writeFileAtomic(a.tombsPath(), b, 0o644); err != nil {
		a.logf("não consegui gravar os apagados (%v)", err)
	}
}

func (a *App) tombed(k, uid string) bool {
	_, ok := a.tombs[k+"|"+uid]
	return ok
}

// applyTombs junta as marcas de apagado de outro aparelho e apaga aqui os
// itens marcados. Devolve quantos itens saíram.
func (a *App) applyTombs(list []tomb) int {
	a.mu.Lock()
	a.tombsLoadLocked()
	added := false
	gone := map[string]bool{}
	for _, t := range list {
		if (t.K != recNote && t.K != recMeasure && t.K != recPeriod) || t.ID == "" || a.tombed(t.K, t.ID) {
			continue
		}
		a.tombs[t.K+"|"+t.ID] = t.T
		gone[t.K+"|"+t.ID] = true
		added = true
	}
	n := 0
	var dirs []int64
	if added {
		a.recMerge = true
		notes := a.notes[:0]
		for _, x := range a.notes {
			if gone[recNote+"|"+x.UID] {
				n++
				continue
			}
			notes = append(notes, x)
		}
		if len(notes) != len(a.notes) {
			a.notes = notes
			_ = a.saveNotesLocked()
		}
		ms := a.measurements[:0]
		for _, x := range a.measurements {
			if gone[recMeasure+"|"+x.UID] {
				dirs = append(dirs, x.ID)
				n++
				continue
			}
			ms = append(ms, x)
		}
		if len(ms) != len(a.measurements) {
			a.measurements = ms
			a.bgAt = time.Time{}
			_ = a.saveMeasurementsLocked()
		}
		ps := a.periods[:0]
		for _, x := range a.periods {
			if gone[recPeriod+"|"+x.UID] {
				n++
				continue
			}
			ps = append(ps, x)
		}
		if len(ps) != len(a.periods) {
			a.periods = ps
			a.bgAt = time.Time{}
			_ = a.savePeriodsLocked()
		}
		a.recMerge = false
		a.saveTombsLocked()
	}
	a.mu.Unlock()
	if a.dataDir != "" {
		for _, id := range dirs {
			_ = os.RemoveAll(a.attachDir(id))
		}
	}
	return n
}

// tombsJSON é o apagados.json para o pacote (nil = nenhum).
func (a *App) tombsJSON() []byte {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.tombsLoadLocked()
	if len(a.tombs) == 0 {
		return nil
	}
	b, _ := json.Marshal(a.tombList())
	return b
}
