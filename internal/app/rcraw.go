package app

import (
	"bufio"
	"bytes"
	"compress/gzip"
	"encoding/hex"
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

// ---------- dados brutos do RadiaCode ----------
//
// Para descobrir o que mais o aparelho oferece, o app guarda tudo o que ele
// manda, como veio:
//   - radiacode/bruto/AAAA-MM-DD-HHMMSS.jsonl.gz: cada registro do DATA_BUF
//     (em hex, com os campos que já se sabe ler), e o espectro a cada hora.
//     Um arquivo por conexão (até a 1.39 era um por dia, com cada conexão
//     emendada no fim: se o app fosse fechado à força, o pedaço sem o fim do
//     gzip estragava a leitura de tudo o que vinha depois);
//   - radiacode/bruto/conexao-AAAAMMDD-HHMMSS.json: a cada conexão, todos os
//     registradores e "strings virtuais" que dá para ler (inclusive as de
//     formato desconhecido, como a cópia da memória).
// Entra no backup. Desliga em Configurações → RadiaCode.

const rcRawDir = "bruto"

type rcRawLog struct {
	mu        sync.Mutex
	day       string
	f         *os.File
	gz        *gzip.Writer
	enc       *json.Encoder
	lastFlush time.Time
	lastSpec  time.Time
}

func (a *App) rcRawOn() bool {
	a.mu.Lock()
	defer a.mu.Unlock()
	return !a.settings.RcNoRaw && a.dataDir != ""
}

func (a *App) rcRawPath(name string) string { return filepath.Join(a.rcDir(), rcRawDir, name) }

// rcRawWrite grava uma linha no arquivo do dia.
func (a *App) rcRawWrite(v map[string]any) {
	l := &a.rc.raw
	l.mu.Lock()
	defer l.mu.Unlock()
	now := time.Now()
	day := now.Format("2006-01-02")
	if l.day != day || l.gz == nil {
		l.closeLocked()
		os.MkdirAll(a.rcRawPath(""), 0o755)
		f, err := os.OpenFile(a.rcRawPath(day+"-"+now.Format("150405")+".jsonl.gz"), os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o644)
		if err != nil {
			return
		}
		l.f, l.gz, l.day = f, gzip.NewWriter(f), day
		l.enc = json.NewEncoder(l.gz)
		l.lastFlush = now
	}
	v["pc"] = now.UnixMilli()
	l.enc.Encode(v)
	if now.Sub(l.lastFlush) > time.Minute {
		l.gz.Flush()
		l.lastFlush = now
	}
}

func (l *rcRawLog) closeLocked() {
	if l.gz != nil {
		l.gz.Close()
	}
	if l.f != nil {
		l.f.Close()
	}
	l.f, l.gz, l.enc = nil, nil, nil
}

func (a *App) rcRawClose() {
	a.rc.raw.mu.Lock()
	a.rc.raw.closeLocked()
	a.rc.raw.mu.Unlock()
}

// rcRawRecords guarda cada registro de uma resposta do DATA_BUF.
func (a *App) rcRawRecords(b []byte, base time.Time) {
	if len(b) == 0 || !a.rcRawOn() {
		return
	}
	for _, r := range radiacode.SplitDataBuf(b, base) {
		v := map[string]any{"t": r.T.UnixMilli(), "seq": r.Seq, "eid": r.EID, "gid": r.GID, "tipo": r.Name, "hex": hex.EncodeToString(r.Data)}
		if len(r.Fields) > 0 {
			v["v"] = r.Fields
		}
		a.rcRawWrite(v)
	}
}

// rcRawSpectrum guarda os espectros (atual e acumulado) de hora em hora.
func (a *App) rcRawSpectrum(dev *radiacode.Device) {
	l := &a.rc.raw
	l.mu.Lock()
	due := time.Since(l.lastSpec) >= time.Hour
	if due {
		l.lastSpec = time.Now()
	}
	l.mu.Unlock()
	if !due || !a.rcRawOn() {
		return
	}
	for _, acc := range []bool{false, true} {
		var s radiacode.Spectrum
		var err error
		if acc {
			s, err = dev.SpectrumAccum()
		} else {
			s, err = dev.Spectrum()
		}
		v := map[string]any{"tipo": map[bool]string{false: "Espectro", true: "EspectroAcumulado"}[acc]}
		if err != nil {
			v["erro"] = err.Error()
		} else {
			v["s"], v["a"], v["contagens"] = s.Duration.Seconds(), []float64{s.A0, s.A1, s.A2}, s.Counts
		}
		a.rcRawWrite(v)
	}
}

// rcRawSnapshot lê tudo o que dá ao conectar e guarda num .json.
func (a *App) rcRawSnapshot(dev *radiacode.Device) {
	if !a.rcRawOn() {
		return
	}
	now := time.Now()
	snap := map[string]any{
		"quando": now.Format(time.RFC3339), "app": AppVersion, "via": dev.Kind(),
		"modelo": dev.Model, "serie": dev.Serial, "firmware": dev.Firmware,
	}
	if cfg, err := dev.Configuration(); err == nil {
		snap["configuracao"] = cfg
	}
	sfr := map[string]any{}
	for _, r := range radiacode.KnownSFR {
		k := fmt.Sprintf("%#x %s", r.ID, r.Name)
		if v, err := dev.ReadSFR(r.ID); err == nil {
			sfr[k] = v
		} else {
			sfr[k] = "erro: " + err.Error()
		}
	}
	snap["registradores"] = sfr
	vs := map[string]any{}
	for _, s := range radiacode.KnownVS {
		k := fmt.Sprintf("%#x %s", s.ID, s.Name)
		if b, err := dev.ReadVS(s.ID); err == nil {
			vs[k] = map[string]any{"bytes": len(b), "hex": hex.EncodeToString(b)}
		} else {
			e := map[string]any{"erro": err.Error()}
			if raw, err2 := dev.ReadVSRaw(s.ID); err2 == nil {
				e["resposta"] = hex.EncodeToString(raw)
			}
			vs[k] = e
		}
	}
	snap["strings"] = vs
	b, _ := json.MarshalIndent(snap, "", "  ")
	os.MkdirAll(a.rcRawPath(""), 0o755)
	os.WriteFile(a.rcRawPath("conexao-"+now.Format("20060102-150405")+".json"), b, 0o644)
}

// rcBackupFiles são os arquivos do RadiaCode que vão no backup.
func (a *App) rcBackupFiles() []string {
	var out []string
	mins, _ := filepath.Glob(filepath.Join(a.rcDir(), "*.csv"))
	for _, m := range mins {
		out = append(out, rcDirName+"/"+filepath.Base(m))
	}
	if a.rcRawOn() {
		a.rc.raw.mu.Lock()
		if a.rc.raw.gz != nil { // o arquivo do dia inteiro, até agora
			a.rc.raw.gz.Flush()
		}
		a.rc.raw.mu.Unlock()
	}
	if _, err := os.Stat(filepath.Join(a.rcDir(), rcDoseFile)); err == nil {
		out = append(out, rcDirName+"/"+rcDoseFile)
	}
	raws, _ := filepath.Glob(filepath.Join(a.rcDir(), rcRawDir, "*"))
	sort.Strings(raws)
	for _, r := range raws {
		out = append(out, rcDirName+"/"+rcRawDir+"/"+filepath.Base(r))
	}
	return out
}

// rcMergeFile junta um arquivo do RadiaCode vindo de um backup: os minutos
// (radiacode/AAAA-MM.csv) entram pela hora, sem repetir; os brutos só se o
// arquivo não existir aqui. Devolve quantos minutos entraram.
func (a *App) rcMergeFile(rel string, data []byte) int {
	name := filepath.Base(rel)
	if strings.Contains(name, "..") || name == "" {
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
	out.WriteString(rcHeader + "\n")
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

// SetRcRaw liga/desliga a gravação dos dados brutos do RadiaCode.
func (a *App) SetRcRaw(on bool) RcState {
	a.mu.Lock()
	a.settings.RcNoRaw = !on
	a.saveSettingsLocked()
	a.mu.Unlock()
	if !on {
		a.rcRawClose()
	}
	a.emitRc()
	return a.GetRadiacode()
}

// rcTextLog passa as mensagens internas do aparelho para o log.txt (só as
// que ainda não passaram nesta conexão).
func (a *App) rcTextLog(dev *radiacode.Device, seen map[string]bool) {
	lines, err := dev.TextMessages()
	if err != nil {
		return
	}
	now := time.Now()
	for _, l := range lines {
		when := now.Add(-time.Duration(l.AgoSec) * time.Second)
		// a mesma mensagem lida de novo vem com outro "segundos atrás":
		// a chave usa o minuto em que aconteceu
		key := when.Truncate(time.Minute).Format("15:04") + "|" + l.Text
		alt := when.Add(-30*time.Second).Truncate(time.Minute).Format("15:04") + "|" + l.Text
		if seen[key] || seen[alt] {
			continue
		}
		seen[key] = true
		a.logf("RadiaCode (mensagem do aparelho, %s): %s", when.Format("15:04:05"), l.Text)
	}
}
