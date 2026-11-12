#!/bin/sh
# Empacota o RadLog num AppImage (rode no Linux, nesta pasta).
# Usa o binário "radlog" que já vem pronto aqui; não precisa de Go.
# Baixa o appimagetool e o runtime oficiais (github.com/AppImage), em versão
# fixa e com a soma conferida. "--so-ferramentas" só baixa e confere.
set -e
cd "$(dirname "$0")"
VER=$(cat VERSION 2>/dev/null || echo dev)
ARCH=x86_64
APPDIR=RadLog.AppDir

# Ferramentas de fora, com versão fixa e SHA-256 conferido antes de rodar.
# Elas entram no AppImage que depois é assinado: nada de "continuous", que
# muda sem aviso. Para atualizar, troque a versão e a soma juntas.
TOOL_VER=1.9.1
TOOL_SHA=ed4ce84f0d9caff66f50bcca6ff6f35aae54ce8135408b3fa33abfc3cb384eb0
RT_VER=20251108
RT_SHA=2fca8b443c92510f1483a883f60061ad09b46b978b2631c807cd873a47ec260d

# fetch URL ARQUIVO SHA256: baixa (se ainda não tem) e confere a soma.
fetch() {
  if [ ! -f "$2" ] || ! echo "$3  $2" | sha256sum -c --status -; then
    curl -fL --retry 3 -o "$2" "$1"
  fi
  if ! echo "$3  $2" | sha256sum -c --status -; then
    echo "ERRO: $2 não confere com o SHA-256 esperado; não vou usar." >&2
    rm -f "$2"
    exit 1
  fi
}

fetch_tools() {
  fetch "https://github.com/AppImage/appimagetool/releases/download/$TOOL_VER/appimagetool-$ARCH.AppImage" appimagetool "$TOOL_SHA"
  chmod +x appimagetool
  fetch "https://github.com/AppImage/type2-runtime/releases/download/$RT_VER/runtime-$ARCH" appimage-runtime "$RT_SHA"
}

if [ "$1" = "--so-ferramentas" ]; then
  fetch_tools
  echo "Ferramentas conferidas."
  exit 0
fi

echo "1/3 Montando $APPDIR..."
rm -rf "$APPDIR"
mkdir -p "$APPDIR/usr/bin" "$APPDIR/usr/share/applications" "$APPDIR/usr/share/icons/hicolor/256x256/apps"
cp radlog "$APPDIR/usr/bin/radlog"
chmod +x "$APPDIR/usr/bin/radlog"
cp AppRun "$APPDIR/AppRun"
chmod +x "$APPDIR/AppRun"
cp radlog.desktop "$APPDIR/radlog.desktop"
cp radlog.desktop "$APPDIR/usr/share/applications/"
cp radlog.png "$APPDIR/radlog.png"
cp radlog.png "$APPDIR/usr/share/icons/hicolor/256x256/apps/"

echo "2/3 appimagetool..."
fetch_tools

echo "3/3 Gerando o AppImage..."
OUT="RadLog-v$VER-$ARCH.AppImage"
ARCH=$ARCH ./appimagetool --appimage-extract-and-run --runtime-file ./appimage-runtime "$APPDIR" "$OUT"
echo "Pronto: ./$OUT"
echo "Precisa da WebKitGTK 4.1 no sistema (Debian/Ubuntu/Parrot: sudo apt install libwebkit2gtk-4.1-0)."
