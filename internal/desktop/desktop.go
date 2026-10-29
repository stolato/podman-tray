// Package desktop holds the operating system integrations: notifications, dialogs,
// terminal, browser, clipboard and the login item. Each OS has its own *_darwin.go,
// *_linux.go or *_windows.go implementation of the same functions:
//
//	SetupNotifications()                  ask for permission, once at startup
//	Notify(title, msg)                    post a notification
//	NotificationsDenied() bool            notifications are off in the OS settings
//	OpenNotificationSettings() error
//	Confirm(title, msg, button) bool      modal warning; false unless the user confirms
//	OpenURL(url) error
//	OpenInTerminal(argv, pager) error     run a command in a new terminal window
//	CopyToClipboard(text) error
//	LoginItemEnabled() bool, CanRunAtLogin() bool, SetLoginItem(bool) error, SyncLoginItem() error
package desktop

import (
	"errors"
	"os"
	"strings"
)

// ErrLoginItemNeedsApproval means the login item is registered but turned off in the
// OS settings (System Settings > General > Login Items on macOS), which SetLoginItem has
// opened.
var ErrLoginItemNeedsApproval = errors.New("ative o Podman Tray em Ajustes do Sistema → Geral → Itens de Início")

// CanRunAtLogin is false for `go run` binaries, which live in a temp dir that disappears.
func CanRunAtLogin() bool {
	exe, err := os.Executable()
	return err == nil && !strings.Contains(exe, "go-build")
}

// shellCommand renders argv as a POSIX shell command line, piping it to less if pager.
func shellCommand(argv []string, pager bool) string {
	parts := make([]string, len(argv))
	for i, a := range argv {
		parts[i] = shellQuote(a)
	}
	line := strings.Join(parts, " ")
	if pager {
		line += " | less"
	}
	return line
}

func shellQuote(s string) string {
	return "'" + strings.ReplaceAll(s, "'", `'\''`) + "'"
}
