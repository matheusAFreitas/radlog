package app

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"

	"radlog/internal/radiacode"
)

// ---------- dados brutos do Radiacode (até a 2.70) ----------
//
// Até a 2.70.2 o app guardava tudo o que o aparelho mandava, como veio, em
// radiacode/bruto (AAAA-MM-DD-HHMMSS.jsonl.gz e conexao-*.json), para
// descobrir o protocolo. O protocolo já é conhecido e a gravação saiu na
// auditoria de 12/11 (P2). Os arquivos antigos ficam onde estão e ainda são
// lidos pelas reposições que rodam uma vez (rclog, rcdoselog, rcmemory) e
// aceitos de backups e PCs antigos; nada novo é gravado ali.

const rcRawDir = "bruto"

func (a *App) rcRawPath(name string) string { return filepath.Join(a.rcDir(), rcRawDir, name) }

// rcBackupFiles são os arquivos do Radiacode que vão no backup.
func (a *App) rcBackupFiles() []string {
	var out []string
	mins, _ := filepath.Glob(filepath.Join(a.rcDir(), "*.csv"))
	for _, m := range mins {
		out = append(out, rcDirName+"/"+filepath.Base(m))
	}
	if _, err := os.Stat(filepath.Join(a.rcDir(), rcDoseFile)); err == nil {
		out = append(out, rcDirName+"/"+rcDoseFile)
	}
	a.rc.spec.mu.Lock()
	if a.rc.spec.gz != nil {
		a.rc.spec.gz.Flush()
	}
	a.rc.spec.mu.Unlock()
	out = append(out, a.actBackupFiles()...)
	specs, _ := a.rcSpecFiles()
	for _, n := range specs {
		out = append(out, rcDirName+"/"+rcSpecDir+"/"+n)
	}
	return out
}

// rcMergeFile junta um arquivo do Radiacode vindo de um backup: os minutos
// (radiacode/AAAA-MM.csv) entram pela hora, sem repetir; os brutos só se o
// arquivo não existir aqui. Devolve quantos minutos entraram.
func (a *App) rcMergeFile(rel string, data []byte) int {
	name := filepath.Base(rel)
	if strings.Contains(name, "..") || name == "" {
		return 0
	}
	if strings.HasPrefix(rel, rcDirName+"/"+actDir+"/") {
		a.actMerge(name, data)
		return 0
	}
	if strings.HasPrefix(rel, rcDirName+"/"+rcSpecDir+"/") {
		if !rcSpecName.MatchString(name) {
			return 0
		}
		dst := a.rcSpecPath(name)
		if _, err := os.Stat(dst); err == nil {
			return 0
		}
		os.MkdirAll(a.rcSpecPath(""), 0o755)
		os.WriteFile(dst, data, 0o644)
		return 0
	}
	if strings.HasPrefix(rel, rcDirName+"/"+rcRawDir+"/") {
		dst := a.rcRawPath(name)
		if _, err := os.Stat(dst); err == nil {
			return 0
		}
		os.MkdirAll(a.rcRawPath(""), 0o755)
		os.WriteFile(dst, data, 0o644)
		return 0
	}
	if rel == rcDirName+"/"+rcDoseFile {
		// dose do aparelho: fica a mais recente
		var in, have rcDoseSaved
		if json.Unmarshal(data, &in) != nil || in.At == 0 {
			return 0
		}
		a.rc.mu.Lock()
		defer a.rc.mu.Unlock()
		if b, err := os.ReadFile(filepath.Join(a.rcDir(), rcDoseFile)); err == nil {
			_ = json.Unmarshal(b, &have)
		}
		if in.At > have.At {
			_ = os.MkdirAll(a.rcDir(), 0o755)
			_ = os.WriteFile(filepath.Join(a.rcDir(), rcDoseFile), data, 0o644)
			if in.At > a.rc.dose.saved.At {
				a.rc.dose.saved, a.rc.dose.loaded = in, true
			}
		}
		return 0
	}
	if !strings.HasSuffix(name, ".csv") || strings.Contains(strings.TrimPrefix(rel, rcDirName+"/"), "/") {
		return 0
	}
	if strings.HasPrefix(name, "log-") { // log do aparelho: junta linha a linha
		var lines []string
		sc := bufio.NewScanner(bytes.NewReader(data))
		for sc.Scan() {
			lines = append(lines, sc.Text())
		}
		a.rc.fileM.Lock()
		defer a.rc.fileM.Unlock()
		a.rcLogMergeLocked(name, lines)
		return 0
	}
	a.rc.fileM.Lock()
	defer a.rc.fileM.Unlock()
	dst := filepath.Join(a.rcDir(), name)
	have := map[string]string{}
	if b, err := os.ReadFile(dst); err == nil {
		sc := bufio.NewScanner(bytes.NewReader(b))
		for sc.Scan() {
			if k, _, ok := strings.Cut(sc.Text(), ","); ok && k != "hora" {
				have[k] = sc.Text()
			}
		}
	}
	added := 0
	sc := bufio.NewScanner(bytes.NewReader(data))
	for sc.Scan() {
		k, _, ok := strings.Cut(sc.Text(), ",")
		if !ok || k == "hora" {
			continue
		}
		if _, err := time.Parse(time.RFC3339, k); err != nil {
			continue
		}
		if _, dup := have[k]; !dup {
			have[k] = sc.Text()
			added++
		}
	}
	if added == 0 {
		return 0
	}
	keys := make([]string, 0, len(have))
	for k := range have {
		keys = append(keys, k)
	}
	sort.Slice(keys, func(i, j int) bool {
		ti, _ := time.Parse(time.RFC3339, keys[i])
		tj, _ := time.Parse(time.RFC3339, keys[j])
		return ti.Before(tj)
	})
	var out bytes.Buffer
	hdr := rcHeader
	if strings.HasPrefix(name, "dose-") {
		hdr = rcDoseHeader
	}
	out.WriteString(hdr + "\n")
	for _, k := range keys {
		out.WriteString(have[k] + "\n")
	}
	os.MkdirAll(a.rcDir(), 0o755)
	tmp := dst + ".tmp"
	if os.WriteFile(tmp, out.Bytes(), 0o644) == nil {
		os.Rename(tmp, dst)
	}
	a.rc.mu.Lock()
	a.rc.today.loaded = false // recalcula a dose de hoje
	a.rc.mu.Unlock()
	a.rc.statsM.Lock()
	a.rc.stats = nil
	a.rc.statsM.Unlock()
	return added
}

