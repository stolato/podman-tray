#!/bin/sh
# Installs Podman Tray for the current user, no root needed:
#   ./install.sh               install (or update) and start it
#   ./install.sh --uninstall   remove it, including the autostart entry
set -eu

here=$(cd "$(dirname "$0")" && pwd)
bin="${XDG_BIN_HOME:-$HOME/.local/bin}"
data="${XDG_DATA_HOME:-$HOME/.local/share}"
icon="$data/icons/hicolor/256x256/apps/podman-tray.png"
desktop="$data/applications/podman-tray.desktop"

if [ "${1:-}" = "--uninstall" ]; then
	pkill -x podman-tray 2>/dev/null || true
	[ -x "$bin/podman-tray" ] && "$bin/podman-tray" --unregister-login || true
	rm -f "$bin/podman-tray" "$icon" "$desktop"
	echo "Podman Tray removido."
	exit 0
fi

pkill -x podman-tray 2>/dev/null || true
mkdir -p "$bin" "$(dirname "$icon")" "$(dirname "$desktop")"
cp "$here/podman-tray" "$bin/podman-tray"
chmod 755 "$bin/podman-tray"
cp "$here/podman-tray.png" "$icon"
# Launchers don't always have ~/.local/bin in PATH: point at the binary directly.
sed "s|^Exec=.*|Exec=\"$bin/podman-tray\"|" "$here/podman-tray.desktop" >"$desktop"
command -v update-desktop-database >/dev/null && update-desktop-database "$data/applications" || true

nohup "$bin/podman-tray" >/dev/null 2>&1 &
echo "Podman Tray instalado em $bin e iniciado. Para abrir ao iniciar a sessão, use ⚙️ Opções no menu."
