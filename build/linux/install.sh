#!/bin/sh
# Instala o GMC Monitor para o seu usuário (sem AppImage): binário em
# ~/.local/bin e atalho no menu de aplicativos.
set -e
cd "$(dirname "$0")"
mkdir -p "$HOME/.local/bin" "$HOME/.local/share/applications" "$HOME/.local/share/icons/hicolor/256x256/apps"
install -m 755 gmc-monitor "$HOME/.local/bin/gmc-monitor"
sed "s|^Exec=.*|Exec=$HOME/.local/bin/gmc-monitor|" gmc-monitor.desktop > "$HOME/.local/share/applications/gmc-monitor.desktop"
install -m 644 gmc-monitor.png "$HOME/.local/share/icons/hicolor/256x256/apps/gmc-monitor.png"
echo "Instalado. Procure \"GMC Monitor\" no menu de aplicativos."

if ! ldconfig -p 2>/dev/null | grep -q libwebkit2gtk-4.1; then
  echo
  echo "Falta a WebKitGTK 4.1. Debian/Ubuntu/Parrot: sudo apt install libwebkit2gtk-4.1-0"
fi
if ! id -nG | grep -qw dialout; then
  echo
  echo "Para ler o contador sem root: sudo usermod -aG dialout $USER  (depois saia e entre de novo)"
fi
