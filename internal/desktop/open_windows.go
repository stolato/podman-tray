package desktop

import (
	"os/exec"
	"strings"
	"syscall"
)

// createNoWindow keeps console programs started from a GUI app from opening a window.
const createNoWindow = 0x08000000

func hideWindow(cmd *exec.Cmd) {
	cmd.SysProcAttr = &syscall.SysProcAttr{HideWindow: true, CreationFlags: createNoWindow}
}

// start launches a program without waiting for it, reaping it when it exits.
func start(cmd *exec.Cmd) error {
	if err := cmd.Start(); err != nil {
		return err
	}
	go func() { _ = cmd.Wait() }()
	return nil
}

func OpenURL(url string) error {
	return start(exec.Command("rundll32.exe", "url.dll,FileProtocolHandler", url))
}

// OpenInTerminal runs argv in Windows Terminal when installed, else in a console window,
// through more if pager. cmd /k keeps the window open afterwards.
func OpenInTerminal(argv []string, pager bool) error {
	quoted := make([]string, len(argv))
	for i, a := range argv {
		quoted[i] = syscall.EscapeArg(a)
	}
	line := strings.Join(quoted, " ")
	if pager {
		line += " | more"
	}

	// The command line is passed raw (CmdLine): cmd.exe has its own parsing, which
	// exec's quoting would get wrong.
	var cmd *exec.Cmd
	if wt, err := exec.LookPath("wt.exe"); err == nil {
		cmd = exec.Command(wt)
		// wt splits commands on ';'.
		cmd.SysProcAttr = &syscall.SysProcAttr{CmdLine: `wt.exe new-tab --title "Podman Tray" cmd.exe /k ` + strings.ReplaceAll(line, ";", `\;`)}
	} else {
		cmd = exec.Command("cmd.exe")
		cmd.SysProcAttr = &syscall.SysProcAttr{CmdLine: `cmd.exe /c start "Podman Tray" cmd.exe /k ` + line}
	}
	return start(cmd)
}

func CopyToClipboard(text string) error {
	cmd := exec.Command("clip.exe")
	cmd.Stdin = strings.NewReader(text)
	hideWindow(cmd)
	return cmd.Run()
}
