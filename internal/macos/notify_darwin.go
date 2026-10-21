package macos

/*
#cgo CFLAGS: -x objective-c -fobjc-arc
#cgo LDFLAGS: -framework AppKit -framework UserNotifications
#include <stdbool.h>
#include <stdlib.h>

bool pt_in_bundle(void);
void pt_notifications_setup(void);
void pt_notify(const char *title, const char *body);
bool pt_notifications_denied(void);
bool pt_confirm(const char *title, const char *msg, const char *button);
*/
import "C"

import "unsafe"

// inBundle is false for `go run` binaries, which can't use UNUserNotificationCenter.
var inBundle = bool(C.pt_in_bundle())

// SetupNotifications asks for notification permission (macOS only prompts the first time)
// so alerts are sent by the app itself and show its icon. Call once at startup.
func SetupNotifications() {
	if inBundle {
		C.pt_notifications_setup()
	}
}

// Notify posts a notification as the app when allowed, otherwise through osascript.
func Notify(title, msg string) {
	if !inBundle {
		notifyScript(title, msg)
		return
	}
	ct, cm := C.CString(title), C.CString(msg)
	defer C.free(unsafe.Pointer(ct))
	defer C.free(unsafe.Pointer(cm))
	C.pt_notify(ct, cm)
}

// NotificationsDenied reports whether notifications are turned off for the app in
// System Settings, in which case Notify falls back to osascript.
func NotificationsDenied() bool {
	return inBundle && bool(C.pt_notifications_denied())
}

// OpenNotificationSettings opens the app's page in System Settings > Notifications.
func OpenNotificationSettings() error {
	return OpenURL("x-apple.systempreferences:com.apple.Notifications-Settings.extension?id=" + bundleID)
}

//export ptNotifyFallback
func ptNotifyFallback(title, msg *C.char) {
	notifyScript(C.GoString(title), C.GoString(msg))
}

// Confirm shows a native warning alert and reports whether the user clicked button.
// Cancelar is the default, so Enter doesn't trigger the action by accident.
func Confirm(title, msg, button string) bool {
	ct, cm, cb := C.CString(title), C.CString(msg), C.CString(button)
	defer C.free(unsafe.Pointer(ct))
	defer C.free(unsafe.Pointer(cm))
	defer C.free(unsafe.Pointer(cb))
	return bool(C.pt_confirm(ct, cm, cb))
}
