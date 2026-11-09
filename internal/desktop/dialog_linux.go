package desktop

import (
	"os/exec"
	"strings"

	"podman-tray/internal/i18n"
)

// Confirm asks through zenity (GNOME and most others) or kdialog (KDE). Without either it
// refuses: destructive actions shouldn't run unconfirmed.
func Confirm(title, msg, button string) bool {
	if zenity, err := exec.LookPath("zenity"); err == nil {
		text := strings.NewReplacer("&", "&amp;", "<", "&lt;", ">", "&gt;").Replace(msg) // Pango markup
		return exec.Command(zenity, "--question", "--title", title, "--text", text,
			"--ok-label", button, "--cancel-label", i18n.T("Cancel"), "--default-cancel").Run() == nil
	}
	if kdialog, err := exec.LookPath("kdialog"); err == nil {
		return exec.Command(kdialog, "--title", title, "--warningcontinuecancel", msg,
			"--continue-label", button, "--cancel-label", i18n.T("Cancel")).Run() == nil
	}
	Notify(title, i18n.T("Install zenity or kdialog to confirm this action."))
	return false
}
