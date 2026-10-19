package macos

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

const (
	launchAgentLabel = "com.podman-tray"
	bundleID         = "com.podman-tray" // must match CFBundleIdentifier in packaging/Info.plist
)

func launchAgentPath() string {
	home, _ := os.UserHomeDir()
	return filepath.Join(home, "Library", "LaunchAgents", launchAgentLabel+".plist")
}

func LoginItemEnabled() bool {
	_, err := os.Stat(launchAgentPath())
	return err == nil
}

// CanRunAtLogin is false for `go run` binaries, which live in a temp dir that disappears.
func CanRunAtLogin() bool {
	exe, err := os.Executable()
	return err == nil && !strings.Contains(exe, "go-build")
}

// programArguments picks how launchd starts the app. Inside a .app bundle it goes through
// LaunchServices by bundle id, so moving the app doesn't break the login item.
func programArguments() ([]string, error) {
	exe, err := os.Executable()
	if err != nil {
		return nil, err
	}
	if resolved, err := filepath.EvalSymlinks(exe); err == nil {
		exe = resolved
	}
	if strings.Contains(exe, ".app/Contents/MacOS/") {
		return []string{"/usr/bin/open", "-b", bundleID}, nil
	}
	return []string{exe}, nil
}

func renderLaunchAgent() ([]byte, error) {
	args, err := programArguments()
	if err != nil {
		return nil, err
	}
	var argXML strings.Builder
	for _, a := range args {
		fmt.Fprintf(&argXML, "\t\t<string>%s</string>\n", xmlEscape(a))
	}
	return []byte(fmt.Sprintf(`<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
<plist version="1.0">
<dict>
	<key>Label</key>
	<string>%s</string>
	<key>ProgramArguments</key>
	<array>
%s	</array>
	<key>RunAtLoad</key>
	<true/>
	<key>ProcessType</key>
	<string>Interactive</string>
</dict>
</plist>
`, launchAgentLabel, argXML.String())), nil
}

func SetLoginItem(enabled bool) error {
	path := launchAgentPath()
	if !enabled {
		if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
			return err
		}
		return nil
	}

	plist, err := renderLaunchAgent()
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	return os.WriteFile(path, plist, 0o644)
}

// SyncLoginItem rewrites an existing LaunchAgent if it no longer matches the running binary
// (e.g. it was moved, or the plist was written by an older version).
func SyncLoginItem() error {
	if !LoginItemEnabled() || !CanRunAtLogin() {
		return nil
	}
	current, err := os.ReadFile(launchAgentPath())
	if err != nil {
		return err
	}
	want, err := renderLaunchAgent()
	if err != nil {
		return err
	}
	if bytes.Equal(current, want) {
		return nil
	}
	return os.WriteFile(launchAgentPath(), want, 0o644)
}

func xmlEscape(s string) string {
	return strings.NewReplacer("&", "&amp;", "<", "&lt;", ">", "&gt;").Replace(s)
}
