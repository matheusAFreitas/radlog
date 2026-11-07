#!/bin/sh
# Compila o RadLog no Linux (precisa de Go, libgtk-3-dev e
# libwebkit2gtk-4.1-dev; em distros com a 4.0, troque webkit2_41 por nada).
set -e
cd "$(dirname "$0")/.."
VER=$(sed -n 's/^const AppVersion = "\(.*\)"/\1/p' internal/app/version.go)
echo "Testes..."
go test ./internal/...
echo "Compilando RadLog $VER..."
CGO_ENABLED=1 go build -trimpath -tags desktop,production,webkit2_41 -ldflags "-w -s" -o "RadLog-v$VER" .
echo "Pronto: ./RadLog-v$VER"
