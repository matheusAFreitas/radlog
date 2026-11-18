package app

import (
	"encoding/json"
	"fmt"
	"path/filepath"
	"time"
)

// ---------- "Estou testando um objeto" (2.89.0) ----------
//
// Avaliação de UX da 2.87.2, achado A5: para encostar um aparelho numa pedra
// sem abrir o Medir objeto, uma ação cala atenção e alarme daquele aparelho
// por alguns minutos. O perigo continua valendo e tocando. A leitura na tela
// não muda; só os alertas ficam quietos.

const objTestDur = 10 * time.Minute

// objTestDev aceita "gmc" e "radiacode".
func objTestDev(dev string) (string, error) {
	switch dev {
	case "gmc", devRC:
		return dev, nil
	}
	return "", fmt.Errorf("aparelho desconhecido %q", dev)
}

// TestObject começa (ou estende) os 10 min de silêncio de atenção e alarme do
// aparelho e devolve até quando (ms).
func (a *App) TestObject(dev string) (int64, error) {
	d, err := objTestDev(dev)
	if err != nil {
		return 0, err
	}
	until := time.Now().Add(objTestDur)
	a.mu.Lock()
	if a.objTest == nil {
		a.objTest = map[string]time.Time{}
	}
	a.objTest[d] = until
	a.mu.Unlock()
	a.objSpanSet(d, time.Now(), until)
	name := "GMC-800"
	if d == devRC {
		name = "RC102"
	}
	a.logEvent(fmt.Sprintf("Testando um objeto no %s: atenção e alarme calados até %s", name, until.Format("15:04")))
	return until.UnixMilli(), nil
}

// StopTestObject devolve os alertas do aparelho antes da hora.
func (a *App) StopTestObject(dev string) {
	a.mu.Lock()
	_, on := a.objTest[dev]
	delete(a.objTest, dev)
	a.mu.Unlock()
	if on {
		a.objSpanSet(dev, time.Time{}, time.Now())
	}
}

// ---------- os minutos do teste ficam fora da dose e do fundo (2.101.5) ----------

const objTestFile = "testes-objeto.json"

// objSpan é um intervalo com o aparelho encostado num objeto.
type objSpan struct {
	Dev  string `json:"dev"`
	From int64  `json:"from"`
	To   int64  `json:"to"`
}

func (a *App) objSpansLocked() []objSpan {
	if !a.objSpansLoaded {
		a.objSpansLoaded = true
		if a.dataDir != "" {
			a.readStateJSON(filepath.Join(a.dataDir, objTestFile), "testes de objeto", &a.objSpans)
		}
	}
	return a.objSpans
}

// objSpanSet abre (from ≠ 0) ou estende/fecha o intervalo do aparelho que
// toca agora; guarda só os últimos 400.
func (a *App) objSpanSet(dev string, from, to time.Time) {
	a.mu.Lock()
	list := a.objSpansLocked()
	now := time.Now().UnixMilli()
	n := len(list) - 1
	for ; n >= 0; n-- {
		if list[n].Dev == dev {
			break
		}
	}
	if n >= 0 && list[n].To >= now {
		list[n].To = to.UnixMilli() // estende (de novo) ou fecha (parou antes)
	} else if !from.IsZero() {
		list = append(list, objSpan{Dev: dev, From: from.UnixMilli(), To: to.UnixMilli()})
	}
	if len(list) > 400 {
		list = list[len(list)-400:]
	}
	a.objSpans = list
	b, _ := json.Marshal(list)
	dir := a.dataDir
	a.mu.Unlock()
	if dir != "" {
		_ = writeFileAtomic(filepath.Join(dir, objTestFile), b, 0o644)
	}
}

// ObjectTests diz até quando cada aparelho está calado (ms); só os que valem.
func (a *App) ObjectTests() map[string]int64 {
	now := time.Now()
	out := map[string]int64{}
	a.mu.Lock()
	defer a.mu.Unlock()
	for d, t := range a.objTest {
		if t.After(now) {
			out[d] = t.UnixMilli()
		}
	}
	return out
}

// objTestQuietLocked: atenção e alarme do aparelho ficam calados agora.
func (a *App) objTestQuietLocked(dev string, now time.Time) bool {
	t, ok := a.objTest[dev]
	if ok && !now.Before(t) {
		delete(a.objTest, dev)
		return false
	}
	return ok
}
