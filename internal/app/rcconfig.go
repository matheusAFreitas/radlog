package app

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"radlog/internal/radiacode"
)

// ---------- ajustes gravados no Radiacode ----------
//
// Configurações › Radiacode mostra os ajustes do próprio aparelho (alarmes,
// som e vibração, tela, unidades) como foram lidos agora, e grava só quando
// a pessoa clica em "Gravar no Radiacode". Antes de gravar, o app guarda uma
// cópia do que estava; depois lê de volta e registra a mudança no histórico
// de configurações. "Desfazer a última gravação" grava a cópia de volta.
//
// Os pedidos passam pelo mesmo Device da leitura ao vivo (o Device serializa
// os pedidos), então não é preciso parar a leitura.

const rcCfgDir = "configuracao"

var errRcOff = errors.New("RC102 desconectado: conecte para ver e mudar os ajustes do aparelho")

func (a *App) rcCfgPath(name string) string { return filepath.Join(a.rcDir(), rcCfgDir, name) }

func (a *App) rcDevice() *radiacode.Device {
	a.rc.mu.Lock()
	defer a.rc.mu.Unlock()
	return a.rc.dev
}

// rcCfgBackup guarda os ajustes num .json (tag: "lida" ou "antes").
func (a *App) rcCfgBackup(s radiacode.Settings, dev *radiacode.Device, tag string) (string, error) {
	if a.dataDir == "" {
		return "", errors.New("sem pasta de dados")
	}
	if err := os.MkdirAll(a.rcCfgPath(""), 0o755); err != nil {
		return "", err
	}
	b, _ := json.MarshalIndent(map[string]any{
		"quando": time.Now().Format(time.RFC3339), "app": AppVersion,
		"modelo": dev.Model, "firmware": dev.Firmware, "ajustes": s,
	}, "", "  ")
	// 2.101.8: duas gravações no mesmo milissegundo não se sobrescrevem (a
	// fila do desfazer vai pelo nome): o horário anda 1 ms até achar um livre
	t := time.Now()
	path := a.rcCfgPath("ajustes-" + t.Format("2006-01-02_150405.000") + "-" + tag + ".json")
	for i := 0; i < 1000; i++ {
		if _, err := os.Stat(path); err != nil {
			break
		}
		t = t.Add(time.Millisecond)
		path = a.rcCfgPath("ajustes-" + t.Format("2006-01-02_150405.000") + "-" + tag + ".json")
	}
	return path, writeFileAtomic(path, b, 0o644)
}

// rcCfgLastBefore é a cópia "antes" mais nova.
func (a *App) rcCfgLastBefore() (radiacode.Settings, string, error) {
	ms, _ := filepath.Glob(a.rcCfgPath("ajustes-*-antes.json"))
	if len(ms) == 0 {
		return radiacode.Settings{}, "", errors.New("ainda não há gravação para desfazer")
	}
	sort.Strings(ms)
	p := ms[len(ms)-1]
	b, err := os.ReadFile(p)
	if err != nil {
		return radiacode.Settings{}, "", err
	}
	var v struct {
		Ajustes radiacode.Settings `json:"ajustes"`
	}
	if err := json.Unmarshal(b, &v); err != nil {
		return radiacode.Settings{}, "", fmt.Errorf("cópia %s ilegível: %w", filepath.Base(p), err)
	}
	return v.Ajustes, p, nil
}

// RcSettings é o que a tela recebe: os ajustes e se dá para desfazer.
type RcSettings struct {
	radiacode.Settings
	CanUndo bool   `json:"canUndo"`
	UndoAt  string `json:"undoAt"` // quando foi a última gravação (nome da cópia)
	// 2.101.6: o aparelho já estava assim; nada foi gravado
	Unchanged bool `json:"unchanged,omitempty"`
}

func (a *App) rcSettingsOut(s radiacode.Settings) RcSettings {
	out := RcSettings{Settings: s}
	if _, p, err := a.rcCfgLastBefore(); err == nil {
		out.CanUndo, out.UndoAt = true, filepath.Base(p)
	}
	return out
}

// GetRcSettings lê os ajustes do Radiacode. A primeira leitura de cada
// conexão guarda uma cópia.
func (a *App) GetRcSettings() (RcSettings, error) {
	dev := a.rcDevice()
	if dev == nil {
		return RcSettings{}, errRcOff
	}
	s, err := dev.ReadSettings()
	if err != nil {
		return RcSettings{}, err
	}
	a.rc.mu.Lock()
	first := !a.rc.cfgReadSince.Equal(time.UnixMilli(a.rc.st.Since))
	a.rc.cfgReadSince = time.UnixMilli(a.rc.st.Since)
	a.rc.mu.Unlock()
	if first {
		_, _ = a.rcCfgBackup(s, dev, "lida")
	}
	return a.rcSettingsOut(s), nil
}