// rcTextLog passa as mensagens internas do aparelho para o log.txt.
func (a *App) rcTextLog(dev *radiacode.Device) {
	lines, err := dev.TextMessages()
	if err != nil {
		return
	}
	for _, msg := range a.rc.texts.take(lines, time.Now()) {
		a.logf("%s", msg)
	}
}

// rcTextRepeat: a mesma mensagem do aparelho entra no log no máximo uma vez
// nesse intervalo; as repetições viram uma contagem na próxima vez que ela
// entra. O Radiacode 102 (firmware 4.15) solta "Error: S1/S2 Data frame
// corrupted!" a cada ~100 s: eram ~5 mil linhas por dia, o log.txt passava
// de 2 MB em horas e o log-anterior.txt perdia o resto.
const rcTextRepeat = time.Hour

// rcTexts lembra as mensagens já vistas entre conexões (ao reconectar, o
// aparelho devolve as antigas de novo) e junta as repetidas.
type rcTexts struct {
	mu      sync.Mutex
	seen    map[string][]time.Time // texto → quando cada uma aconteceu
	last    map[string]time.Time   // texto → quando entrou no log pela última vez
	skipped map[string]int         // texto → repetições desde então
}

func (t *rcTexts) take(lines []radiacode.TextLine, now time.Time) []string {
	t.mu.Lock()
	defer t.mu.Unlock()
	if t.seen == nil {
		t.seen, t.last, t.skipped = map[string][]time.Time{}, map[string]time.Time{}, map[string]int{}
	}
	for k, ws := range t.seen { // não cresce sem fim
		keep := ws[:0]
		for _, w := range ws {
			if now.Sub(w) <= 48*time.Hour {
				keep = append(keep, w)
			}
		}
		if len(keep) == 0 {
			delete(t.seen, k)
		} else {
			t.seen[k] = keep
		}
	}
	var out []string
	for _, l := range lines {
		when := now.Add(-time.Duration(l.AgoSec) * time.Second)
		// a mesma mensagem lida de novo vem com outro "segundos atrás"
		// (arredondamento e a hora da leitura): até 5 s de diferença é ela
		dup := false
		for _, w := range t.seen[l.Text] {
			if d := when.Sub(w); d > -5*time.Second && d < 5*time.Second {
				dup = true
				break
			}
		}
		if dup {
			continue
		}
		t.seen[l.Text] = append(t.seen[l.Text], when)
		if prev, ok := t.last[l.Text]; ok && when.Sub(prev) < rcTextRepeat {
			t.skipped[l.Text]++
			continue
		}
		msg := fmt.Sprintf("RC102 (mensagem do aparelho, %s): %s", when.Format("15:04:05"), l.Text)
		if n := t.skipped[l.Text]; n > 0 {
			msg += fmt.Sprintf(" (repetiu mais %d vezes desde %s)", n, t.last[l.Text].Format("15:04"))
		}
		t.last[l.Text], t.skipped[l.Text] = when, 0
		out = append(out, msg)
	}
	return out
}
