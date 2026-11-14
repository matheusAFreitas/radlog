package app

import (
	"archive/zip"
	"bytes"
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

	"radlog/internal/photometa"
)

// ---------- sincronização entre PCs: Radiacode, históricos e anexos (2.7.1) ----------
//
// Até a 2.7.0 a sincronização só levava os minutos do GMC, os alertas, os
// períodos, as medições e as anotações. Agora leva também:
//   - os arquivos do Radiacode por linha (minutos, dose e log do aparelho),
//     comparando um resumo por dia de cada arquivo, como nos minutos do GMC;
//   - os arquivos que só são copiados (espectros por minuto, dados brutos),
//     pelo nome, menos o que ainda está sendo gravado;
//   - a atividade (Bq), a dose do aparelho, o histórico de configurações e as
//     versões da calibração, sempre (são pequenos e a mescla não repete);
//   - os anexos das medições, pela medição (início) e pelo nome.

// rcInv é o inventário do Radiacode de um PC.
type rcInv struct {
	Days  map[string]string `json:"days"`  // "AAAA-MM.csv|2026-11-07" → "n:hash"
	Files []string          `json:"files"` // "espectros/…", "bruto/…" (já fechados)
}

type rcInvFile struct {
	mod  time.Time
	size int64
	days map[string]string
}

var rcInvCache = struct {
	sync.Mutex
	m map[string]rcInvFile
}{m: map[string]rcInvFile{}}

// rcLineFiles são os CSV do Radiacode que se juntam linha a linha.
func (a *App) rcLineFiles() []string {
	ents, _ := os.ReadDir(a.rcDir())
	var out []string
	for _, e := range ents {
		n := e.Name()
		if e.IsDir() || !strings.HasSuffix(n, ".csv") {
			continue
		}
		if rcMonthFile.MatchString(n) || strings.HasPrefix(n, "dose-") || strings.HasPrefix(n, "log-") {
			out = append(out, n)
		}
	}
	sort.Strings(out)
	return out
}

// rcLineKey é a chave que a mescla usa para não repetir a linha (e o dia).
func rcLineKey(file, line string) (key, day string, ok bool) {
	if len(line) < 10 || strings.HasPrefix(line, "hora") {
		return "", "", false
	}
	if strings.HasPrefix(file, "log-") {
		it, ok := parseRcLogLine(line)
		if !ok {
			return "", "", false
		}
		it.Src = ""
		return it.csvLine(), line[:10], true
	}
	k, _, ok := strings.Cut(line, ",")
	if !ok {
		return "", "", false
	}
	if _, err := time.Parse(time.RFC3339, k); err != nil {
		return "", "", false
	}
	return k, line[:10], true
}

func (a *App) readRcLines(name string) []string {
	a.rc.fileM.Lock()
	b, err := os.ReadFile(filepath.Join(a.rcDir(), name))
	a.rc.fileM.Unlock()
	if err != nil {
		return nil
	}
	var out []string
	sc := rcScanner(bytes.NewReader(b))
	sc.Buffer(make([]byte, 1<<16), 1<<20)
	for sc.Scan() {
		out = append(out, sc.Text())
	}
	return out
}

// rcInventory resume o que este PC tem do Radiacode.
func (a *App) rcInventory() rcInv {
	inv := rcInv{Days: map[string]string{}, Files: []string{}}
	if a.dataDir == "" {
		return inv
	}
	for _, n := range a.rcLineFiles() {
		fi, err := os.Stat(filepath.Join(a.rcDir(), n))
		if err != nil {
			continue
		}
		rcInvCache.Lock()
		c, ok := rcInvCache.m[filepath.Join(a.rcDir(), n)]
		rcInvCache.Unlock()
		if !ok || !c.mod.Equal(fi.ModTime()) || c.size != fi.Size() {
			keys := map[string][]string{}
			for _, l := range a.readRcLines(n) {
				if k, d, ok := rcLineKey(n, l); ok {
					keys[d] = append(keys[d], k)
				}
			}
			c = rcInvFile{mod: fi.ModTime(), size: fi.Size(), days: map[string]string{}}
			for d, ks := range keys {
				sort.Strings(ks)
				h := fnv.New64a()
				cnt := 0
				for i, k := range ks {
					if i > 0 && k == ks[i-1] {
						continue
					}
					cnt++
					h.Write([]byte(k + "\n"))
				}
				c.days[d] = fmt.Sprintf("%d:%x", cnt, h.Sum64())
			}
			rcInvCache.Lock()
			rcInvCache.m[filepath.Join(a.rcDir(), n)] = c
			rcInvCache.Unlock()
		}
		for d, h := range c.days {
			inv.Days[n+"|"+d] = h
		}
	}
	// arquivos só copiados: os que não estão sendo gravados agora
	active := map[string]bool{}
	a.rc.spec.mu.Lock()
	if a.rc.spec.f != nil {
		active[filepath.Base(a.rc.spec.f.Name())] = true
	}
	a.rc.spec.mu.Unlock()
	specs, _ := a.rcSpecFiles()
	for _, n := range specs {
		if !active[n] {
			inv.Files = append(inv.Files, rcSpecDir+"/"+n)
		}
	}
	sort.Strings(inv.Files)
	return inv
}

