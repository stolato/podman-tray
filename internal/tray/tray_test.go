package tray

import (
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
