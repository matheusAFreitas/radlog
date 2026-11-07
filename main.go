// RadLog: registro de radiação do contador Geiger GQ GMC-800 e do RadiaCode.
// O app fica em internal/app; aqui só entra a interface embutida.
package main

import (
	"embed"
	"log"

	"radlog/internal/app"
)

//go:embed all:frontend/dist
var assets embed.FS

func main() {
	if err := app.Run(assets); err != nil {
		log.Fatal(err)
	}
}
