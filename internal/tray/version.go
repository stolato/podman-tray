package tray

// Set at build time by the Makefile (-X) from Info.plist; plain `go build` leaves "dev".
// The build timestamp only goes into the bundle (CFBundleVersion, shown in Finder's Get Info).
var version = "dev"

// VersionString is shown in the menu and the tooltip, e.g. "Podman Tray 1.2.0".
func VersionString() string {
	return "Podman Tray " + version
}
