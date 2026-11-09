package desktop

import (
	"errors"
	"os"
	"os/exec"
	"strings"

	"podman-tray/internal/i18n"
)

// start launches a program without waiting for it, reaping it when it exits.
func start(cmd *exec.Cmd) error {
	if err := cmd.Start(); err != nil {
		return err
	}
	go func() { _ = cmd.Wait() }()
	return nil
}

func OpenURL(url string) error {
	return start(exec.Command("xdg-open", url))
}

// terminals are tried in order after $TERMINAL, each with the flag that makes it run
// the command that follows.
var terminals = []struct {
	name string
	args []string
}{
	{"x-terminal-emulator", []string{"-e"}}, // Debian/Ubuntu's configured default
	{"gnome-terminal", []string{"--"}},
	{"konsole", []string{"-e"}},
	{"xfce4-terminal", []string{"-x"}},
	{"kitty", nil},
	{"alacritty", []string{"-e"}},
	{"wezterm", []string{"start", "--"}},
	{"foot", nil},
	{"xterm", []string{"-e"}},
}

// OpenInTerminal runs argv in a new terminal window, through less if pager. If the
// command fails, the window waits for Enter so the error can be read.
func OpenInTerminal(argv []string, pager bool) error {
	script := shellCommand(argv, pager) +
		`; s=$?; [ $s -eq 0 ] || [ $s -eq 130 ] || { echo; printf ` +
		shellQuote(i18n.T("(exited with code %s; press Enter to close)")+`\n`) + ` "$s"; read _; }`
	run := []string{"sh", "-c", script}

	if t := os.Getenv("TERMINAL"); t != "" {
		if path, err := exec.LookPath(t); err == nil {
			return start(exec.Command(path, append([]string{"-e"}, run...)...))
		}
	}
	for _, t := range terminals {
		if path, err := exec.LookPath(t.name); err == nil {
			return start(exec.Command(path, append(append([]string{}, t.args...), run...)...))
		}
	}
	return errors.New(i18n.T("no terminal found; set the TERMINAL variable"))
}

// CopyToClipboard uses wl-copy on Wayland, else xclip or xsel on X11.
func CopyToClipboard(text string) error {
	var tools [][]string
	if os.Getenv("WAYLAND_DISPLAY") != "" {
		tools = append(tools, []string{"wl-copy"})
	}
	tools = append(tools, []string{"xclip", "-selection", "clipboard"}, []string{"xsel", "--clipboard", "--input"})
	for _, t := range tools {
		if _, err := exec.LookPath(t[0]); err == nil {
			cmd := exec.Command(t[0], t[1:]...)
			cmd.Stdin = strings.NewReader(text)
			return cmd.Run()
		}
	}
	return errors.New(i18n.T("install wl-clipboard, xclip or xsel to copy"))
}
