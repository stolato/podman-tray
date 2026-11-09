package podman

// Apps launched from Finder/launchd get a minimal PATH, so look in the usual install
// locations too.
var candidates = []string{
	"/opt/homebrew/bin/podman",
	"/usr/local/bin/podman",
	"/opt/podman/bin/podman",
}

var extraPath = []string{"/opt/homebrew/bin", "/usr/local/bin", "/usr/bin", "/bin", "/usr/sbin", "/sbin"}
