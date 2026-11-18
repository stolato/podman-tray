package desktop

import (
	"bytes"
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/godbus/dbus/v5"

	"podman-tray/internal/i18n"
	"podman-tray/packaging"
)

// iconName is looked up in the icon theme; packaging/linux/install.sh installs it.
const iconName = "podman-tray"

func SetupNotifications() {}

// Notify posts a notification through the freedesktop D-Bus API, which every desktop
// (GNOME, KDE, XFCE...) and standalone daemon (mako, dunst, swaync...) implements. It
// waits for the server's reply, so a notification sent right before exiting isn't lost.
func Notify(title, msg string) {
	conn, err := dbus.SessionBus()
	if err != nil {
		return
	}
	// The body may be read as markup by some servers.
	body := strings.NewReplacer("&", "&amp;", "<", "&lt;", ">", "&gt;").Replace(msg)
	hints := map[string]dbus.Variant{
		// Lets the server match the notification to the app (name, icon, per-app settings).
		"desktop-entry": dbus.MakeVariant(iconName),
	}
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	conn.Object("org.freedesktop.Notifications", "/org/freedesktop/Notifications").CallWithContext(ctx,
		"org.freedesktop.Notifications.Notify", 0,
		"Podman Tray", uint32(0), notifyIcon(), title, body, []string{}, hints, int32(-1),
	)
}

// notifyIcon is the app icon as a file, written to the user cache dir from the copy
// embedded in the binary: the icon theme only has it after install.sh, and not when
// running from the tarball or `make run`. Falls back to the theme name.
var notifyIcon = sync.OnceValue(func() string {
	dir, err := os.UserCacheDir()
	if err != nil {
		return iconName
	}
	return cachedIcon(dir)
})

// cachedIcon writes the embedded icon under cacheDir, unless an identical copy is
// already there, and returns its path.
func cachedIcon(cacheDir string) string {
	path := filepath.Join(cacheDir, "podman-tray", "icon.png")
	if current, err := os.ReadFile(path); err == nil && bytes.Equal(current, packaging.AppIcon) {
		return path
	}
	if os.MkdirAll(filepath.Dir(path), 0o755) != nil || os.WriteFile(path, packaging.AppIcon, 0o644) != nil {
		return iconName
	}
	return path
}

// NotificationsDenied is always false: there's no portable way to ask.
func NotificationsDenied() bool { return false }

func OpenNotificationSettings() error {
	return errors.New(i18n.T("open the notification settings of your desktop environment"))
}
