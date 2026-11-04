#!/bin/sh
# Empacota o GMC Monitor num AppImage (rode no Linux, nesta pasta).
# Usa o binário "gmc-monitor" que já vem pronto aqui; não precisa de Go.
# Baixa o appimagetool oficial (github.com/AppImage/appimagetool) na
# primeira vez.
set -e
cd "$(dirname "$0")"
VER=$(cat VERSION 2>/dev/null || echo dev)
ARCH=x86_64
APPDIR=GMC-Monitor.AppDir

echo "1/3 Montando $APPDIR..."
rm -rf "$APPDIR"
mkdir -p "$APPDIR/usr/bin" "$APPDIR/usr/share/applications" "$APPDIR/usr/share/icons/hicolor/256x256/apps"
cp gmc-monitor "$APPDIR/usr/bin/gmc-monitor"
chmod +x "$APPDIR/usr/bin/gmc-monitor"
cp AppRun "$APPDIR/AppRun"
chmod +x "$APPDIR/AppRun"
cp gmc-monitor.desktop "$APPDIR/gmc-monitor.desktop"
cp gmc-monitor.desktop "$APPDIR/usr/share/applications/"
cp gmc-monitor.png "$APPDIR/gmc-monitor.png"
cp gmc-monitor.png "$APPDIR/usr/share/icons/hicolor/256x256/apps/"

echo "2/3 appimagetool..."
if [ ! -x ./appimagetool ]; then
  curl -L -o appimagetool "https://github.com/AppImage/appimagetool/releases/download/continuous/appimagetool-$ARCH.AppImage"
  chmod +x appimagetool
fi

echo "3/3 Gerando o AppImage..."
OUT="GMC-Monitor-v$VER-$ARCH.AppImage"
ARCH=$ARCH ./appimagetool --appimage-extract-and-run "$APPDIR" "$OUT"
echo "Pronto: ./$OUT"
echo "Precisa da WebKitGTK 4.1 no sistema (Debian/Ubuntu/Parrot: sudo apt install libwebkit2gtk-4.1-0)."
