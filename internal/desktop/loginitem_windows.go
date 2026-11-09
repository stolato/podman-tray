package desktop

import (
	"errors"
	"os"
	"path/filepath"

	"golang.org/x/sys/windows/registry"
)

// The per-user Run key starts programs at sign-in; it shows up in Task Manager's
// Startup apps, where the user can also turn it off.
const (
	runKey   = `Software\Microsoft\Windows\CurrentVersion\Run`
	runValue = "Podman Tray"
)

func LoginItemEnabled() bool {
	_, err := runCommand()
	return err == nil
}

func runCommand() (string, error) {
	k, err := registry.OpenKey(registry.CURRENT_USER, runKey, registry.QUERY_VALUE)
	if err != nil {
		return "", err
	}
	defer k.Close()
	v, _, err := k.GetStringValue(runValue)
	return v, err
}

func SetLoginItem(enabled bool) error {
	k, err := registry.OpenKey(registry.CURRENT_USER, runKey, registry.SET_VALUE)
	if err != nil {
		return err
	}
	defer k.Close()
	if !enabled {
		if err := k.DeleteValue(runValue); err != nil && !errors.Is(err, registry.ErrNotExist) {
			return err
		}
		return nil
	}
	want, err := wantRunCommand()
	if err != nil {
		return err
	}
	return k.SetStringValue(runValue, want)
}

// SyncLoginItem rewrites the Run entry if the executable moved since it was written.
func SyncLoginItem() error {
	current, err := runCommand()
	if err != nil || !CanRunAtLogin() {
		return nil
	}
	want, err := wantRunCommand()
	if err != nil || current == want {
		return err
	}
	return SetLoginItem(true)
}

func wantRunCommand() (string, error) {
	exe, err := os.Executable()
	if err != nil {
		return "", err
	}
	if resolved, err := filepath.EvalSymlinks(exe); err == nil {
		exe = resolved
	}
	return `"` + exe + `"`, nil
}
