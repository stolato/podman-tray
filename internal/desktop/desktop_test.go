package desktop

import "testing"

func TestShellCommand(t *testing.T) {
	argv := []string{"/opt/podman/bin/podman", "exec", "-it", "it's", "sh", "-c", "a && b"}
	want := `'/opt/podman/bin/podman' 'exec' '-it' 'it'\''s' 'sh' '-c' 'a && b'`
	if got := shellCommand(argv, false); got != want {
		t.Errorf("shellCommand() = %s, want %s", got, want)
	}
	if got := shellCommand([]string{"podman", "inspect", "web"}, true); got != `'podman' 'inspect' 'web' | less` {
		t.Errorf("shellCommand(pager) = %s", got)
	}
}
