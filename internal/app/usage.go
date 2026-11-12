package app

import (
	"encoding/json"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

// ---------- uso do app (2.72) ----------
//
// Spec ux/uso-do-app.html. Conta, só neste computador, quantas vezes cada
// página, aba e botão foi aberto e por quanto tempo a pessoa ficou olhando.
// A tela manda os totais a cada minuto (RecordUsage); aqui ficam só totais
// por dia, sem horário, em uso.json. Fica fora do backup, da sincronização e
// do PDF (as listas de arquivos de lá são fechadas). Desligado por padrão.
//
// Chaves (a tela monta): "p:pagina", "a:pagina/aba", "c:pagina/botao",
// "h:pagina/cartao" (Como funciona), "s:pagina/seletor=valor".

const usageFile = "uso.json"

// guarda no máximo isto de dias; o resto é cortado ao gravar
const usageMaxDays = 400

// UsageCount é o total de uma chave: vezes aberta (ou cliques) e segundos.
type UsageCount struct {
	N int   `json:"n,omitempty"`
	S int64 `json:"s,omitempty"`
}

type usageData struct {
	Since int64                            `json:"since"` // ms: quando começou a contar
	Days  map[string]map[string]UsageCount `json:"days"`  // "2006-01-02" → chave → total
}

// UsageInfo é o que a tela de Configurações › Dados mostra.
type UsageInfo struct {
	On     bool                  `json:"on"`
	Since  int64                 `json:"since"`
	Days   int                   `json:"days"` // dias com alguma contagem no período
	Totals map[string]UsageCount `json:"totals"`
}

func (a *App) usagePath() string { return filepath.Join(a.dataDir, usageFile) }

func (a *App) usageLoadLocked() {
	if a.usageLoaded {
		return
	}
	a.usageLoaded = true
	var d usageData
	if a.dataDir != "" && a.readStateJSON(a.usagePath(), "uso do app", &d) {
		a.usage = d
	}
	if a.usage.Days == nil {
		a.usage.Days = map[string]map[string]UsageCount{}
	}
}

func (a *App) usageSaveLocked() {
	if a.dataDir == "" {
		return
	}
	if len(a.usage.Days) > usageMaxDays {
		ks := make([]string, 0, len(a.usage.Days))
		for k := range a.usage.Days {
			ks = append(ks, k)
		}
		sort.Strings(ks)
		for _, k := range ks[:len(ks)-usageMaxDays] {
			delete(a.usage.Days, k)
		}
	}
	b, _ := json.Marshal(a.usage)
	if err := writeFileAtomic(a.usagePath(), b, 0o644); err != nil {
		a.logf("uso do app: %v", err)
	}
}

// chave aceita: prefixo conhecido, curta, sem quebra de linha
func usageKeyOK(k string) bool {
	if len(k) < 3 || len(k) > 120 || strings.ContainsAny(k, "\n\r\t") || k[1] != ':' {
		return false
	}
	return strings.ContainsRune("pachs", rune(k[0]))
}

// RecordUsage soma o que a tela juntou desde o último envio. Não faz nada
// com a contagem desligada.
func (a *App) RecordUsage(batch map[string]UsageCount) {
	a.recordUsageAt(time.Now(), batch)
}

func (a *App) recordUsageAt(now time.Time, batch map[string]UsageCount) {
	a.mu.Lock()
	defer a.mu.Unlock()
	if !a.settings.UsageOn || len(batch) == 0 {
		return
	}
	a.usageLoadLocked()
	if a.usage.Since == 0 {
		a.usage.Since = now.UnixMilli()
	}
	day := now.Format("2006-01-02")
	m := a.usage.Days[day]
	if m == nil {
		m = map[string]UsageCount{}
		a.usage.Days[day] = m
	}
	n := 0
	for k, c := range batch {
		if !usageKeyOK(k) || n >= 500 {
			continue
		}
		n++
		// um envio por minuto: mais que isto é engano da tela
		c.N = min(max(c.N, 0), 1000)
		c.S = min(max(c.S, 0), 600)
		t := m[k]
		t.N += c.N
		t.S += c.S
		m[k] = t
	}
	a.usageSaveLocked()
}

// GetUsage soma os últimos days dias (0 = tudo).
func (a *App) GetUsage(days int) UsageInfo {
	return a.usageAt(time.Now(), days)
}

func (a *App) usageAt(now time.Time, days int) UsageInfo {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.usageLoadLocked()
	out := UsageInfo{On: a.settings.UsageOn, Since: a.usage.Since, Totals: map[string]UsageCount{}}
	from := ""
	if days > 0 {
		from = now.AddDate(0, 0, -(days - 1)).Format("2006-01-02")
	}
	for d, m := range a.usage.Days {
		if d < from {
			continue
		}
		out.Days++
		for k, c := range m {
			t := out.Totals[k]
			t.N += c.N
			t.S += c.S
			out.Totals[k] = t
		}
	}
	return out
}

// ResetUsage apaga a contagem; se ainda estiver ligada, recomeça agora.
func (a *App) ResetUsage() UsageInfo {
	a.mu.Lock()
	a.usageLoadLocked()
	a.usage = usageData{Days: map[string]map[string]UsageCount{}}
	if a.settings.UsageOn {
		a.usage.Since = time.Now().UnixMilli()
	}
	a.usageSaveLocked()
	a.mu.Unlock()
	return a.GetUsage(30)
}

// SetUsageOn liga ou desliga a contagem (desligar não apaga o que já contou).
func (a *App) SetUsageOn(on bool) UsageInfo {
	a.mu.Lock()
	a.usageLoadLocked()
	a.settings.UsageOn = on
	a.saveSettingsLocked()
	if on && a.usage.Since == 0 {
		a.usage.Since = time.Now().UnixMilli()
		a.usageSaveLocked()
	}
	a.mu.Unlock()
	return a.GetUsage(30)
}
