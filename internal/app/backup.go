package app

import (
	"archive/zip"
	"bytes"
	"encoding/csv"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"time"

	"radlog/internal/store"

	wruntime "github.com/wailsapp/wails/v2/pkg/runtime"
)

// ---------- backup ----------
//
// Um backup é um .zip com as leituras (CSV por mês), eventos e alertas,
// fora de casa, medições, segundos dos alertas e o cache de chuva. Importar
// MESCLA: só entra o que falta (o que já existe aqui não muda).

// MergeResult diz o que entrou ao importar um backup.
type MergeResult struct {
	From         string `json:"from"` // PC de origem
	Created      string `json:"created"`
	Minutes      int    `json:"minutes"`
	Events       int    `json:"events"`
	Periods      int    `json:"periods"`
	Measurements int    `json:"measurements"`
	AlertFiles   int    `json:"alertFiles"`
	Notes        int    `json:"notes"`
	RcMinutes    int    `json:"rcMinutes"` // minutos do RadiaCode
}

func (m MergeResult) total() int {
	return m.Minutes + m.Events + m.Periods + m.Measurements + m.AlertFiles + m.Notes + m.RcMinutes
}

type backupManifest struct {
	App     string `json:"app"`
	Version string `json:"version"`
	Host    string `json:"host"`
	Created string `json:"created"`
}

func hostName() string {
	h, _ := os.Hostname()
	if h == "" {
		h = "pc"
	}
	return h
}

var badFileChars = regexp.MustCompile(`[^A-Za-z0-9_.-]+`)

// ---------- gerar ----------

// writeBackup escreve o .zip em w.
func (a *App) writeBackup(w io.Writer) error {
	zw := zip.NewWriter(w)
	man, _ := json.MarshalIndent(backupManifest{App: "gmc-monitor", Version: AppVersion, Host: hostName(), Created: time.Now().Format(time.RFC3339)}, "", "  ")
	if f, err := zw.Create("backup.json"); err == nil {
		_, _ = f.Write(man)
	}
	add := func(rel string) error {
		src := filepath.Join(a.dataDir, filepath.FromSlash(rel))
		b, err := os.ReadFile(src)
		if err != nil {
			if errors.Is(err, os.ErrNotExist) {
				return nil
			}
			return err
		}
		f, err := zw.Create(rel)
		if err != nil {
			return err
		}
		_, err = f.Write(b)
		return err
	}
	a.mu.Lock()
	files := []string{"eventos.csv", "fora-de-casa.json", "medicoes.json", "anotacoes.json", "chuva.json", "chuva-radar.json"}
	a.mu.Unlock()
	months, _ := filepath.Glob(filepath.Join(a.dataDir, "leituras", "*.csv"))
	for _, m := range months {
		files = append(files, "leituras/"+filepath.Base(m))
	}
	alerts, _ := filepath.Glob(filepath.Join(a.alertDir(), "*.json"))
	for _, f := range alerts {
		files = append(files, "alertas-detalhe/"+filepath.Base(f))
	}
	files = append(files, a.rcBackupFiles()...)
	for _, f := range files {
		if err := add(f); err != nil {
			zw.Close()
			return err
		}
	}
	return zw.Close()
}

func (a *App) writeBackupFile(dst string) error {
	tmp := dst + ".tmp"
	f, err := os.Create(tmp)
	if err != nil {
		return err
	}
	if err := a.writeBackup(f); err != nil {
		f.Close()
		os.Remove(tmp)
		return err
	}
	if err := f.Close(); err != nil {
		os.Remove(tmp)
		return err
	}
	return os.Rename(tmp, dst)
}

// ExportBackup pergunta onde salvar e grava o backup. Devolve o caminho
// (vazio se cancelou).
func (a *App) ExportBackup() (string, error) {
	if a.ctx == nil {
		return "", errors.New("sem janela")
	}
	name := fmt.Sprintf("gmc-backup-%s-%s.zip", badFileChars.ReplaceAllString(hostName(), "_"), time.Now().Format("2006-01-02"))
	dst, err := wruntime.SaveFileDialog(a.ctx, wruntime.SaveDialogOptions{
		Title: "Salvar backup", DefaultFilename: name,
		Filters: []wruntime.FileFilter{{DisplayName: "Backup do GMC Monitor (*.zip)", Pattern: "*.zip"}},
	})
	if err != nil || dst == "" {
		return "", err
	}
	if !strings.HasSuffix(strings.ToLower(dst), ".zip") {
		dst += ".zip"
	}
	if err := a.writeBackupFile(dst); err != nil {
		a.logf("backup: erro ao salvar %s: %v", dst, err)
		return "", err
	}
	a.logf("backup: salvo em %s", dst)
	a.logEvent("Backup salvo: " + filepath.Base(dst))
	return dst, nil
}

