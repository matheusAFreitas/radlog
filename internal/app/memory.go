package app

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"time"

	"radlog/internal/gmc"
)

// MemoryInfo é quanto da memória de histórico do contador estava ocupado na
// última vez que o histórico foi baixado.
type MemoryInfo struct {
	Known      bool   `json:"known"`
	UsedBytes  int    `json:"usedBytes"`
	TotalBytes int    `json:"totalBytes"`
	At         int64  `json:"at"` // ms
	Serial     string `json:"serial"`
}

func (a *App) memoryPath() string { return filepath.Join(a.dataDir, "memoria.json") }

// usedFlash conta os bytes até o último que não é 0xFF (memória vazia).
func usedFlash(raw []byte) int {
	n := len(raw)
	for n > 0 && raw[n-1] == 0xFF {
		n--
	}
	return n
}

func (a *App) saveMemoryInfo(raw []byte) {
	a.mu.Lock()
	serial := a.info.Serial
	a.mu.Unlock()
	m := MemoryInfo{Known: true, UsedBytes: usedFlash(raw), TotalBytes: gmc.FlashSize, At: time.Now().UnixMilli(), Serial: serial}
	b, _ := json.MarshalIndent(m, "", "  ")
	_ = writeFileAtomic(a.memoryPath(), b, 0o644)
}

// GetMemoryInfo devolve o uso da memória do contador conectado (se o
// histórico dele já foi baixado alguma vez).
func (a *App) GetMemoryInfo() MemoryInfo {
	b, err := os.ReadFile(a.memoryPath())
	if err != nil {
		return MemoryInfo{TotalBytes: gmc.FlashSize}
	}
	var m MemoryInfo
	if json.Unmarshal(b, &m) != nil {
		return MemoryInfo{TotalBytes: gmc.FlashSize}
	}
	a.mu.Lock()
	serial := a.info.Serial
	a.mu.Unlock()
	if serial != "" && m.Serial != "" && serialKey(m.Serial) != serialKey(serial) {
		return MemoryInfo{TotalBytes: gmc.FlashSize} // é de outro contador
	}
	m.TotalBytes = gmc.FlashSize
	return m
}

// serialKey compara números de série sem ligar para espaços e maiúsculas
// (até a 1.13.0 o app guardava "0a1b2c..."; agora "0A 1B 2C ...").
func serialKey(s string) string { return strings.ToLower(strings.ReplaceAll(s, " ", "")) }
