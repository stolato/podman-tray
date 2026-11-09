package podman

// Where the official installer puts podman.exe, for sessions whose PATH lacks it.
var candidates = []string{
	`C:\Program Files\RedHat\Podman\podman.exe`,
}

var extraPath []string