// ImportBackup pergunta qual .zip e mescla.
func (a *App) ImportBackup() (MergeResult, error) {
	if a.ctx == nil {
		return MergeResult{}, errors.New("sem janela")
	}
	src, err := wruntime.OpenFileDialog(a.ctx, wruntime.OpenDialogOptions{
		Title:   "Importar backup",
		Filters: []wruntime.FileFilter{{DisplayName: "Backup do GMC Monitor (*.zip)", Pattern: "*.zip"}},
	})
	if err != nil || src == "" {
		return MergeResult{}, err
	}
	res, err := a.mergeBackup(src)
	if err != nil {
		return res, err
	}
	a.logEvent(fmt.Sprintf("Backup importado (%s): %s", res.From, mergeText(res)))
	return res, nil
}

func mergeText(r MergeResult) string {
	if r.total() == 0 {
		return "nada novo"
	}
	var p []string
	add := func(n int, one, many string) {
		if n == 1 {
			p = append(p, "1 "+one)
		} else if n > 1 {
			p = append(p, fmt.Sprintf("%d %s", n, many))
		}
	}
	add(r.Minutes, "minuto de leitura", "minutos de leitura")
	add(r.Events, "evento/alerta", "eventos/alertas")
	add(r.Periods, "período fora de casa", "períodos fora de casa")
	add(r.Measurements, "medição", "medições")
	add(r.AlertFiles, "detalhe de alerta", "detalhes de alerta")
	add(r.Notes, "anotação", "anotações")
	return "+" + strings.Join(p, ", ")
}

// ---------- mesclar ----------

func readZipFile(f *zip.File) ([]byte, error) {
	rc, err := f.Open()
	if err != nil {
		return nil, err
	}
	defer rc.Close()
	return io.ReadAll(io.LimitReader(rc, 64<<20))
}

func eventKey(e Event) string {
	return fmt.Sprintf("%d|%s|%s|%s", e.Time/1000, e.Kind, e.Level, e.Text)
}

// mergeBackup junta o conteúdo do .zip aos dados daqui (só o que falta).
func (a *App) mergeBackup(src string) (MergeResult, error) { return a.mergeZip(src, true) }

