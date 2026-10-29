package desktop

import (
	"errors"
	"strings"

	"github.com/godbus/dbus/v5"
)

// iconName is looked up in the icon theme; packaging/linux/install.sh installs it.
const iconName = "podman-tray"

func SetupNotifications() {}

// Notify posts a notification through the freedesktop D-Bus API, which every desktop
// (GNOME, KDE, XFCE...) implements.
func Notify(title, msg string) {
	conn, err := dbus.SessionBus()
	if err != nil {
		return
	}
	// The body may be read as markup by some servers.
	body := strings.NewReplacer("&", "&amp;", "<", "&lt;", ">", "&gt;").Replace(msg)
	conn.Object("org.freedesktop.Notifications", "/org/freedesktop/Notifications").Go(
		"org.freedesktop.Notifications.Notify", 0, nil,
		"Podman Tray", uint32(0), iconName, title, body, []string{}, map[string]dbus.Variant{}, int32(-1),
	)
}

// NotificationsDenied is always false: there's no portable way to ask.
func NotificationsDenied() bool { return false }

func OpenNotificationSettings() error {
	return errors.New("abra as configurações de notificação do seu ambiente gráfico")
}
