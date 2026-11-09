package i18n

/*
#cgo LDFLAGS: -framework CoreFoundation
#include <CoreFoundation/CoreFoundation.h>
#include <stdlib.h>

// First of the user's preferred languages (System Settings > General > Language & Region),
// e.g. "pt-BR"; empty if unknown. Apps opened from Finder don't get LANG, so ask the OS.
static char *pt_preferred_language(void) {
  char *out = NULL;
  CFArrayRef langs = CFLocaleCopyPreferredLanguages();
  if (langs != NULL) {
    if (CFArrayGetCount(langs) > 0) {
      CFStringRef l = CFArrayGetValueAtIndex(langs, 0);
      char buf[64];
      if (CFStringGetCString(l, buf, sizeof buf, kCFStringEncodingUTF8)) {
        out = strdup(buf);
      }
    }
    CFRelease(langs);
  }
  return out;
}
*/
import "C"

import "unsafe"

func systemLanguage() string {
	l := C.pt_preferred_language()
	if l == nil {
		return envLanguage()
	}
	defer C.free(unsafe.Pointer(l))
	return C.GoString(l)
}
