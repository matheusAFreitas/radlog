package app

import (
	"context"
	"io/fs"
	"os"
	"time"

	"github.com/wailsapp/wails/v2"
	"github.com/wailsapp/wails/v2/pkg/options"
	"github.com/wailsapp/wails/v2/pkg/options/assetserver"
	"github.com/wailsapp/wails/v2/pkg/options/windows"
)

// id da trava de uma cópia só (não mudar: as versões antigas usam o mesmo)
const instanceID = "gmc-monitor-7f3c2a"

// Run abre a janela e roda o app até ela fechar. assets é a interface
// (frontend/dist), embutida pelo main.go da raiz.
func Run(assets fs.FS) error {
	// Deixa o WebView2 tocar o som do alarme sem precisar de um clique antes.
	if os.Getenv("WEBVIEW2_ADDITIONAL_BROWSER_ARGUMENTS") == "" {
		os.Setenv("WEBVIEW2_ADDITIONAL_BROWSER_ARGUMENTS", "--autoplay-policy=no-user-gesture-required")
	}
	// aberta pela versão antiga (troca de versão): espera ela fechar
	if startedUpdating() {
		waitPreviousExit(20 * time.Second)
	}
	// pasta de dados (na primeira abertura da 2.0, leva os dados da pasta
	// "GMC Monitor"; migrate.go). Antes da janela: o WebView2 guarda o dele lá.
	migNow = prepareDataDir()
	migDocsMoved, migDocsErr = migrateDocs(docsBase())
	// a Wails repassa os argumentos para a cópia já aberta: vai junto quem
	// somos, para ela saber se é outra versão (handoff.go)
	os.Args = append(os.Args[:1], tagArgs(os.Args[1:])...)
	app := NewApp()

	err := wails.Run(&options.App{
		Title:     appTitle,
		Width:     1280,
		Height:    880,
		MinWidth:  420,
		MinHeight: 560,
		AssetServer: &assetserver.Options{
			Assets: assets,
		},
		BackgroundColour: &options.RGBA{R: 13, G: 16, B: 18, A: 255},
		OnStartup:        app.startup,
		OnShutdown:       app.shutdown,
		OnBeforeClose: func(ctx context.Context) bool {
			return app.onBeforeClose()
		},
		// iniciado com o Windows: abre direto na bandeja
		StartHidden: startedInTray(),
		Bind:        []interface{}{app},
		// Duas cópias abertas brigariam pela mesma porta serial.
		SingleInstanceLock: &options.SingleInstanceLock{
			UniqueId: instanceID,
			OnSecondInstanceLaunch: func(d options.SecondInstanceData) {
				app.secondInstance(d.Args)
			},
		},
		Windows: &windows.Options{
			Theme: windows.Dark,
			// uma pasta só para o WebView2; sem isso ele cria uma pasta
			// nova em %APPDATA% para cada nome de .exe (uma por versão)
			WebviewUserDataPath: webviewDir(),
		},
	})
	return err
}
