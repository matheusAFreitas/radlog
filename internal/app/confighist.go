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
	"time"

	"radlog/internal/gmc"
)

// ---------- histórico de configurações (2.1) ----------
//
// Tudo o que muda a comparação entre períodos fica registrado com a data:
// limites, tempo morto, calibração do GMC, saturação, modo de gravação da
// memória e alterações feitas na configuração do contador. Uma linha por
// mudança em configuracoes-historico.jsonl (entra no backup).
// A primeira vez que o app roda com isto, grava o estado inicial.

const cfgHistFile = "configuracoes-historico.jsonl"

// ConfigChange é uma mudança registrada.
type ConfigChange struct {
	T      int64  `json:"t"`
	Field  string `json:"campo"`
	From   string `json:"de"` // "" = primeiro registro
	To     string `json:"para"`
	Source string `json:"origem"` // "você", "contador", "app"
}

func (a *App) cfgHistPath() string { return filepath.Join(a.dataDir, cfgHistFile) }

// cfgSnapshotLocked descreve, campo a campo, a configuração atual. Chame com a.mu.
func (a *App) cfgSnapshotLocked() map[string]string {
	s := a.settings
	lv := s.Levels.Norm()
	cal := a.convCalLocked()
	onOff := func(on bool, txt string) string {
		if !on {
			return "desligado"
		}
		return txt
	}
	m := map[string]string{
		"Aviso de atenção":   onOff(s.AttentionEnabled, "a partir de "+thrDesc(lv.AttentionUSv, lv.AttentionCPM, cal)),
		"Alarme":             onOff(s.AlarmEnabled, "a partir de "+thrDesc(s.AlarmUSv, s.AlarmCPM, cal)),
		"Alarme de perigo":   "a partir de " + thrDesc(lv.DangerUSv, lv.DangerCPM, cal),
		"Tempo morto":        onOff(lv.DeadTimeOn, fmt.Sprintf("ligado, τ = %s µs", fmtBR(lv.DeadTimeUs, 0))),
		"Saturação do GMC":   fmt.Sprintf("fim da escala %s µSv/h, queda brusca a partir de %s µSv/h", fmtBR(lv.SatMaxUSv, 0), fmtBR(lv.SatDropUSv, 0)),
		"Saturação do RC102": fmt.Sprintf("fim da escala %s µSv/h", fmtBR(lv.RcSatUSv, 0)),
		"Calibração do GMC":  calDesc(a.cal),
		"Calibração em uso":  a.calInUseDesc(),
	}
	if a.devSaveMode != "" {
		m["Gravação na memória do GMC"] = a.devSaveMode
	}
	return m
}

// calDesc resume a curva: o 1º ponto (o que vale no fundo) e de onde veio.
func calDesc(c gmc.Calibration) string {
	if len(c.Points) == 0 {
		return "sem pontos"
	}
	var pts []string
	for _, p := range c.Points {
		pts = append(pts, fmt.Sprintf("%s CPM = %s µSv/h", fmtBR(p.CPM, 0), fmtBR(p.USv, 2)))
	}
	k := c.Points[0].USv / c.Points[0].CPM
	return fmt.Sprintf("%s µSv/h por CPM no fundo (%s; %s)", fmtBR(k, 5), c.Source, strings.Join(pts, ", "))
}

// readCfgHist lê o histórico (do mais velho para o mais novo).
func (a *App) readCfgHist() []ConfigChange {
	b, err := os.ReadFile(a.cfgHistPath())
	if err != nil {
		return nil
	}
	return parseCfgHist(b)
}

func parseCfgHist(b []byte) []ConfigChange {
	var out []ConfigChange
	sc := bufio.NewScanner(bytes.NewReader(b))
	sc.Buffer(make([]byte, 1<<16), 1<<20)
	for sc.Scan() {
		var c ConfigChange
		if json.Unmarshal(sc.Bytes(), &c) == nil && c.T > 0 && c.Field != "" {
			out = append(out, c)
		}
	}
	sort.SliceStable(out, func(i, j int) bool { return out[i].T < out[j].T })
	return out
}

// cfgRecord compara a configuração atual com a última registrada e anota o
// que mudou. source: "você", "contador" ou "app".
func (a *App) cfgRecord(source string) {
	if a.dataDir == "" {
		return
	}
	a.cfgMu.Lock()
	defer a.cfgMu.Unlock()
	last := map[string]string{}
	for _, c := range a.readCfgHist() {
		last[c.Field] = c.To
	}
	a.mu.Lock()
	now := a.cfgSnapshotLocked()
	a.mu.Unlock()
	var keys []string
	for k := range now {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	t := time.Now().UnixMilli()
	var lines []string
	for _, k := range keys {
		old, ok := last[k]
		if ok && old == now[k] {
			continue
		}
		src := source
		if !ok {
			src = "início do registro"
		}
		b, _ := json.Marshal(ConfigChange{T: t, Field: k, From: old, To: now[k], Source: src})
		lines = append(lines, string(b))
	}
	if len(lines) == 0 {
		return
	}
	f, err := os.OpenFile(a.cfgHistPath(), os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o644)
	if err != nil {
		return
	}
	defer f.Close()
	f.WriteString(strings.Join(lines, "\n") + "\n")
}

// cfgNote anota uma mudança que não é um campo fixo (ex.: a configuração do
// contador alterada pelo app).
func (a *App) cfgNote(field, to, source string) {
	if a.dataDir == "" {
		return
	}
	a.cfgMu.Lock()
	defer a.cfgMu.Unlock()
	b, _ := json.Marshal(ConfigChange{T: time.Now().UnixMilli(), Field: field, To: to, Source: source})
	f, err := os.OpenFile(a.cfgHistPath(), os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o644)
	if err != nil {
		return
	}
	defer f.Close()
	f.Write(append(b, '\n'))
}

// GetConfigHistory devolve o histórico, do mais novo para o mais velho.
func (a *App) GetConfigHistory() []ConfigChange {
	a.cfgMu.Lock()
	h := a.readCfgHist()
	a.cfgMu.Unlock()
	out := make([]ConfigChange, 0, len(h))
	for i := len(h) - 1; i >= 0; i-- {
		out = append(out, h[i])
	}
	return out
}

// cfgMerge junta o histórico vindo de um backup (sem repetir, em ordem).
func (a *App) cfgMerge(data []byte) int {
	a.cfgMu.Lock()
	defer a.cfgMu.Unlock()
	cur := a.readCfgHist()
	key := func(c ConfigChange) string { b, _ := json.Marshal(c); return string(b) }
	have := map[string]bool{}
	for _, c := range cur {
		have[key(c)] = true
	}
	added := 0
	for _, c := range parseCfgHist(data) {
		if !have[key(c)] {
			have[key(c)] = true
			cur = append(cur, c)
			added++
		}
	}
	if added == 0 {
		return 0
	}
	sort.SliceStable(cur, func(i, j int) bool { return cur[i].T < cur[j].T })
	var buf bytes.Buffer
	for _, c := range cur {
		b, _ := json.Marshal(c)
		buf.Write(append(b, '\n'))
	}
	tmp := a.cfgHistPath() + ".tmp"
	if os.WriteFile(tmp, buf.Bytes(), 0o644) == nil {
		os.Rename(tmp, a.cfgHistPath())
	}
	return added
}
