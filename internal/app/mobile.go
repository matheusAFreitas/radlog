package app

import (
	"os"
	"time"

	"radlog/internal/radiacode"
)

// O app de Android (3.0.0) roda este mesmo App, sem o Wails: o pacote
// radlog/mobile cria o App com NewMobile, chama StartMobile e repassa as
// chamadas da tela (os mesmos métodos que o desktop expõe ao Wails). Assim a
// gravação por minuto, a dose, os alarmes, os alertas e a nuvem do celular
// são exatamente os do PC.
//
// No celular só sobe o que faz sentido lá: o laço do RC102 (Bluetooth pelo
// host do Android, ver internal/radiacode/ble_android.go), a nuvem, os
// segundos dos alertas e a gravação pendente. Ficam de fora o GQ GMC-800
// (só funciona por cabo), a bandeja, o radar e a chuva, o atualizador do PC,
// a sincronização pela rede e o início com o Windows.

// NewMobile cria o App do celular com a pasta de dados dir (a pasta privada
// do app no Android). emit recebe os eventos que no desktop iriam para a
// tela pelo Wails.
func NewMobile(dir string, emit func(name string, data any)) *App {
	a := NewApp()
	a.dataDir = dir
	a.mobile = true
	a.emitHook = emit
	return a
}

// SetMobileOpenURL troca quem abre endereços (o login do Google): no
// Android, o navegador do sistema.
func SetMobileOpenURL(f func(string)) {
	cloudOpenURL = func(_ *App, u string) { f(u) }
}

// SetMobileHost dá o nome do aparelho (o modelo do celular), que aparece
// na lista de aparelhos da nuvem.
func SetMobileHost(name string) { mobileHost = name }

// IsMobile diz se este App é o do celular.
func (a *App) IsMobile() bool { return a.mobile }

// StartMobile é o startup do celular.
func (a *App) StartMobile() {
	_ = os.MkdirAll(a.dataDir, 0o755)
	a.logStart()
	a.logf("modo celular (Android)")
	a.wdReport()
	radiacode.BLELogf = a.logf
	a.settings = a.loadSettings()
	a.loadLastCal()
	a.loadCalHist()
	a.mu.Lock()
	if a.migrateThrLocked() {
		a.saveSettingsLocked()
	}
	a.mu.Unlock()
	a.loadPeriods()
	a.loadNotes()
	a.loadMeasurements()
	a.events = a.loadEvents()
	if a.settingsBroken {
		a.logEvent("Não consegui ler as configurações (arquivo estragado): voltaram ao padrão. Confira os alarmes e a calibração")
	}
	a.store = a.openStore()
	a.goLoop("alertSecondsLoop", a.alertSecondsLoop)
	a.goLoop("pendingRetry", a.pendingRetry)
	a.goLoop("cloudLoop", a.cloudLoop)
	a.goLoop("rcLoop", a.rcLoop)
	a.goLoop("watchdog", a.watchdogLoop)
	a.goSafe("rcBackfill", func() { a.rcDoseBackfill(); a.rcLogBackfill() })
}

// StopMobile grava o que falta (o sistema vai fechar o app).
func (a *App) StopMobile() {
	a.logf("==== fechando o app (celular)")
	a.closing.Store(true)
	a.rc.mu.Lock()
	if d := a.rc.dev; d != nil {
		d.Close()
	}
	a.rcDoseSaveLiveLocked(time.Now())
	a.rc.mu.Unlock()
	if a.store != nil {
		_ = a.store.FlushOrKeep()
	}
	a.flushWrites()
}

// MobileFlush grava o que está pendente sem fechar (o Android pode matar o
// app em segundo plano a qualquer hora).
func (a *App) MobileFlush() {
	a.rc.mu.Lock()
	a.rcDoseSaveLiveLocked(time.Now())
	a.rc.mu.Unlock()
	if a.store != nil {
		_ = a.store.Flush()
	}
	a.flushWrites()
}