// rcWant compara os inventários: o que falta aqui (para pedir) ou lá (para
// mandar), conforme a ordem.
func rcWant(theirs, mine rcInv) (days, files []string) {
	for k, h := range theirs.Days {
		if mine.Days[k] != h {
			days = append(days, k)
		}
	}
	have := map[string]bool{}
	for _, f := range mine.Files {
		have[f] = true
	}
	for _, f := range theirs.Files {
		if !have[f] {
			files = append(files, f)
		}
	}
	sort.Strings(days)
	sort.Strings(files)
	return
}

// rcAddToDelta põe no pacote as linhas dos dias pedidos, os arquivos pedidos
// e os pequenos de sempre.
func (a *App) rcAddToDelta(zw *zip.Writer, days, files []string, small bool) error {
	byFile := map[string]map[string]bool{}
	for _, k := range days {
		f, d, ok := strings.Cut(k, "|")
		if !ok || filepath.Base(f) != f {
			continue
		}
		if byFile[f] == nil {
			byFile[f] = map[string]bool{}
		}
		byFile[f][d] = true
	}
	for f, ds := range byFile {
		var out bytes.Buffer
		n := 0
		for _, l := range a.readRcLines(f) {
			if strings.HasPrefix(l, "hora") {
				out.WriteString(l + "\n")
				continue
			}
			if _, d, ok := rcLineKey(f, l); ok && ds[d] {
				out.WriteString(l + "\n")
				n++
			}
		}
		if n == 0 {
			continue
		}
		w, err := zw.Create(rcDirName + "/" + f)
		if err != nil {
			return err
		}
		if _, err := w.Write(out.Bytes()); err != nil {
			return err
		}
	}
	add := func(rel string) error {
		b, err := os.ReadFile(filepath.Join(a.dataDir, filepath.FromSlash(rel)))
		if err != nil {
			return nil
		}
		w, err := zw.Create(rel)
		if err != nil {
			return err
		}
		_, err = w.Write(b)
		return err
	}
	for _, f := range files {
		dir, name := filepath.Split(f)
		dir = strings.TrimSuffix(dir, "/")
		if (dir != rcSpecDir && dir != rcRawDir) || name == "" || filepath.Base(name) != name {
			continue
		}
		if err := add(rcDirName + "/" + dir + "/" + name); err != nil {
			return err
		}
	}
	if !small {
		return nil
	}
	// pequenos, sempre: atividade, dose do aparelho, históricos
	smalls := a.actBackupFiles()
	smalls = append(smalls, rcDirName+"/"+rcDoseFile, cfgHistFile, calHistFile)
	for _, rel := range smalls {
		if err := add(rel); err != nil {
			return err
		}
	}
	return nil
}

// ---------- anexos das medições ----------

// attachInventory lista os anexos como "início da medição (s)/arquivo": o
// número da medição muda de um PC para o outro, o início não.
func (a *App) attachInventory() []string {
	a.mu.Lock()
	defer a.mu.Unlock()
	out := []string{}
	for _, m := range a.measurements {
		for _, at := range m.Attachments {
			if photometa.Risky(filepath.Ext(at.File)) {
				continue // fica só neste PC (S1)
			}
			out = append(out, fmt.Sprintf("%d/%s", m.Time/1000, at.File))
		}
	}
	sort.Strings(out)
	return out
}

func attachWant(theirs, mine []string) []string {
	have := map[string]bool{}
	for _, k := range mine {
		have[k] = true
	}
	var out []string
	for _, k := range theirs {
		if !have[k] {
			out = append(out, k)
		}
	}
	return out
}

// attachAddToDelta põe no pacote os anexos pedidos (no caminho com o número
// da medição deste PC; quem recebe acha a dele pelo início).
func (a *App) attachAddToDelta(zw *zip.Writer, keys []string) error {
	if len(keys) == 0 {
		return nil
	}
	want := map[string]bool{}
	for _, k := range keys {
		want[k] = true
	}
	a.mu.Lock()
	var rels []string
	starts := map[string]int64{} // número da medição aqui → início (s)
	for _, m := range a.measurements {
		for _, at := range m.Attachments {
			if want[fmt.Sprintf("%d/%s", m.Time/1000, at.File)] {
				rels = append(rels, fmt.Sprintf("%s/%d/%s", attachDirName, m.ID, at.File))
				starts[strconv.FormatInt(m.ID, 10)] = m.Time / 1000
			}
		}
	}
	a.mu.Unlock()
	// o início de cada medição vai junto: o pacote da nuvem não leva o
	// medicoes.json (auditoria 2.79.0, N1)
	if len(starts) > 0 {
		b, _ := json.Marshal(starts)
		w, err := zw.Create(attachStartsFile)
		if err != nil {
			return err
		}
		if _, err := w.Write(b); err != nil {
			return err
		}
	}
	for _, rel := range rels {
		b, err := os.ReadFile(filepath.Join(a.dataDir, filepath.FromSlash(rel)))
		if err != nil {
			continue
		}
		var ok bool
		if b, ok = a.attachForExport(rel, b); !ok {
			continue
		}
		w, err := zw.Create(rel)
		if err != nil {
			return err
		}
		if _, err := w.Write(b); err != nil {
			return err
		}
	}
	return nil
}
