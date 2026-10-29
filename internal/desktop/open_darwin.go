package desktop

import (
	"fmt"
	"os/exec"
	"strings"
)

func appleScriptString(s string) string {
	s = strings.ReplaceAll(s, `\`, `\\`)
	return `"` + strings.ReplaceAll(s, `"`, `\"`) + `"`
}

// notifyScript is the fallback when the app can't post notifications itself; they show
// up attributed to Script Editor.
func notifyScript(title, msg string) {
	script := fmt.Sprintf("display notification %s with title %s", appleScriptString(msg), appleScriptString(title))
	_ = exec.Command("osascript", "-e", script).Start()
}

func OpenURL(url string) error {
	return exec.Command("open", url).Run()
}

// OpenInTerminal runs argv in a new Terminal.app window, through less if pager.
func OpenInTerminal(argv []string, pager bool) error {
	return exec.Command("osascript",
		"-e", `tell application "Terminal" to activate`,
		"-e", fmt.Sprintf(`tell application "Terminal" to do script %s`, appleScriptString(shellCommand(argv, pager))),
	).Run()
}

func CopyToClipboard(text string) error {
	cmd := exec.Command("pbcopy")
	cmd.Stdin = strings.NewReader(text)
	return cmd.Run()
}
