package tray

import (
	"path/filepath"
	"slices"
	"testing"
	"time"

	"podman-tray/internal/podman"
)

func TestStatsLabel(t *testing.T) {
	t0 := time.Unix(1000, 0)
	st := podman.Stats{Name: "web", CPUTime: "10s", CPUPercent: "3.00%", MemUsage: "4MB / 2GB"}

	prev, next := map[string]cpuSample{}, map[string]cpuSample{}
	if got, want := statsLabel(st, t0, prev, next), "CPU 3.00% · Mem 4MB / 2GB"; got != want {
		t.Errorf("first sample = %q, want %q", got, want)
	}

	// 1s of CPU over 2s of wall time = 50%.
	st.CPUTime = "11s"
	prev, next = next, map[string]cpuSample{}
	if got, want := statsLabel(st, t0.Add(2*time.Second), prev, next), "CPU 50.0% · Mem 4MB / 2GB"; got != want {
		t.Errorf("delta sample = %q, want %q", got, want)
	}

	// A stale previous sample falls back to podman's figure.
	prev, next = next, map[string]cpuSample{}
	if got, want := statsLabel(st, t0.Add(time.Minute), prev, next), "CPU 3.00% · Mem 4MB / 2GB"; got != want {
		t.Errorf("stale sample = %q, want %q", got, want)
	}
}

func TestNextWait(t *testing.T) {
	a := &App{}

	var b time.Duration
	var waits []time.Duration
	for range 6 {
		var w time.Duration
		w, b = a.nextWait(true, b)
		waits = append(waits, w)
	}
	want := []time.Duration{5 * time.Second, 10 * time.Second, 20 * time.Second, 40 * time.Second, 60 * time.Second, 60 * time.Second}
	for i := range want {
		if waits[i] != want[i] {
			t.Fatalf("backoff = %v, want %v", waits, want)
		}
	}

	if w, b := a.nextWait(false, b); w != pollInterval || b != 0 {
		t.Errorf("no events: wait %v backoff %v", w, b)
	}
	a.eventsLive.Store(true)
	if w, _ := a.nextWait(false, 0); w != idleInterval {
		t.Errorf("events live: wait %v", w)
	}
	a.menuOpenedAt.Store(time.Now().UnixNano())
	if w, _ := a.nextWait(false, 0); w != statsInterval {
		t.Errorf("menu open: wait %v", w)
	}
}

func TestVersionString(t *testing.T) {
	defer func(v string) { version = v }(version)

	version = "1.2.3"
	if got, want := VersionString(), "Podman Tray 1.2.3"; got != want {
		t.Errorf("VersionString() = %q, want %q", got, want)
	}
}

func TestBulkTargets(t *testing.T) {
	cs := []podman.Container{
		{Names: []string{"up"}, State: "running"},
		{Names: []string{"paused"}, State: "paused"},
		{Names: []string{"exited"}, State: "exited"},
		{Names: []string{"created"}, State: "created"},
	}
	tests := map[string][]bulkTarget{
		"start": {{"paused", "unpause"}, {"exited", "start"}, {"created", "start"}},
		"stop":  {{"up", "stop"}, {"paused", "stop"}},
	}
	for action, want := range tests {
		if got := bulkTargets(cs, action); !slices.Equal(got, want) {
			t.Errorf("bulkTargets(%q) = %v, want %v", action, got, want)
		}
	}
}

func TestGroupContainers(t *testing.T) {
	compose := func(name, project string) podman.Container {
		return podman.Container{Names: []string{name}, Labels: map[string]string{"com.docker.compose.project": project}}
	}
	cs := []podman.Container{
		compose("api", "web"),
		{Names: []string{"cache"}},
		{Names: []string{"db"}, PodName: "data"},
		compose("ui", "web"),
	}
	groups, loose := groupContainers(cs)

	var got []string
	for _, g := range groups {
		for _, c := range g.containers {
			got = append(got, g.Kind+":"+g.Name+"/"+c.Name())
		}
	}
	want := []string{"pod:data/db", "compose:web/api", "compose:web/ui"}
	if !slices.Equal(got, want) {
		t.Errorf("groups = %v, want %v", got, want)
	}
	if len(loose) != 1 || loose[0].Name() != "cache" {
		t.Errorf("loose = %v, want [cache]", loose)
	}
}

