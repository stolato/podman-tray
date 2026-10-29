package main

import (
	"fmt"
	"os"

	"fyne.io/systray"

	"podman-tray/internal/macos"
	"podman-tray/internal/tray"
)

func main() {
	// Used by `make uninstall`: the SMAppService login item can only be removed by the app
	// itself, and would otherwise linger in Login Items after the app is deleted.
	if len(os.Args) > 1 && os.Args[1] == "--unregister-login" {
		if err := macos.SetLoginItem(false); err != nil {
			fmt.Fprintln(os.Stderr, "podman-tray:", err)
			os.Exit(1)
		}
		return
	}

	app := tray.New()
	systray.Run(app.OnReady, app.OnExit)
}
