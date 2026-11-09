package desktop

import (
	"encoding/base64"
	"os"
	"os/exec"
	"unicode/utf16"

	"golang.org/x/sys/windows/registry"
)

// appID is the AppUserModelID toasts are sent as. Registering it under
// HKCU\Software\Classes\AppUserModelId is how unpackaged apps get toasts shown with
// their own name.
const appID = "com.podman-tray"

const appIDKey = `Software\Classes\AppUserModelId\` + appID

func SetupNotifications() {
	k, _, err := registry.CreateKey(registry.CURRENT_USER, appIDKey, registry.SET_VALUE)
	if err != nil {
		return
	}
	defer k.Close()
	_ = k.SetStringValue("DisplayName", "Podman Tray")
}

// toastScript builds the toast from environment variables, so title and body need no
// escaping. Windows PowerShell 5.1 (always installed) can load the WinRT types.
const toastScript = `
[Windows.UI.Notifications.ToastNotificationManager, Windows.UI.Notifications, ContentType = WindowsRuntime] | Out-Null
[Windows.Data.Xml.Dom.XmlDocument, Windows.Data.Xml.Dom.XmlDocument, ContentType = WindowsRuntime] | Out-Null
$x = New-Object Windows.Data.Xml.Dom.XmlDocument
$x.LoadXml('<toast><visual><binding template="ToastGeneric"><text/><text/></binding></visual></toast>')
$t = $x.GetElementsByTagName('text')
$t.Item(0).AppendChild($x.CreateTextNode($env:PT_TITLE)) | Out-Null
$t.Item(1).AppendChild($x.CreateTextNode($env:PT_BODY)) | Out-Null
[Windows.UI.Notifications.ToastNotificationManager]::CreateToastNotifier($env:PT_APPID).Show([Windows.UI.Notifications.ToastNotification]::new($x))
`

// Notify shows a toast notification.
func Notify(title, msg string) {
	cmd := exec.Command("powershell.exe", "-NoProfile", "-NonInteractive", "-EncodedCommand", encodePowerShell(toastScript))
	cmd.Env = append(os.Environ(), "PT_TITLE="+title, "PT_BODY="+msg, "PT_APPID="+appID)
	hideWindow(cmd)
	_ = start(cmd)
}

// encodePowerShell is the base64 of the UTF-16LE script, as -EncodedCommand expects.
func encodePowerShell(script string) string {
	u := utf16.Encode([]rune(script))
	b := make([]byte, 2*len(u))
	for i, c := range u {
		b[2*i], b[2*i+1] = byte(c), byte(c>>8)
	}
	return base64.StdEncoding.EncodeToString(b)
}

// NotificationsDenied reports whether the app was turned off in Settings > System >
// Notifications.
func NotificationsDenied() bool {
	k, err := registry.OpenKey(registry.CURRENT_USER,
		`Software\Microsoft\Windows\CurrentVersion\Notifications\Settings\`+appID, registry.QUERY_VALUE)
	if err != nil {
		return false
	}
	defer k.Close()
	enabled, _, err := k.GetIntegerValue("Enabled")
	return err == nil && enabled == 0
}

func OpenNotificationSettings() error {
	return OpenURL("ms-settings:notifications")
}
