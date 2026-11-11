package app

import "runtime"

// AppVersion é a versão do app. Ao mudar, atualize também build/windows/winres.json
// e o CHANGELOG.md.
const AppVersion = "2.60.1"

// GetVersion devolve a versão do app.
func (a *App) GetVersion() string { return AppVersion }

// GetPlatform devolve o sistema ("windows", "linux"...), para a tela
// esconder o que só existe no Windows (bandeja, iniciar com o sistema).
func (a *App) GetPlatform() string { return runtime.GOOS }
