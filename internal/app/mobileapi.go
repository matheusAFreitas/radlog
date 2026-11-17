package app

// ---------- o que a tela do celular chama (A4 da auditoria 2.70) ----------
//
// O app de Android usa este mesmo pacote: a tela dele (HTML num WebView)
// chama os métodos do App pelo nome, como o desktop pelo Wails. Esta lista
// é o contrato com o celular: o teste TestMobileAPI trava o nome, os
// argumentos, o retorno e os campos JSON de cada um, para uma mudança feita
// pensando no PC não quebrar o celular sem ninguém ver. Mudou de propósito?
// Rode `go test ./internal/app -run TestMobileAPI -update` e confira a tela
// do celular (android/app/src/main/assets/www/app.js) antes do merge.

// MobileMethods são os métodos do App que a tela do celular pode chamar.
var MobileMethods = []string{
	"CheckCloudCode",
	"CloudCancel",
	"CloudDisconnect",
	"CloudLogin",
	"CloudLoginWait",
	"CloudOpen",
	"CloudSyncNow",
	"GetChart",
	"GetCloud",
	"GetEvents",
	"GetLogs",
	"GetPeriods",
	"GetRadiacode",
	"GetRcSettings",
	"GetSettings",
	"GetWhereabouts",
	"LeaveHome",
	"SaveSettings",
	"ScanRadiacodeBLE",
	"SetRadiacode",
	"SetRcSettings",
	"SetWhere",
	"UndoRcSettings",
}
