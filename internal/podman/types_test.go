package podman

import "testing"

func TestContainerName(t *testing.T) {
	tests := []struct {
		c    Container
		want string
	}{
		{Container{Names: []string{"web"}, ID: "0123456789abcdef"}, "web"},
		{Container{ID: "0123456789abcdef"}, "0123456789ab"},
		{Container{ID: "abc"}, "abc"},
		{Container{}, ""},
	}
	for _, tt := range tests {
		if got := tt.c.Name(); got != tt.want {
			t.Errorf("Name() = %q, want %q", got, tt.want)
		}
	}
}

func TestPortList(t *testing.T) {
	c := Container{Ports: []Port{
		{HostPort: 8080, ContainerPort: 80, Protocol: "tcp"},
		{HostPort: 5353, ContainerPort: 53, Protocol: "udp"},
	}}
	if got, want := c.PortList(), "localhost:8080 → 80/tcp, localhost:5353 → 53/udp"; got != want {
		t.Errorf("PortList() = %q, want %q", got, want)
	}
}

func TestPortURL(t *testing.T) {
	tests := []struct {
		p          Port
		label, url string
	}{
		{Port{HostIP: "0.0.0.0", HostPort: 8080, ContainerPort: 80, Range: 1, Protocol: "tcp"},
			"localhost:8080 → 80/tcp", "http://localhost:8080"},
		{Port{HostIP: "127.0.0.1", HostPort: 3000, ContainerPort: 3000, Protocol: "tcp"},
			"127.0.0.1:3000 → 3000/tcp", "http://127.0.0.1:3000"},
		{Port{HostIP: "::1", HostPort: 9000, ContainerPort: 9000, Protocol: "tcp"},
			"::1:9000 → 9000/tcp", "http://[::1]:9000"},
		{Port{HostPort: 8000, ContainerPort: 80, Range: 3, Protocol: "tcp"},
			"localhost:8000-8002 → 80-82/tcp", "http://localhost:8000"},
		{Port{HostPort: 5353, ContainerPort: 53, Protocol: "udp"},
			"localhost:5353 → 53/udp", ""},
	}
	for _, tt := range tests {
		if got := tt.p.Label(); got != tt.label {
			t.Errorf("Label() = %q, want %q", got, tt.label)
		}
		if got := tt.p.URL(); got != tt.url {
			t.Errorf("URL() = %q, want %q", got, tt.url)
		}
	}
}

func TestShellQuote(t *testing.T) {
	if got, want := shellQuote("it's"), `'it'\''s'`; got != want {
		t.Errorf("shellQuote() = %q, want %q", got, want)
	}
}

func TestPickMachine(t *testing.T) {
	tests := []struct {
		ms   []Machine
		want string
	}{
		{nil, ""},
		{[]Machine{{Name: "only"}}, "only"},
		{[]Machine{{Name: "a"}, {Name: "b", Running: true}}, "b"},
		{[]Machine{{Name: "a", Running: true}, {Name: "b", Default: true}}, "b"},
	}
	for _, tt := range tests {
		got := ""
		if m := pickMachine(tt.ms); m != nil {
			got = m.Name
		}
		if got != tt.want {
			t.Errorf("pickMachine(%v) = %q, want %q", tt.ms, got, tt.want)
		}
	}
}

func TestMachineInfo(t *testing.T) {
	m := Machine{Name: "podman-machine-default", CPUs: 4, Memory: "2147483648"}
	if got, want := m.Info(), "podman-machine-default · 4 CPUs · 2 GB"; got != want {
		t.Errorf("Info() = %q, want %q", got, want)
	}
}
