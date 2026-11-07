package app

import (
	"bytes"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	wruntime "github.com/wailsapp/wails/v2/pkg/runtime"

	"radlog/internal/gmc"
)

// Ajustes do próprio contador (não confundir com Settings, que são do app).

func (a *App) backupDir() string { return filepath.Join(a.dataDir, "configuracao") }

// saveConfigBackup guarda uma cópia dos 512 bytes da configuração.
func (a *App) saveConfigBackup(cfg []byte, tag string) (string, error) {
	dir := a.backupDir()
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", err
	}
	a.mu.Lock()
	serial := a.info.Serial
	a.mu.Unlock()
	name := "config-" + time.Now().Format("2006-01-02_150405") + "-" + tag
	if serial != "" {
		name += "-" + strings.ReplaceAll(serial, " ", "")
	}
	path := filepath.Join(dir, name+".bin")
	return path, os.WriteFile(path, cfg, 0o644)
}

// exclusive pausa a leitura ao vivo e roda fn com o contador só para si.
func (a *App) exclusive(fn func(d *gmc.Device) error) error {
	a.mu.Lock()
	d := a.dev
	a.mu.Unlock()
	if d == nil {
		return errors.New("contador desconectado")
	}
	if a.downloading.Load() {
		return errors.New("espere terminar de baixar o histórico")
	}
	if !a.cfgBusy.CompareAndSwap(false, true) {
		return errors.New("já estou mexendo na configuração do contador")
	}
	defer a.cfgBusy.Store(false)
	time.Sleep(1200 * time.Millisecond) // deixa a leitura ao vivo em andamento terminar
	return fn(d)
}

// GetDeviceConfig lê os ajustes do contador. Na primeira leitura de cada
// conexão guarda uma cópia de segurança.
func (a *App) GetDeviceConfig() (gmc.Config800, error) {
	var out gmc.Config800
	err := a.exclusive(func(d *gmc.Device) error {
		raw, err := d.Config()
		if err != nil {
			return err
		}
		out, err = gmc.DecodeConfig800(raw)
		if err != nil {
			return err
		}
		a.mu.Lock()
		first := !a.cfgBackedUp
		a.cfgBackedUp = true
		a.mu.Unlock()
		if first {
			_, _ = a.saveConfigBackup(raw, "lida")
		}
		return nil
	})
	return out, err
}

// SetDeviceConfig grava os ajustes no contador. Antes guarda uma cópia; se a
// gravação falhar, tenta pôr a configuração antiga de volta.
func (a *App) SetDeviceConfig(c gmc.Config800) (gmc.Config800, error) {
	var result gmc.Config800
	err := a.exclusive(func(d *gmc.Device) error {
		orig, err := d.Config()
		if err != nil {
			return err
		}
		before, err := gmc.DecodeConfig800(orig)
		if err != nil {
			return err
		}
		if !before.Editable {
			return fmt.Errorf("não vou gravar: a configuração do contador tem valores que o app não reconhece (%s)",
				strings.Join(before.Problems, "; "))
		}
		next, err := gmc.EncodeConfig800(orig, c)
		if err != nil {
			return err
		}
		if bytes.Equal(next, orig) {
			result = before
			return nil
		}
		backup, err := a.saveConfigBackup(orig, "antes")
		if err != nil {
			return fmt.Errorf("não consegui salvar a cópia de segurança, então não gravei nada: %w", err)
		}

		if err := d.WriteConfig(next); err != nil {
			msg := "a gravação falhou (" + err.Error() + ")"
			if rerr := d.WriteConfig(orig); rerr == nil {
				msg += "; a configuração anterior foi restaurada"
			} else {
				msg += "; não consegui restaurar a anterior — use \"Restaurar cópia\" com " + filepath.Base(backup)
			}
			a.logEvent("Erro ao gravar a configuração do contador")
			return errors.New(msg)
		}

		result, _ = gmc.DecodeConfig800(next)
		a.mu.Lock()
		a.cal = result.Calibration
		a.mu.Unlock()
		a.saveLastCal(result.Calibration)
		a.logEvent("Configuração do contador alterada: " + gmc.DescribeChanges(before, result))
		if v, ok := result.Values["saveMode"]; ok {
			a.mu.Lock()
			a.devSaveMode = gmc.SaveModeName(v)
			a.mu.Unlock()
		}
		a.cfgNote("Configuração do contador", gmc.DescribeChanges(before, result), "você (pelo app)")
		a.cfgRecord("você (pelo app)")
		return nil
	})
	return result, err
}

// RestoreDeviceConfig grava no contador uma cópia salva antes.
func (a *App) RestoreDeviceConfig() (string, error) {
	_ = os.MkdirAll(a.backupDir(), 0o755)
	path, err := wruntime.OpenFileDialog(a.ctx, wruntime.OpenDialogOptions{
		Title:            "Escolha a cópia da configuração",
		DefaultDirectory: a.backupDir(),
		Filters:          []wruntime.FileFilter{{DisplayName: "Configuração do GMC (*.bin)", Pattern: "*.bin"}},
	})
	if err != nil || path == "" {
		return "", err
	}
	raw, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}
	dec, err := gmc.DecodeConfig800(raw)
	if err != nil {
		return "", err
	}
	if !dec.Editable {
		return "", fmt.Errorf("esse arquivo não parece uma configuração válida do GMC-800 (%s)", strings.Join(dec.Problems, "; "))
	}
	err = a.exclusive(func(d *gmc.Device) error {
		cur, err := d.Config()
		if err == nil {
			_, _ = a.saveConfigBackup(cur, "antes-de-restaurar")
		}
		if err := d.WriteConfig(raw); err != nil {
			return err
		}
		defer a.cfgRecord("você (cópia restaurada)")
		a.mu.Lock()
		a.cal = dec.Calibration
		a.mu.Unlock()
		a.saveLastCal(dec.Calibration)
		a.logEvent("Configuração do contador restaurada de " + filepath.Base(path))
		return nil
	})
	if err != nil {
		return "", err
	}
	return filepath.Base(path), nil
}

// OpenConfigBackups abre a pasta com as cópias da configuração.
func (a *App) OpenConfigBackups() error {
	_ = os.MkdirAll(a.backupDir(), 0o755)
	return openFolder(a.backupDir())
}

// SnapshotDeviceConfig guarda na memória a configuração atual, para depois
// comparar com DiffDeviceConfig (serve para descobrir onde fica cada ajuste:
// tira a foto, muda algo no menu do contador, compara).
func (a *App) SnapshotDeviceConfig() error {
	return a.exclusive(func(d *gmc.Device) error {
		raw, err := d.Config()
		if err != nil {
			return err
		}
		a.mu.Lock()
		a.cfgSnapshot = raw
		a.mu.Unlock()
		return nil
	})
}

// DiffDeviceConfig lê a configuração de novo e diz quais bytes mudaram desde
// a foto.
func (a *App) DiffDeviceConfig() ([]gmc.ByteDiff, error) {
	a.mu.Lock()
	snap := a.cfgSnapshot
	a.mu.Unlock()
	if snap == nil {
		return nil, errors.New("tire a foto da configuração primeiro")
	}
	var diff []gmc.ByteDiff
	err := a.exclusive(func(d *gmc.Device) error {
		raw, err := d.Config()
		if err != nil {
			return err
		}
		diff = gmc.DiffConfig(snap, raw)
		return nil
	})
	if diff == nil {
		diff = []gmc.ByteDiff{}
	}
	return diff, err
}
