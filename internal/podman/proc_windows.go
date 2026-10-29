package podman

import (
	"os/exec"
	"syscall"
)

// createNoWindow keeps console programs started from a GUI app from opening a window.
const createNoWindow = 0x08000000

func hideWindow(cmd *exec.Cmd) {
	cmd.SysProcAttr = &syscall.SysProcAttr{HideWindow: true, CreationFlags: createNoWindow}
}