func TestGroupStateIcon(t *testing.T) {
	up := podman.Container{State: "running"}
	down := podman.Container{State: "exited"}
	tests := []struct {
		cs   []podman.Container
		busy bool
		want string
	}{
		{[]podman.Container{up, up}, false, "🟢"},
		{[]podman.Container{up, down}, false, "🟡"},
		{[]podman.Container{down, down}, false, "⚪️"},
		{[]podman.Container{up, down}, true, "⏳"},
	}
	for _, tt := range tests {
		st := &groupState{}
		for i, c := range tt.cs {
			st.add(c, tt.busy && i == 0)
		}
		if got := st.icon(); got != tt.want {
			t.Errorf("icon() = %s, want %s", got, tt.want)
		}
	}
}

func TestProjectPods(t *testing.T) {
	web := podman.Group{Kind: podman.GroupCompose, Name: "web"}
	label := map[string]string{"io.podman.compose.project": "web"}
	cs := []podman.Container{
		{Names: []string{"api"}, Labels: label, PodName: "pod_web"},
		{Names: []string{"ui"}, Labels: label, PodName: "pod_web"},
		{Names: []string{"worker"}, Labels: label, PodName: "shared"},
		{Names: []string{"other"}, PodName: "shared"}, // not in the project: keep its pod
		{Names: []string{"loose"}},
	}
	if got, want := projectPods(cs, web), []string{"pod_web"}; !slices.Equal(got, want) {
		t.Errorf("projectPods() = %v, want %v", got, want)
	}
}

func TestNewerVersion(t *testing.T) {
	tests := []struct {
		latest, current string
		want            bool
	}{
		{"1.3.0", "1.2.0", true},
		{"1.10.0", "1.9.2", true},
		{"1.2.0", "1.2.0", false},
		{"1.2", "1.2.0", false},
		{"1.2.1", "1.2", true},
		{"1.1.9", "1.2.0", false},
		{"2.0.0-beta", "1.2.0", false}, // unparsable: never offered
		{"1.3.0", "dev", false},
	}
	for _, tt := range tests {
		if got := newerVersion(tt.latest, tt.current); got != tt.want {
			t.Errorf("newerVersion(%q, %q) = %t, want %t", tt.latest, tt.current, got, tt.want)
		}
	}
}

func TestSettingsRoundTrip(t *testing.T) {
	path := filepath.Join(t.TempDir(), "sub", "settings.json")
	if s := loadSettings(path); s.QuietSuccess {
		t.Error("default QuietSuccess = true, want false")
	}
	if err := saveSettings(path, settings{QuietSuccess: true}); err != nil {
		t.Fatal(err)
	}
	if s := loadSettings(path); !s.QuietSuccess {
		t.Error("QuietSuccess not persisted")
	}
}

func TestHealthChanged(t *testing.T) {
	a := &App{health: map[string]string{}}
	steps := []struct {
		health string
		want   bool
	}{
		{"healthy", true},
		{"healthy", false},
		{"unhealthy", true},
		{"unhealthy", false},
		{"", true}, // removed: forget it
		{"unhealthy", true},
	}
	for i, s := range steps {
		if got := a.healthChanged("web", s.health); got != s.want {
			t.Errorf("step %d: healthChanged(%q) = %t, want %t", i, s.health, got, s.want)
		}
	}
}

func TestContainerIcon(t *testing.T) {
	tests := []struct {
		c    podman.Container
		want string
	}{
		{podman.Container{State: "running", Status: "Up 1 minute (unhealthy)"}, "🟠"},
		{podman.Container{State: "running", Status: "Up 1 minute (healthy)"}, "🟢"},
		{podman.Container{State: "exited", Status: "Exited (1) 1 minute ago"}, "⚪️"},
	}
	for _, tt := range tests {
		if got := containerIcon(tt.c); got != tt.want {
			t.Errorf("containerIcon(%q) = %s, want %s", tt.c.Status, got, tt.want)
		}
	}
}
