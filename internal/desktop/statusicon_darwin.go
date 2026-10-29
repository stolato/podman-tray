package desktop

/*
#include <stdlib.h>

void pt_set_badged_icon(const void *png, int len, double dot_x, double dot_y, double dot_r);
*/
import "C"

import "unsafe"

// SetBadgedIcon replaces the tray icon with mask (a PNG whose alpha is the shape),
// drawn in the menu bar's text color, plus a green dot centered at (x, y) with
// radius r. Coordinates are in points of the 16pt icon, origin at the bottom left.
// Unlike a template image, it can keep the dot colored while still following the
// menu bar's light/dark appearance.
func SetBadgedIcon(mask []byte, x, y, r float64) {
	if len(mask) == 0 {
		return
	}
	C.pt_set_badged_icon(unsafe.Pointer(&mask[0]), C.int(len(mask)), C.double(x), C.double(y), C.double(r))
}