// rcApply grava next no aparelho (cópia antes, relê, registra).
func (a *App) rcApply(next radiacode.Settings, what string) (RcSettings, error) {
	out, _, err := a.rcApplyCopy(next, what)
	return out, err
}

// rcApplyCopy é o rcApply que devolve também a cópia "antes" que criou ("" =
// não criou: nada mudou ou deu erro antes).
func (a *App) rcApplyCopy(next radiacode.Settings, what string) (RcSettings, string, error) {
	dev := a.rcDevice()
	if dev == nil {
		return RcSettings{}, "", errRcOff
	}
	if !a.rcCfgBusy.CompareAndSwap(false, true) {
		return RcSettings{}, "", errors.New("já estou gravando no RC102")
	}
	defer a.rcCfgBusy.Store(false)
	cur, err := dev.ReadSettings()
	if err != nil {
		return RcSettings{}, "", err
	}
	if p := next.Check(); len(p) > 0 {
		return a.rcSettingsOut(cur), "", fmt.Errorf("não gravei: %s", strings.Join(p, "; "))
	}
	if radiacode.DescribeChanges(cur, next) == "nada mudou" {
		out := a.rcSettingsOut(cur)
		out.Unchanged = true
		return out, "", nil
	}
	bk, err := a.rcCfgBackup(cur, dev, "antes")
	if err != nil {
		return a.rcSettingsOut(cur), "", fmt.Errorf("não consegui salvar a cópia de segurança, então não gravei nada: %w", err)
	}
	got, n, werr := dev.WriteSettings(cur, next)
	desc := radiacode.DescribeChanges(cur, got)
	if desc != "nada mudou" {
		a.logEvent("Ajustes do RC102 alterados" + what + ": " + desc)
		a.cfgNote("Ajustes do RC102", desc, "você (pelo app)")
	}
	// os alertas do Radiacode usam os alarmes 1 e 2 de taxa do aparelho
	if al, err := dev.Alarms(); err == nil {
		a.rcSet(func(s *RcState) { s.Alarms = &al })
	}
	if werr != nil {
		a.logEvent("Erro ao gravar os ajustes do RC102: " + werr.Error())
		return a.rcSettingsOut(got), bk, fmt.Errorf("a gravação parou no %dº ajuste (%s); o que já foi gravado ficou, e a cópia de antes está salva", n+1, werr.Error())
	}
	if left := radiacode.DescribeChanges(got, next); left != "nada mudou" {
		return a.rcSettingsOut(got), bk, fmt.Errorf("o aparelho aceitou, mas leu de volta diferente: %s", left)
	}
	return a.rcSettingsOut(got), bk, nil
}

// SetRcSettings grava os ajustes no Radiacode (só o que mudou).
func (a *App) SetRcSettings(next radiacode.Settings) (RcSettings, error) {
	return a.rcApply(next, "")
}

// UndoRcSettings grava de volta a cópia de antes da última gravação.
func (a *App) UndoRcSettings() (RcSettings, error) {
	prev, p, err := a.rcCfgLastBefore()
	if err != nil {
		return RcSettings{}, err
	}
	out, bk, err := a.rcApplyCopy(prev, " (desfeito)")
	if err == nil {
		// a cópia usada vira "desfeita" para o próximo desfazer pegar a anterior
		_ = os.Rename(p, p[:len(p)-len("-antes.json")]+"-desfeita.json")
		// e a cópia que o rcApply acabou de criar (se criou: com o aparelho já
		// igual à cópia não cria nenhuma) não é uma gravação nova
		if strings.HasSuffix(bk, "-antes.json") {
			_ = os.Rename(bk, bk[:len(bk)-len("-antes.json")]+"-desfazer.json")
		}
		un := out.Unchanged
		out = a.rcSettingsOut(out.Settings)
		out.Unchanged = un
	}
	return out, err
}

// ResetRcDose zera a dose acumulada do Radiacode.
func (a *App) ResetRcDose() error {
	dev := a.rcDevice()
	if dev == nil {
		return errRcOff
	}
	if err := dev.ResetDose(); err != nil {
		return err
	}
	a.logEvent("Dose acumulada do RC102 zerada pelo app")
	a.cfgNote("Dose acumulada do RC102", "zerada", "você (pelo app)")
	return nil
}

// ResetRcSpectrum recomeça o espectro da tela do Radiacode.
func (a *App) ResetRcSpectrum() error {
	dev := a.rcDevice()
	if dev == nil {
		return errRcOff
	}
	if err := dev.ResetSpectrum(); err != nil {
		return err
	}
	a.logEvent("Espectro da tela do RC102 recomeçado pelo app")
	return nil
}

// OpenRcConfigBackups abre a pasta das cópias dos ajustes do Radiacode.
func (a *App) OpenRcConfigBackups() error {
	_ = os.MkdirAll(a.rcCfgPath(""), 0o755)
	return openFolder(a.rcCfgPath(""))
}
