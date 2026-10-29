package desktop

import (
	"strings"
	"testing"
)

func TestDesktopExecQuote(t *testing.T) {
	got := desktopExecQuote(`/home/a b/$x/"q"/podman-tray`)
	want := `"/home/a b/\$x/\"q\"/podman-tray"`
	if got != want {
		t.Errorf("desktopExecQuote() = %s, want %s", got, want)
	}
}

func TestRenderAutostartUsesAppImage(t *testing.T) {
	t.Setenv("APPIMAGE", "/opt/Podman Tray.AppImage")
	entry, err := renderAutostart()
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(entry), "\nExec=\"/opt/Podman Tray.AppImage\"\n") {
		t.Errorf("Exec line missing in:\n%s", entry)
	}
}
