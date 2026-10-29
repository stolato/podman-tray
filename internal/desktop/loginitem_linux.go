package desktop

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// autostartPath is the XDG autostart entry, honored by GNOME, KDE, XFCE and others.
func autostartPath() string {
	dir, err := os.UserConfigDir() // $XDG_CONFIG_HOME or ~/.config
	if err != nil {
		return ""
	}
	return filepath.Join(dir, "autostart", "podman-tray.desktop")
}

func LoginItemEnabled() bool {
	_, err := os.Stat(autostartPath())
	return err == nil
}

func SetLoginItem(enabled bool) error {
	path := autostartPath()
	if !enabled {
		if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
			return err
		}
		return nil
	}
	entry, err := renderAutostart()
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	return os.WriteFile(path, entry, 0o644)
}

// SyncLoginItem rewrites the autostart entry if the binary moved since it was written.
func SyncLoginItem() error {
	if !LoginItemEnabled() || !CanRunAtLogin() {
		return nil
	}
	current, err := os.ReadFile(autostartPath())
	if err != nil {
		return err
	}
	want, err := renderAutostart()
	if err != nil || bytes.Equal(current, want) {
		return err
	}
	return os.WriteFile(autostartPath(), want, 0o644)
}

func renderAutostart() ([]byte, error) {
	exe, err := appPath()
	if err != nil {
		return nil, err
	}
	return []byte(fmt.Sprintf(`[Desktop Entry]
Type=Application
Name=Podman Tray
Comment=Containers do Podman na bandeja do sistema
Exec=%s
Icon=%s
Terminal=false
X-GNOME-Autostart-enabled=true
`, desktopExecQuote(exe), iconName)), nil
}

// appPath is the file to start at login: the AppImage itself when running from one
// (os.Executable would be inside its temporary mount), otherwise the binary.
func appPath() (string, error) {
	if appImage := os.Getenv("APPIMAGE"); appImage != "" {
		return appImage, nil
	}
	exe, err := os.Executable()
	if err != nil {
		return "", err
	}
	if resolved, err := filepath.EvalSymlinks(exe); err == nil {
		exe = resolved
	}
	return exe, nil
}

// desktopExecQuote quotes an argument for a .desktop Exec key (Desktop Entry spec).
func desktopExecQuote(s string) string {
	return `"` + strings.NewReplacer(`\`, `\\`, `"`, `\"`, "`", "\\`", "$", `\$`).Replace(s) + `"`
}
