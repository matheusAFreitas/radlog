//go:build !windows

package app

// Fora do Windows (testes no Linux) não há bandeja nem início automático.

const trayAvailable = false

func (a *App) startTray()                                                            {}
func (a *App) stopTray()                                                             {}
func (a *App) trayApply(tip string, plain bool, level, key string, mk func() []byte) {}
func setAutostart(on bool) error                                                     { return nil }
func autostartPath() string                                                          { return "" }
func startupDisabled() bool                                                          { return false }
func enableStartup() error                                                           { return nil }

type trayState struct{}

func (a *App) trayRelabel() {}

func renameAutostart() {}
