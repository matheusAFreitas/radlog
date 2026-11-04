#!/bin/sh
# Compila o GMC Monitor no Linux (precisa de Go, libgtk-3-dev e
# libwebkit2gtk-4.1-dev; em distros com a 4.0, troque webkit2_41 por nada).
set -e
cd "$(dirname "$0")"
VER=$(sed -n 's/^const AppVersion = "\(.*\)"/\1/p' version.go)
echo "Testes..."
go test ./internal/...
echo "Compilando GMC Monitor $VER..."
CGO_ENABLED=1 go build -trimpath -tags desktop,production,webkit2_41 -ldflags "-w -s" -o "GMC-Monitor-v$VER" .
echo "Pronto: ./GMC-Monitor-v$VER"
