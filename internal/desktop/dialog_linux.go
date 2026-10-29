package desktop

import (
	"os/exec"
	"strings"
)

// Confirm asks through zenity (GNOME and most others) or kdialog (KDE). Without either it
// refuses: destructive actions shouldn't run unconfirmed.
func Confirm(title, msg, button string) bool {
	if zenity, err := exec.LookPath("zenity"); err == nil {
		text := strings.NewReplacer("&", "&amp;", "<", "&lt;", ">", "&gt;").Replace(msg) // Pango markup
		return exec.Command(zenity, "--question", "--title", title, "--text", text,
			"--ok-label", button, "--cancel-label", "Cancelar", "--default-cancel").Run() == nil
	}
	if kdialog, err := exec.LookPath("kdialog"); err == nil {
		return exec.Command(kdialog, "--title", title, "--warningcontinuecancel", msg,
			"--continue-label", button, "--cancel-label", "Cancelar").Run() == nil
	}
	Notify(title, "Instale o zenity ou o kdialog para confirmar esta ação.")
	return false
}
