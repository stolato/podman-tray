package desktop

import (
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"sync"

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

// terminal is a terminal emulator, with the flag that makes it run the command that follows.
type terminal struct {
	name string
	args []string
}

// terminals can be picked in the menu. In automatic mode they're tried in this order,
// after $TERMINAL and the launchers: desktop environments' own terminals first.
var terminals = []terminal{
	{"gnome-terminal", []string{"--"}},
	{"ptyxis", []string{"--"}},
	{"konsole", []string{"-e"}},
	{"xfce4-terminal", []string{"-x"}},
	{"ghostty", []string{"-e"}},
	{"kitty", nil},
	{"alacritty", []string{"-e"}},
	{"wezterm", []string{"start", "--"}},
	{"foot", nil},
	{"terminator", []string{"-x"}},
	{"xterm", []string{"-e"}},
}

// launchers open the user's default terminal rather than being one, so they're used by
// the automatic choice but not offered in the menu.
var launchers = []terminal{
	{"xdg-terminal-exec", nil},              // freedesktop default terminal spec (GNOME 47+, Omarchy...)
	{"x-terminal-emulator", []string{"-e"}}, // Debian/Ubuntu's configured default
}

// Terminals lists the installed terminals the user can pick, by command name.
func Terminals() []string {
	var names []string
	for _, t := range terminals {
		if _, err := exec.LookPath(t.name); err == nil {
			names = append(names, t.name)
		}
	}
	return names
}

// DefaultTerminal names the terminal the automatic choice opens, or "" if there's none.
var DefaultTerminal = sync.OnceValue(func() string {
	t, path, ok := automaticTerminal()
	if !ok {
		return ""
	}
	switch t.name {
	case "xdg-terminal-exec":
		// Prints the desktop entry it would launch, e.g. "Alacritty.desktop".
		if out, err := exec.Command(path, "--print-id").Output(); err == nil {
			if id := strings.TrimSpace(strings.SplitN(string(out), ":", 2)[0]); id != "" {
				return strings.TrimSuffix(id, ".desktop")
			}
		}
	case "x-terminal-emulator":
		if real, err := filepath.EvalSymlinks(path); err == nil {
			return strings.TrimSuffix(filepath.Base(real), ".wrapper")
		}
	}
	return t.name
})

// automaticTerminal is $TERMINAL, else the first launcher or terminal installed.
func automaticTerminal() (terminal, string, bool) {
	if env := os.Getenv("TERMINAL"); env != "" {
		if path, err := exec.LookPath(env); err == nil {
			return knownTerminal(filepath.Base(env)), path, true
		}
	}
	for _, t := range slices.Concat(launchers, terminals) {
		if path, err := exec.LookPath(t.name); err == nil {
			return t, path, true
		}
	}
	return terminal{}, "", false
}

// knownTerminal returns name with its flag, assuming -e, the most common, for unknown ones.
func knownTerminal(name string) terminal {
	for _, t := range slices.Concat(launchers, terminals) {
		if t.name == name {
			return t
		}
	}
	return terminal{name, []string{"-e"}}
}

// OpenInTerminal runs argv in a new window of the preferred terminal, or of the automatic
// choice if preferred is "" or not installed, through less if pager. If the command
// fails, the window waits for Enter so the error can be read.
func OpenInTerminal(argv []string, pager bool, preferred string) error {
	script := shellCommand(argv, pager) +
		`; s=$?; [ $s -eq 0 ] || [ $s -eq 130 ] || { echo; printf ` +
		shellQuote(i18n.T("(exited with code %s; press Enter to close)")+`\n`) + ` "$s"; read _; }`
	run := []string{"sh", "-c", script}

	t, path, ok := terminal{}, "", false
	if preferred != "" {
		if p, err := exec.LookPath(preferred); err == nil {
			t, path, ok = knownTerminal(preferred), p, true
		}
	}
	if !ok {
		t, path, ok = automaticTerminal()
	}
	if !ok {
		return errors.New(i18n.T("no terminal found; pick one in ⚙️ Options → Terminal or set the TERMINAL variable"))
	}
	return start(exec.Command(path, append(append([]string{}, t.args...), run...)...))
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
