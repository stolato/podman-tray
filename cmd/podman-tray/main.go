package main

import (
	"fyne.io/systray"

	"podman-tray/internal/tray"
)

func main() {
	app := tray.New()
	systray.Run(app.OnReady, app.OnExit)
}
