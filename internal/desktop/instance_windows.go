package desktop

import (
	"errors"

	"golang.org/x/sys/windows"
)

// instanceMutex is held, never closed, for the life of the process; Windows releases it
// on exit, even after a crash.
var instanceMutex windows.Handle

// AlreadyRunning claims the single-instance lock (a per-session named mutex) and reports
// whether another instance holds it.
func AlreadyRunning() bool {
	name, err := windows.UTF16PtrFromString(`Local\com.podman-tray`)
	if err != nil {
		return false
	}
	h, err := windows.CreateMutex(nil, false, name)
	if errors.Is(err, windows.ERROR_ALREADY_EXISTS) {
		_ = windows.CloseHandle(h)
		return true
	}
	instanceMutex = h
	return false
}
