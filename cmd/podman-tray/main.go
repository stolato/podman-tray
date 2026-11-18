package main

import (
	"fmt"
	"os"

	"fyne.io/systray"

	"podman-tray/internal/desktop"
	"podman-tray/internal/i18n"
	"podman-tray/internal/tray"
)

func main() {
	// Used by `make uninstall`: the SMAppService login item can only be removed by the app
	// itself, and would otherwise linger in Login Items after the app is deleted.
	if len(os.Args) > 1 && os.Args[1] == "--unregister-login" {
		if err := desktop.SetLoginItem(false); err != nil {
			fmt.Fprintln(os.Stderr, "podman-tray:", err)
			os.Exit(1)
		}
		return
	}

	// Autostart plus a launch from the app menu would otherwise put two icons in the tray.
	// Exits quietly, like macOS does for a second launch of the .app.
	if desktop.AlreadyRunning() {
		fmt.Fprintln(os.Stderr, "podman-tray:", i18n.T("already running"))
		return
	}

	app := tray.New()
	systray.Run(app.OnReady, app.OnExit)
}
