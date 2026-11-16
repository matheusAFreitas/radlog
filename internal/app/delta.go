package app

import (
	"archive/zip"
	"bytes"
	"crypto/rand"
	"encoding/csv"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"hash/fnv"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"radlog/internal/store"
)

// ---------- pacote do que falta (o que sobrou da sincronização pela rede) ----------
//
// A troca pela rede local (sync.go, pareamento CPace) saiu na 2.85.0: a nuvem
// substitui. Daqui ficam o inventário dos dias e o pacote do que falta, que a
// nuvem (cloudsync.go) usa do mesmo jeito.

// invCache guarda o resumo de cada arquivo de mês (refeito só quando o
// arquivo muda).
type invCache struct {
	mu  sync.Mutex
	inv map[string]invMonth
}

type invMonth struct {
	mod  time.Time
	size int64
	days map[string]string
}

func randHex(n int) string {
	b := make([]byte, n)
	_, _ = rand.Read(b)
	return hex.EncodeToString(b)
}

func containsStr(list []string, s string) bool {
	for _, x := range list {
		if x == s {
			return true
		}
	}
	return false
}

type pullReq struct {
	Days    []string `json:"days"`
	Alerts  []string `json:"alerts"`
	RcDays  []string `json:"rcDays,omitempty"`
	RcFiles []string `json:"rcFiles,omitempty"`
	Anexos  []string `json:"anexos,omitempty"`
	Rc      bool     `json:"rc,omitempty"` // quem pede entende o pacote novo
	// nuvem (cloud.go): sem o nome do PC e com o manifesto fixo, para o pacote
	// sair igual quando os dados não mudaram
	cloud bool
	// sem os pequenos de sempre (na nuvem eles vão no pacote "comum")
	noSmall bool
}

// dayDigests resume os minutos gravados de cada dia (UTC): "n:hash". Dois
// PCs com o mesmo resumo num dia têm os mesmos minutos.
func (a *App) dayDigests() (map[string]string, error) {
	if a.store == nil {
		return map[string]string{}, nil
	}
	files, err := a.store.MonthFiles()
	if err != nil {
		return nil, err
	}
	s := &a.inv
	s.mu.Lock()
	if s.inv == nil {
		s.inv = map[string]invMonth{}
	}
	s.mu.Unlock()
	out := map[string]string{}
	for _, fi := range files {
		s.mu.Lock()
		c, ok := s.inv[fi.Name()]
		s.mu.Unlock()
		if !ok || !c.mod.Equal(fi.ModTime()) || c.size != fi.Size() {
			pts, err := a.store.ReadMonthFile(fi.Name())
			if err != nil {
				return nil, err
			}
			c = invMonth{mod: fi.ModTime(), size: fi.Size(), days: digestDays(pts)}
			s.mu.Lock()
			s.inv[fi.Name()] = c
			s.mu.Unlock()
		}
		for d, h := range c.days {
			out[d] = h
		}
	}
	return out, nil
}

func digestDays(pts []store.Point) map[string]string {
	mins := map[string][]int64{}
	for _, p := range pts {
		d := p.T.UTC().Format("2006-01-02")
		mins[d] = append(mins[d], p.T.Unix()/60)
	}
	out := map[string]string{}
	for d, m := range mins {
		sort.Slice(m, func(i, j int) bool { return m[i] < m[j] })
		h := fnv.New64a()
		n := 0
		for i, x := range m {
			if i > 0 && x == m[i-1] {
				continue
			}
			n++
			h.Write([]byte(strconv.FormatInt(x, 36) + ","))
		}
		out[d] = fmt.Sprintf("%d:%x", n, h.Sum64())
	}
	return out
}

// alertFiles lista os segundos de alerta guardados.
func (a *App) alertFiles() []string {
	files, _ := filepath.Glob(filepath.Join(a.alertDir(), "*.json"))
	out := make([]string, 0, len(files))
	for _, f := range files {
		out = append(out, filepath.Base(f))
	}
	return out
}

// buildDelta monta um "backup" só com os dias e segundos de alerta pedidos,
// mais os alertas encerrados, períodos, medições e anotações (pequenos).
func (a *App) buildDelta(req pullReq) ([]byte, error) {
	days, alerts := req.Days, req.Alerts
	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)
	bm := backupManifest{App: backupAppID, Version: AppVersion, Host: hostName(), Created: time.Now().Format(time.RFC3339)}
	if req.cloud {
		bm = backupManifest{App: backupAppID, Version: "nuvem"}
	}
	man, _ := json.Marshal(bm)
	if f, err := zw.Create("backup.json"); err == nil {
		_, _ = f.Write(man)
	}
	if len(days) > 0 && a.store != nil {
		var pts []store.Point
		for _, d := range days {
			t, err := time.Parse("2006-01-02", d)
			if err != nil {
				continue
			}
			p, err := a.store.Range(t, t.Add(24*time.Hour))
			if err != nil {
				return nil, err
			}
			pts = append(pts, p...)
		}
		if f, err := zw.Create("leituras/sincronizacao.csv"); err == nil {
			if err := store.WriteCSV(f, pts); err != nil {
				return nil, err
			}
		}
	}
	if !req.noSmall {
		// só alertas encerrados (os de agora ainda mudam)
		a.mu.Lock()
		var rows [][]string
		for _, e := range a.events {
			if e.Kind == kindAlert && !e.Demo && (e.End != 0 || e.Retro) {
				rows = append(rows, eventRow(e))
			}
		}
		a.mu.Unlock()
		if f, err := zw.Create("eventos.csv"); err == nil {
			_ = csv.NewWriter(f).WriteAll(rows)
		}
		if b := a.tombsJSON(); b != nil {
			if f, err := zw.Create(tombsFile); err == nil {
				_, _ = f.Write(b)
			}
		}
		for _, name := range []string{"fora-de-casa.json", "medicoes.json", "anotacoes.json"} {
			if b, err := os.ReadFile(filepath.Join(a.dataDir, name)); err == nil {
				if f, err := zw.Create(name); err == nil {
					_, _ = f.Write(b)
				}
			}
		}
	}
	for _, n := range alerts {
		n = filepath.Base(n)
		if !strings.HasSuffix(n, ".json") {
			continue
		}
		if b, err := os.ReadFile(filepath.Join(a.alertDir(), n)); err == nil {
			if f, err := zw.Create("alertas-detalhe/" + n); err == nil {
				_, _ = f.Write(b)
			}
		}
	}
	if req.Rc {
		if err := a.rcAddToDelta(zw, req.RcDays, req.RcFiles, !req.noSmall); err != nil {
			return nil, err
		}
		if err := a.attachAddToDelta(zw, req.Anexos); err != nil {
			return nil, err
		}
	}
	if err := zw.Close(); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

// mergeDelta junta um pacote recebido (mesma mescla do Importar backup).
func (a *App) mergeDelta(zipBytes []byte, from string) (MergeResult, error) {
	dir := filepath.Join(a.dataDir, "tmp")
	_ = os.MkdirAll(dir, 0o755)
	f, err := os.CreateTemp(dir, "sincronizacao-*.zip")
	if err != nil {
		return MergeResult{}, err
	}
	name := f.Name()
	defer os.Remove(name)
	if _, err := f.Write(zipBytes); err != nil {
		f.Close()
		return MergeResult{}, err
	}
	f.Close()
	res, err := a.mergeZip(name, false)
	res.From = from
	return res, err
}

// ---------- cliente (quem conecta) ----------
