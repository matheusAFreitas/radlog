#!/bin/sh
# Confere que a versão é a mesma nos lugares que mudam a cada release:
# internal/app/version.go, build/windows/winres.json (quatro campos), o
# DEMO_VERSION de frontend/dist/demo.js e o topo do CHANGELOG.md. Roda no CI; a 2.28 a 2.70 saíram com o campo fixo
# do .exe parado em 2.28.0.0 porque ninguém conferia.
set -e
cd "$(dirname "$0")/.."
V=$(sed -n 's/^const AppVersion = "\(.*\)"/\1/p' internal/app/version.go)
[ -n "$V" ] || { echo "não achei a versão em internal/app/version.go"; exit 1; }
fail=0
want() { # want ARQUIVO TEXTO: o texto tem de aparecer no arquivo
  if ! grep -qF -- "$2" "$1"; then echo "ERRO: $1 não tem $2"; fail=1; fi
}
W=build/windows/winres.json
want "$W" "\"version\": \"$V.0\""
want "$W" "\"file_version\": \"$V.0\""
want "$W" "\"product_version\": \"$V.0\""
want "$W" "\"FileVersion\": \"$V\""
want "$W" "\"ProductVersion\": \"$V\""
# o modo demonstração mostra a versão atual e a seguinte (U1, 2.78.0)
want frontend/dist/demo.js "const DEMO_VERSION = \"$V\";"
TOP=$(grep -m1 '^## ' CHANGELOG.md)
if [ "$TOP" != "## $V" ]; then echo "ERRO: o topo do CHANGELOG.md é \"$TOP\", esperado \"## $V\""; fail=1; fi
[ "$fail" = 0 ] && echo "versão $V igual em version.go, winres.json, demo.js e CHANGELOG.md"
exit $fail
