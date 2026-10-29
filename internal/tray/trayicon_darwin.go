package tray

import (
	"fyne.io/systray"

	"podman-tray/internal/desktop"
)

// applyTrayIcon uses the black pod as a mask, so both variants follow the menu bar's
// light/dark appearance; the dot stays green.
func applyTrayIcon(active bool) {
	if active {
		const pt = 16.0 / iconSize // systray shows the icon at 16pt
		desktop.SetBadgedIcon(trayIcon(true), dotX*pt, (iconSize-dotY)*pt, dotR*pt)
		return
	}
	icon := trayIcon(false)
	systray.SetTemplateIcon(icon, icon)
}
