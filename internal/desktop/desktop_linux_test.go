package desktop

import (
	"bytes"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"podman-tray/packaging"
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

func TestCachedIcon(t *testing.T) {
	dir := t.TempDir()
	path := cachedIcon(dir)
	if path != filepath.Join(dir, "podman-tray", "icon.png") {
		t.Fatalf("cachedIcon() = %s", path)
	}
	// A stale copy (e.g. from an older version) is replaced.
	if err := os.WriteFile(path, []byte("old"), 0o644); err != nil {
		t.Fatal(err)
	}
	cachedIcon(dir)
	if got, _ := os.ReadFile(path); !bytes.Equal(got, packaging.AppIcon) {
		t.Error("cached icon doesn't match the embedded one")
	}
}

func TestKnownTerminal(t *testing.T) {
	if got := knownTerminal("wezterm").args; !slices.Equal(got, []string{"start", "--"}) {
		t.Errorf("wezterm args = %q", got)
	}
	if got := knownTerminal("xdg-terminal-exec").args; got != nil {
		t.Errorf("xdg-terminal-exec args = %q, want none", got)
	}
	if got := knownTerminal("my-term").args; !slices.Equal(got, []string{"-e"}) {
		t.Errorf("unknown terminal args = %q, want -e", got)
	}
}
