// Package packaging embeds the app icon, so the binary can use it at runtime (Linux
// notifications) without depending on it having been installed.
package packaging

import _ "embed"

//go:embed AppIcon.png
var AppIcon []byte
