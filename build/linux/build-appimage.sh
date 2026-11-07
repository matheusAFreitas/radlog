#!/bin/sh
# Empacota o RadLog num AppImage (rode no Linux, nesta pasta).
# Usa o binário "radlog" que já vem pronto aqui; não precisa de Go.
# Baixa o appimagetool oficial (github.com/AppImage/appimagetool) na
# primeira vez.
set -e
cd "$(dirname "$0")"
VER=$(cat VERSION 2>/dev/null || echo dev)
ARCH=x86_64
APPDIR=RadLog.AppDir

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
if [ ! -x ./appimagetool ]; then
  curl -L -o appimagetool "https://github.com/AppImage/appimagetool/releases/download/continuous/appimagetool-$ARCH.AppImage"
  chmod +x appimagetool
fi

echo "3/3 Gerando o AppImage..."
OUT="RadLog-v$VER-$ARCH.AppImage"
ARCH=$ARCH ./appimagetool --appimage-extract-and-run "$APPDIR" "$OUT"
echo "Pronto: ./$OUT"
echo "Precisa da WebKitGTK 4.1 no sistema (Debian/Ubuntu/Parrot: sudo apt install libwebkit2gtk-4.1-0)."
