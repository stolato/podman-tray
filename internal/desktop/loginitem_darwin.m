#import <Foundation/Foundation.h>
#import <ServiceManagement/ServiceManagement.h>
#include <stdbool.h>
#include <string.h>

bool pt_login_available(void) {
  if (@available(macOS 13.0, *)) {
    return true;
  }
  return false;
}

// Raw SMAppServiceStatus: 0 not registered, 1 enabled, 2 requires approval, 3 not found.
int pt_login_status(void) {
  if (@available(macOS 13.0, *)) {
    return (int)SMAppService.mainAppService.status;
  }
  return 0;
}

// Registers or unregisters the app as a login item. Returns NULL on success, otherwise
// an error message the caller must free.
char *pt_login_set(bool enable) {
  if (@available(macOS 13.0, *)) {
    SMAppService *svc = SMAppService.mainAppService;
    if (!enable && svc.status == SMAppServiceStatusNotRegistered) {
      return NULL; // unregistering an unknown service is an error; nothing to do
    }
    NSError *err = nil;
    BOOL ok = enable ? [svc registerAndReturnError:&err] : [svc unregisterAndReturnError:&err];
    if (!ok) {
      return strdup(err.localizedDescription.UTF8String);
    }
    return NULL;
  }
  return strdup("requer macOS 13");
}

void pt_open_login_items_settings(void) {
  if (@available(macOS 13.0, *)) {
    [SMAppService openSystemSettingsLoginItems];
  }
}
