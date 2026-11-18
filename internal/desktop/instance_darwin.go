package desktop

// AlreadyRunning is always false on macOS: LaunchServices already brings the running
// .app forward instead of starting a second copy.
func AlreadyRunning() bool { return false }
