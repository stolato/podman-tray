package tray

// Set at build time by the Makefile (-X); plain `go build` leaves the defaults.
var (
	version = "dev"
	build   = "" // build timestamp, yyyymmddHHMM
)

// VersionString is shown in the menu, e.g. "Podman Tray 1.0.0 · build 15/10/2026 20:41".
func VersionString() string {
	s := "Podman Tray " + version
	return s
}
