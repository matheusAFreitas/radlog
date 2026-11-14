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
	"strconv"
	"strings"
	"time"

	"radlog/internal/store"

	wruntime "github.com/wailsapp/wails/v2/pkg/runtime"
)

// ---------- backup ----------
//
// Um backup é um .zip com as leituras (CSV por mês), eventos e alertas,
// fora de casa, medições, segundos dos alertas, o cache de chuva e as
// configurações sem o lugar. Importar MESCLA: só entra o que falta (o que já
// existe aqui não muda); as configurações só mudam se a pessoa escolher.

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
	RcMinutes    int    `json:"rcMinutes"`         // minutos do RC102
	Deleted      int    `json:"deleted,omitempty"` // apagados em outro aparelho (N4)
	// ajustes das configurações do backup diferentes dos daqui (2.74); a
	// tela pergunta se quer usar (ApplyBackupSettings)
	Settings int `json:"settings"`
}

func (m MergeResult) total() int {
	return m.Minutes + m.Events + m.Periods + m.Measurements + m.AlertFiles + m.Notes + m.RcMinutes + m.Deleted
}

// backupAppID marca os backups; os da 1.x (GMC Monitor) continuam valendo.
const (
	backupAppID    = "radlog"
	oldBackupAppID = "gmc-monitor"
)

