package app

import (
	"encoding/json"
	"reflect"
)

// ---------- configurações no backup (2.74) ----------
//
// O backup leva as preferências que valem em qualquer PC: alarme, níveis,
// som, bandeja, pulso, idioma, atualização e medição. Não leva o lugar da
// chuva (nem se ela está ligada), nem o que é só deste PC: porta, Bluetooth,
// pastas, tamanho da tela, início com o Windows, sincronização, uso do app.
// Ao importar, a tela pergunta se quer usar as do backup.

const backupSettingsFile = "settings.json"

// portableCopy copia de src para dst só os campos que viajam no backup.
func portableCopy(dst *Settings, src Settings) {
	dst.AutoConnect = src.AutoConnect
	dst.AlarmEnabled = src.AlarmEnabled
	dst.AlarmCPM = src.AlarmCPM
	dst.AlarmUSv = src.AlarmUSv
	dst.AlarmSound = src.AlarmSound
	dst.AlarmRaise = src.AlarmRaise
	dst.AttentionEnabled = src.AttentionEnabled
	dst.RainHide = src.RainHide
	dst.Levels = src.Levels
	dst.AwayAskMin = src.AwayAskMin
	dst.TrayOnClose = src.TrayOnClose
	dst.TrayPlain = src.TrayPlain
	dst.TrayDevice = src.TrayDevice
	dst.PulseOff = src.PulseOff
	dst.PulseStrip = src.PulseStrip
	dst.PulseLogo = src.PulseLogo
	dst.NoUpdateCheck = src.NoUpdateCheck
	dst.KeepOldVersions = src.KeepOldVersions
	dst.Lang = src.Lang
	dst.NoMeasureReport = src.NoMeasureReport
}

// portableSettings é o que vai para o backup.
func portableSettings(s Settings) Settings {
	var p Settings
	portableCopy(&p, s)
	return p
}

// cpmFree tira os limites em CPM que saem dos µSv/h pela calibração de cada
// PC: com eles, o mesmo alarme em µSv/h pareceria diferente.
func cpmFree(s Settings) Settings {
	if s.AlarmUSv > 0 {
		s.AlarmCPM = 0
	}
	if s.Levels.AttentionUSv > 0 {
		s.Levels.AttentionCPM = 0
	}
	if s.Levels.DangerUSv > 0 {
		s.Levels.DangerCPM = 0
	}
	return s
}

// portableDiff conta quantos ajustes do backup são diferentes dos daqui
// (cada campo dos níveis conta como um).
func portableDiff(here, backup Settings) int {
	h, b := cpmFree(portableSettings(here)), cpmFree(portableSettings(backup))
	return diffFields(reflect.ValueOf(h), reflect.ValueOf(b))
}

func diffFields(h, b reflect.Value) int {
	n := 0
	for i := 0; i < h.NumField(); i++ {
		hf, bf := h.Field(i), b.Field(i)
		if hf.Kind() == reflect.Struct {
			n += diffFields(hf, bf)
		} else if !reflect.DeepEqual(hf.Interface(), bf.Interface()) {
			n++
		}
	}
	return n
}

// backupSettingsJSON é o configuracoes.json do backup.
func (a *App) backupSettingsJSON() []byte {
	a.mu.Lock()
	p := portableSettings(a.settings)
	a.mu.Unlock()
	b, _ := json.MarshalIndent(p, "", "  ")
	return b
}

// stashBackupSettings guarda as configurações de um backup importado e diz
// quantos campos mudariam (0: nada a perguntar).
func (a *App) stashBackupSettings(b []byte) int {
	s := defaultSettings()
	if len(b) == 0 || json.Unmarshal(b, &s) != nil {
		a.mu.Lock()
		a.bkSettings = nil
		a.mu.Unlock()
		return 0
	}
	s.Levels = s.Levels.Norm()
	s.AwayAskMin = normAwayAsk(s.AwayAskMin)
	a.mu.Lock()
	defer a.mu.Unlock()
	n := portableDiff(a.settings, s)
	if n == 0 {
		a.bkSettings = nil
		return 0
	}
	a.bkSettings = &s
	return n
}

// ApplyBackupSettings usa as configurações do último backup importado (só
// os campos que viajam; o resto fica como está).
func (a *App) ApplyBackupSettings() Settings {
	a.mu.Lock()
	bk := a.bkSettings
	a.bkSettings = nil
	s := a.settings
	a.mu.Unlock()
	if bk == nil {
		return a.GetSettings()
	}
	portableCopy(&s, *bk)
	s = a.saveSettingsFrom(s, "backup")
	a.logEvent("Configurações do backup aplicadas")
	return s
}
