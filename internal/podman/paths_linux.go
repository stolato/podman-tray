package podman

// Desktop sessions usually have a full PATH; these cover autostart entries that don't.
var candidates = []string{
	"/usr/bin/podman",
	"/usr/local/bin/podman",
}

var extraPath = []string{"/usr/local/bin", "/usr/bin", "/bin", "/usr/sbin", "/sbin"}