// mergeZip é a mescla do backup, usada também pela sincronização entre PCs
// (backup=false: só registra no log quando entrou alguma coisa).
func (a *App) mergeZip(src string, backup bool) (MergeResult, error) {
	var res MergeResult
	zr, err := zip.OpenReader(src)
	if err != nil {
		return res, fmt.Errorf("não é um backup válido: %w", err)
	}
	defer zr.Close()
	files := map[string]*zip.File{}
	for _, f := range zr.File {
		files[path.Clean(f.Name)] = f
	}
	mf, ok := files["backup.json"]
	if !ok {
		return res, errors.New("não é um backup do GMC Monitor (falta backup.json)")
	}
	if b, err := readZipFile(mf); err == nil {
		var m backupManifest
		if json.Unmarshal(b, &m) == nil {
			if m.App != "gmc-monitor" {
				return res, errors.New("não é um backup do GMC Monitor")
			}
			res.From, res.Created = m.Host, m.Created
		}
	}

	// leituras
	var names []string
	for n := range files {
		if strings.HasPrefix(n, "leituras/") && strings.HasSuffix(n, ".csv") {
			names = append(names, n)
		}
	}
	sort.Strings(names)
	for _, n := range names {
		b, err := readZipFile(files[n])
		if err != nil {
			return res, err
		}
		pts := store.ParseCSV(bytes.NewReader(b))
		if len(pts) == 0 || a.store == nil {
			continue
		}
		added, err := a.store.ImportNew(pts)
		if err != nil {
			return res, err
		}
		res.Minutes += len(added)
	}

	// eventos e alertas
	if f, ok := files["eventos.csv"]; ok {
		if b, err := readZipFile(f); err == nil {
			r := csv.NewReader(bytes.NewReader(b))
			r.FieldsPerRecord = -1
			var incoming []Event
			for {
				rec, err := r.Read()
				if err == io.EOF {
					break
				}
				if err != nil {
					continue
				}
				if ev, ok := parseEventRow(rec); ok {
					incoming = append(incoming, ev)
				}
			}
			a.mu.Lock()
			have := map[string]bool{}
			for _, e := range a.events {
				have[eventKey(e)] = true
			}
			for _, e := range incoming {
				if have[eventKey(e)] {
					continue
				}
				have[eventKey(e)] = true
				a.evSeq++
				e.ID = a.evSeq
				a.events = append(a.events, e)
				res.Events++
			}
			if res.Events > 0 {
				sort.SliceStable(a.events, func(i, j int) bool { return a.events[i].Time < a.events[j].Time })
				if len(a.events) > maxEvents {
					a.events = a.events[len(a.events)-maxEvents:]
				}
			}
			a.mu.Unlock()
			if res.Events > 0 {
				a.saveEvents()
			}
		}
	}

	// fora de casa
	if f, ok := files["fora-de-casa.json"]; ok {
		if b, err := readZipFile(f); err == nil {
			var list []Period
			if json.Unmarshal(b, &list) == nil {
				a.mu.Lock()
				var max int64
				have := map[string]bool{}
				for _, p := range a.periods {
					have[fmt.Sprintf("%d-%d", p.From, p.To)] = true
					max = maxInt64(max, p.ID)
				}
				for _, p := range list {
					if have[fmt.Sprintf("%d-%d", p.From, p.To)] || p.To <= p.From {
						continue
					}
					max++
					p.ID = max
					a.periods = append(a.periods, p)
					res.Periods++
				}
				if res.Periods > 0 {
					sort.Slice(a.periods, func(i, j int) bool { return a.periods[i].From < a.periods[j].From })
					_ = a.savePeriodsLocked()
				}
				a.mu.Unlock()
			}
		}
	}

	// medições
	if f, ok := files["medicoes.json"]; ok {
		if b, err := readZipFile(f); err == nil {
			var list []SavedMeasure
			if json.Unmarshal(b, &list) == nil {
				a.mu.Lock()
				var max int64
				have := map[int64]bool{}
				for _, m := range a.measurements {
					have[m.Time/1000] = true
					max = maxInt64(max, m.ID)
				}
				for _, m := range list {
					if have[m.Time/1000] {
						continue
					}
					max++
					m.ID = max
					a.measurements = append(a.measurements, m)
					res.Measurements++
				}
				if res.Measurements > 0 {
					sort.Slice(a.measurements, func(i, j int) bool { return a.measurements[i].Time < a.measurements[j].Time })
					_ = a.saveMeasurementsLocked()
				}
				a.mu.Unlock()
			}
		}
	}

	// anotações
	if f, ok := files["anotacoes.json"]; ok {
		if b, err := readZipFile(f); err == nil {
			var list []Note
			if json.Unmarshal(b, &list) == nil {
				res.Notes = a.mergeNotes(list)
			}
		}
	}

	// segundos dos alertas
	for n, f := range files {
		if !strings.HasPrefix(n, "alertas-detalhe/") || !strings.HasSuffix(n, ".json") {
			continue
		}
		dst := filepath.Join(a.alertDir(), filepath.Base(n))
		if _, err := os.Stat(dst); err == nil {
			continue
		}
		b, err := readZipFile(f)
		if err != nil {
			continue
		}
		_ = os.MkdirAll(a.alertDir(), 0o755)
		if os.WriteFile(dst, b, 0o644) == nil {
			res.AlertFiles++
		}
	}

	// RadiaCode: minutos (junta pela hora) e dados brutos (copia o que falta)
	for n, f := range files {
		b, err := func() ([]byte, error) {
			if !strings.HasPrefix(n, rcDirName+"/") {
				return nil, nil
			}
			return readZipFile(f)
		}()
		if err != nil || b == nil {
			continue
		}
		res.RcMinutes += a.rcMergeFile(n, b)
	}

	// chuva (só se for o mesmo lugar)
	if f, ok := files["chuva.json"]; ok {
		if b, err := readZipFile(f); err == nil {
			var c rainCache
			if json.Unmarshal(b, &c) == nil && c.V == rainCacheVersion {
				a.mu.Lock()
				if a.rain != nil && a.rain.Lat == c.Lat && a.rain.Lon == c.Lon {
					for k, v := range c.Hours {
						if _, ok := a.rain.Hours[k]; !ok {
							a.rain.Hours[k] = v
						}
					}
				}
				a.mu.Unlock()
			}
		}
	}
	if f, ok := files["chuva-radar.json"]; ok {
		if b, err := readZipFile(f); err == nil {
			var r radarStore
			if json.Unmarshal(b, &r) == nil && r.V == radarVersion {
				a.mu.Lock()
				if a.radar != nil && a.radar.Lat == r.Lat && a.radar.Lon == r.Lon {
					for k, v := range r.Points {
						if _, ok := a.radar.Points[k]; !ok {
							a.radar.Points[k] = v
						}
					}
					a.saveRadarLocked()
				}
				a.mu.Unlock()
			}
		}
	}

	if backup {
		a.logf("backup: mesclado %s (%s): %s", filepath.Base(src), res.From, mergeText(res))
	}
	if res.total() > 0 {
		a.mu.Lock()
		a.bgAt = time.Time{} // recalcula o fundo
		a.mu.Unlock()
		a.emit("stored", time.Now().UnixMilli())
		a.emit("event", Event{Kind: kindAlert})
		a.emit("periods", a.GetPeriods())
		a.emit("measurements", a.GetMeasurements())
		a.emit("notes", a.GetNotes(0, 0))
	}
	return res, nil
}

func maxInt64(a, b int64) int64 {
	if a > b {
		return a
	}
	return b
}
