package tray

import (
	"context"
	"time"

	"podman-tray/internal/podman"
)

// eventDebounce coalesces bursts (a restart emits died, init, start, restart...) into one refresh.
const eventDebounce = 300 * time.Millisecond

// ignoredEvents don't change anything shown in the menu. Exec ones fire on every Shell….
var ignoredEvents = map[string]bool{
	"sync":      true,
	"exec":      true,
	"exec_died": true,
	"attach":    true,
}

// watchEvents keeps a `podman events` stream open, reconnecting with backoff when it
// drops (e.g. the machine was stopped). While it is up, the refresh loop polls slowly.
func (a *App) watchEvents() {
	backoff := time.Second
	for {
		start := time.Now()
		_ = a.pm.Events(context.Background(), func() {
			a.eventsLive.Store(true)
			a.triggerRefresh() // catch up on whatever happened while disconnected
		}, a.onEvent)
		a.eventsLive.Store(false)

		if time.Since(start) > time.Minute {
			backoff = time.Second
		}
		time.Sleep(backoff)
		backoff = min(2*backoff, maxBackoff)
	}
}

func (a *App) onEvent(e podman.Event) {
	if ignoredEvents[e.Status] {
		return
	}
	a.debounceMu.Lock()
	defer a.debounceMu.Unlock()
	if a.debounce == nil {
		a.debounce = time.AfterFunc(eventDebounce, a.triggerRefresh)
		return
	}
	a.debounce.Reset(eventDebounce)
}
