package desktop

import "golang.org/x/sys/windows"

const (
	mbOKCancel      = 0x00000001
	mbIconWarning   = 0x00000030
	mbDefButton2    = 0x00000100 // Cancel is the default, so Enter doesn't confirm by accident
	mbSetForeground = 0x00010000
	mbTopmost       = 0x00040000
	idOK            = 1
)

// Confirm shows a warning message box. Its buttons can't be renamed, so OK stands for
// button and the title carries the question.
func Confirm(title, msg, button string) bool {
	t, err1 := windows.UTF16PtrFromString(title)
	m, err2 := windows.UTF16PtrFromString(msg + "\n\nOK: " + button)
	if err1 != nil || err2 != nil {
		return false
	}
	ret, _ := windows.MessageBox(0, m, t, mbOKCancel|mbIconWarning|mbDefButton2|mbSetForeground|mbTopmost)
	return ret == idOK
}
