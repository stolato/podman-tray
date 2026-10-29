package macos

/*
#cgo CFLAGS: -x objective-c -fobjc-arc
#cgo LDFLAGS: -framework ServiceManagement
#include <stdbool.h>
#include <stdlib.h>

bool pt_login_available(void);
int pt_login_status(void);
char *pt_login_set(bool enable);
void pt_open_login_items_settings(void);
*/
import "C"

import (
	"errors"
	"unsafe"
)

// SMAppServiceStatus values.
const (
	statusEnabled          = 1
	statusRequiresApproval = 2
)

// ErrLoginItemNeedsApproval means the login item is registered but turned off in
// System Settings > General > Login Items, which SetLoginItem has opened.
var ErrLoginItemNeedsApproval = errors.New("ative o Podman Tray em Ajustes do Sistema → Geral → Itens de Início")

// useAppService reports whether the login item goes through SMAppService (macOS 13+),
// which lists the app under Login Items in System Settings. It needs an app bundle;
// plain binaries keep using a LaunchAgent.
func useAppService() bool {
	return inBundle && bool(C.pt_login_available())
}

func appServiceOn() bool {
	return C.pt_login_status() == statusEnabled
}

func setAppService(enabled bool) error {
	if msg := C.pt_login_set(C.bool(enabled)); msg != nil {
		defer C.free(unsafe.Pointer(msg))
		return errors.New(C.GoString(msg))
	}
	if enabled && C.pt_login_status() == statusRequiresApproval {
		C.pt_open_login_items_settings()
		return ErrLoginItemNeedsApproval
	}
	return nil
}
