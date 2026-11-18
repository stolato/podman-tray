package desktop

import "github.com/godbus/dbus/v5"

// busName is owned by the running instance for as long as it lives; the bus frees it
// when the process exits, even after a crash.
const busName = "io.github.stolato.PodmanTray"

// AlreadyRunning claims the single-instance lock and reports whether another instance
// holds it. Without a session bus it can't tell, and lets this one run.
func AlreadyRunning() bool {
	conn, err := dbus.SessionBus()
	if err != nil {
		return false
	}
	reply, err := conn.RequestName(busName, dbus.NameFlagDoNotQueue)
	return err == nil && reply == dbus.RequestNameReplyExists
}
