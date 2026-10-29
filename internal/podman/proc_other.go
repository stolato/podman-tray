//go:build !windows

package podman

import "os/exec"

func hideWindow(*exec.Cmd) {}