type backupManifest struct {
	App     string `json:"app"`
	Version string `json:"version"`
	Host    string `json:"host,omitempty"` // até a 2.77.0; não vai mais (D4)
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
	man, _ := json.MarshalIndent(backupManifest{App: backupAppID, Version: AppVersion, Created: time.Now().Format(time.RFC3339)}, "", "  ")
	if f, err := zw.Create("backup.json"); err == nil {
		_, _ = f.Write(man)
	}
	// as configurações, sem o lugar e sem o que é só deste PC (backupcfg.go)
	if f, err := zw.Create(backupSettingsFile); err == nil {
		_, _ = f.Write(a.backupSettingsJSON())
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
		if rel == "chuva.json" || rel == "chuva-radar.json" {
			b = hidePlace(b) // o lugar não sai do PC
		}
		if strings.HasPrefix(rel, attachDirName+"/") {
			var ok bool
			if b, ok = a.attachForExport(rel, b); !ok {
				return nil
			}
		}
		f, err := zw.Create(rel)
		if err != nil {
			return err
		}
		_, err = f.Write(b)
		return err
	}
	a.mu.Lock()
	files := []string{"alertas.csv", "eventos.csv", tombsFile, "fora-de-casa.json", "medicoes.json", "anotacoes.json", "chuva.json", "chuva-radar.json", rainDaysFile, cfgHistFile, calHistFile, intercalFile, plannedFile}
	a.mu.Unlock()
	months, _ := filepath.Glob(filepath.Join(a.dataDir, "leituras", "*.csv"))
	for _, m := range months {
		files = append(files, "leituras/"+filepath.Base(m))
	}
	alerts, _ := filepath.Glob(filepath.Join(a.alertDir(), "*.json"))
	for _, f := range alerts {
		files = append(files, "alertas-detalhe/"+filepath.Base(f))
	}
	anexos, _ := filepath.Glob(filepath.Join(a.dataDir, attachDirName, "*", "*"))
	for _, f := range anexos {
		rel, err := filepath.Rel(a.dataDir, f)
		if err == nil {
			files = append(files, filepath.ToSlash(rel))
		}
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
	if err := f.Sync(); err != nil {
		f.Close()
		os.Remove(tmp)
		return err
	}
	if err := f.Close(); err != nil {
		os.Remove(tmp)
		return err
	}
	if err := store.RenameRetry(tmp, dst); err != nil {
		os.Remove(tmp)
		return err
	}
	return nil
}

// ExportBackup pergunta onde salvar e grava o backup. Devolve o caminho
// (vazio se cancelou).
func (a *App) ExportBackup() (string, error) {
	if a.ctx == nil {
		return "", errors.New("sem janela")
	}
	// sem o nome do PC (D4): ele ia dentro do arquivo e no nome sugerido
	name := fmt.Sprintf("radlog-backup-%s.zip", time.Now().Format("2006-01-02"))
	dst, err := wruntime.SaveFileDialog(a.ctx, wruntime.SaveDialogOptions{
		Title: "Salvar backup", DefaultFilename: name,
		Filters: []wruntime.FileFilter{{DisplayName: "Backup do RadLog (*.zip)", Pattern: "*.zip"}},
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
		Filters: []wruntime.FileFilter{{DisplayName: "Backup do RadLog (*.zip)", Pattern: "*.zip"}},
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
	add(r.RcMinutes, "registro do RC102", "registros do RC102")
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
		return res, errors.New("não é um backup do RadLog (falta backup.json)")
	}
	if b, err := readZipFile(mf); err == nil {
		var m backupManifest
		if json.Unmarshal(b, &m) == nil {
			if m.App != backupAppID && m.App != oldBackupAppID {
				return res, errors.New("não é um backup do RadLog")
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
	for _, name := range []string{"alertas.csv", "eventos.csv"} {
		f, ok := files[name]
		if !ok {
			continue
		}
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
				a.events = trimEvents(a.events)
			}
			a.mu.Unlock()
			if res.Events > 0 {
				a.saveEvents()
			}
		}
	}

	// apagados em outro aparelho (N4): antes dos itens, para não voltarem
	if f, ok := files[tombsFile]; ok {
		if b, err := readZipFile(f); err == nil {
			var list []tomb
			if json.Unmarshal(b, &list) == nil {
				res.Deleted = a.applyTombs(list)
			}
		}
	}

	// fora de casa
	if f, ok := files["fora-de-casa.json"]; ok {
		if b, err := readZipFile(f); err == nil {
			var list []Period
			if json.Unmarshal(b, &list) == nil {
				if backup {
					for i := range list {
						if list[i].UID == "" {
							list[i].UID = legacyPeriodUID(list[i])
						}
						list[i].Edited = a.backupRevive(recPeriod, list[i].UID, list[i].Edited)
					}
				}
				res.Periods = a.mergePeriods(list)
			}
		}
	}

	// saídas programadas que faltam aqui
	if f, ok := files[plannedFile]; ok && backup {
		if b, err := readZipFile(f); err == nil {
			var list []PlannedTrip
			if json.Unmarshal(b, &list) == nil {
				a.mergePlanned(list)
			}
		}
	}

	// medições
	attachIDs := map[int64]int64{} // id da medição no backup → id aqui
	if f, ok := files["medicoes.json"]; ok {
		if b, err := readZipFile(f); err == nil {
			var list []SavedMeasure
			if json.Unmarshal(b, &list) == nil {
				if backup {
					for i := range list {
						if list[i].UID == "" {
							list[i].UID = legacyMeasureUID(list[i])
						}
						list[i].Edited = a.backupRevive(recMeasure, list[i].UID, list[i].Edited)
					}
				}
				a.mu.Lock()
				a.tombsLoadLocked()
				var max int64
				have := map[int64]int{} // segundo do início → índice
				byUID := map[string]int{}
				for i, m := range a.measurements {
					have[m.Time/1000] = i
					if m.UID != "" {
						byUID[m.UID] = i
					}
					max = maxInt64(max, m.ID)
				}
				changed := false
				for _, m := range list {
					m.Attachments = cleanAttachments(m.Attachments)
					if m.UID == "" {
						m.UID = legacyMeasureUID(m)
					}
					if a.tombBlocks(recMeasure, m.UID, m.Edited) {
						continue
					}
					i, same := byUID[m.UID]
					if same {
						local := &a.measurements[i]
						attachIDs[m.ID] = local.ID
						if m.Edited > local.Edited {
							// editada lá depois daqui (N4): vale a de lá
							m.ID = local.ID
							*local = m
							changed = true
							continue
						}
						if m.Edited != 0 || local.Edited != 0 {
							continue
						}
					} else {
						i, same = have[m.Time/1000]
					}
					if same {
						// a mesma medição de antes da 2.80: junta os anexos que faltam
						local := &a.measurements[i]
						attachIDs[m.ID] = local.ID
						for _, at := range m.Attachments {
							dup := false
							for _, x := range local.Attachments {
								dup = dup || x.File == at.File
							}
							if !dup {
								local.Attachments = append(local.Attachments, at)
								changed = true
							}
						}
						continue
					}
					max++
					attachIDs[m.ID] = max
					m.ID = max
					a.measurements = append(a.measurements, m)
					byUID[m.UID] = len(a.measurements) - 1
					res.Measurements++
				}
				if res.Measurements > 0 || changed {
					sort.Slice(a.measurements, func(i, j int) bool { return a.measurements[i].Time < a.measurements[j].Time })
					a.bgAt = time.Time{}
					a.recMerge = true
					_ = a.saveMeasurementsLocked()
					a.recMerge = false
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
				if backup {
					for i := range list {
						if list[i].UID == "" {
							list[i].UID = legacyNoteUID(list[i])
						}
						list[i].Edited = a.backupRevive(recNote, list[i].UID, list[i].Edited)
					}
				}
				res.Notes = a.mergeNotes(list)
			}
		}
	}

	// configurações: só ficam guardadas para a pessoa escolher (o pacote da
	// sincronização não traz)
	if backup {
		var b []byte
		if f, ok := files[backupSettingsFile]; ok {
			b, _ = readZipFile(f)
		}
		res.Settings = a.stashBackupSettings(b) // sem o arquivo: esquece o anterior
	}

	// histórico de configurações
	if f, ok := files[cfgHistFile]; ok {
		if b, err := readZipFile(f); err == nil {
			a.cfgMerge(b)
		}
	}

	// versões da calibração
	if f, ok := files[calHistFile]; ok {
		if b, err := readZipFile(f); err == nil {
			a.calMerge(b)
		}
	}

	// intercalibração com o Radiacode: só entra se aqui não houver nenhuma
	if f, ok := files[intercalFile]; ok && a.loadIntercal().Start == 0 {
		if b, err := readZipFile(f); err == nil {
			var d intercalData
			if json.Unmarshal(b, &d) == nil && d.Start > 0 {
				_ = a.saveIntercal(d)
			}
		}
	}

	// anexos das medições (copia o que falta). Sem o medicoes.json no mesmo
	// pacote (nuvem), a medição daqui é achada pelo início (N1).
	var starts map[string]int64
	if f, ok := files[attachStartsFile]; ok {
		if b, err := readZipFile(f); err == nil {
			_ = json.Unmarshal(b, &starts)
		}
	}
	localByStart := func(sec int64) (int64, bool) {
		a.mu.Lock()
		defer a.mu.Unlock()
		for _, m := range a.measurements {
			if m.Time/1000 == sec {
				return m.ID, true
			}
		}
		return 0, false
	}
	for n, f := range files {
		parts := strings.Split(n, "/")
		if len(parts) != 3 || parts[0] != attachDirName || parts[1] == "" || safeFileName(parts[2]) != parts[2] {
			continue
		}
		bid, err := strconv.ParseInt(parts[1], 10, 64)
		if err != nil {
			continue
		}
		lid, ok := attachIDs[bid]
		if !ok {
			sec, has := starts[parts[1]]
			if !has {
				continue
			}
			if lid, ok = localByStart(sec); !ok {
				continue
			}
		}
		dst := filepath.Join(a.attachDir(lid), parts[2])
		if _, err := os.Stat(dst); err == nil {
			continue
		}
		if b, err := readZipFile(f); err == nil {
			_ = os.MkdirAll(filepath.Dir(dst), 0o755)
			_ = writeFileAtomic(dst, b, 0o644)
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
		if writeFileAtomic(dst, b, 0o644) == nil {
			res.AlertFiles++
		}
	}

	// Radiacode: minutos (junta pela hora) e dados brutos (copia o que falta)
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
	rainSame := false
	if f, ok := files["chuva.json"]; ok {
		if b, err := readZipFile(f); err == nil {
			var c rainCache
			if json.Unmarshal(b, &c) == nil && c.V == rainCacheVersion {
				a.mu.Lock()
				if a.rain != nil {
					same, ok := samePlace(a.rain.Lat, a.rain.Lon, c.Lat, c.Lon, c.Place)
					if !ok {
						same = samePressure(a.rain.Press, c.Press)
					}
					rainSame = same
				}
				if rainSame {
					for k, v := range c.Hours {
						if _, ok := a.rain.Hours[k]; !ok {
							a.rain.Hours[k] = v
						}
					}
					if a.rain.Press == nil {
						a.rain.Press = map[int64]float64{}
					}
					for k, v := range c.Press {
						if _, ok := a.rain.Press[k]; !ok {
							a.rain.Press[k] = v
						}
					}
				}
				a.mu.Unlock()
			}
		}
	}
	if f, ok := files[rainDaysFile]; ok {
		if b, err := readZipFile(f); err == nil {
			a.mergeRainDaysBackup(b)
		}
	}
	if f, ok := files["chuva-radar.json"]; ok {
		if b, err := readZipFile(f); err == nil {
			var r radarStore
			if json.Unmarshal(b, &r) == nil && r.V == radarVersion {
				a.mu.Lock()
				same := rainSame // o radar do backup é do mesmo lugar que a chuva dele
				if a.radar != nil {
					if s, ok := samePlace(a.radar.Lat, a.radar.Lon, r.Lat, r.Lon, r.Place); ok {
						same = s
					}
				}
				if a.radar != nil && same {
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
