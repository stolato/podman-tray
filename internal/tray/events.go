package tray

import (
	"fmt"
	"time"

	"podman-tray/internal/macos"
	"podman-tray/internal/podman"
)

const (
	// eventDebounce coalesces bursts (a restart emits died, init, start, restart...) into one refresh.
	eventDebounce = 300 * time.Millisecond
	// alertCooldown limits alerts per container, so one with a restart policy that keeps
	// crashing doesn't flood the notification center.
	alertCooldown = 5 * time.Minute
)

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
	defer close(a.eventsDone)
	backoff := time.Second
	for {
		start := time.Now()
		_ = a.pm.Events(a.ctx, func() {
			a.eventsLive.Store(true)
			a.triggerRefresh() // catch up on whatever happened while disconnected
		}, a.onEvent)
		a.eventsLive.Store(false)

		if time.Since(start) > time.Minute {
			backoff = time.Second
		}
		select {
		case <-a.ctx.Done():
			return
		case <-time.After(backoff):
		}
		backoff = min(2*backoff, maxBackoff)
	}
}

func (a *App) onEvent(e podman.Event) {
	if ignoredEvents[e.Status] {
		return
	}
	switch e.Status {
	case "died":
		if e.ExitCode != 0 {
			go a.checkCrash(e.Name, e.ExitCode)
		}
	case "health_status":
		// Sent on every check; only a change is worth a refresh.
		if !a.healthChanged(e.Name, e.Health) {
			return
		}
		if e.Health == "unhealthy" {
			a.alert(e.Name, "⚠️ "+e.Name+" está unhealthy", "O healthcheck do container está falhando.")
		}
	case "remove":
		a.healthChanged(e.Name, "")
	}
	a.debounceMu.Lock()
	defer a.debounceMu.Unlock()
	if a.debounce == nil {
		a.debounce = time.AfterFunc(eventDebounce, a.triggerRefresh)
		return
	}
	a.debounce.Reset(eventDebounce)
}

// checkCrash alerts when a container exited on its own with an error. `podman stop` also
// ends in a non-zero exit (137/143), so it asks podman whether a user stopped it.
func (a *App) checkCrash(name string, exitCode int) {
	st, err := a.pm.State(name)
	if err != nil || st.StoppedByUser { // removed meanwhile (e.g. --rm), or stopped on purpose
		return
	}
	msg := fmt.Sprintf("Saiu com código %d.", exitCode)
	if st.OOMKilled {
		msg = "Encerrado por falta de memória (OOM)."
	}
	a.alert(name, "⚠️ "+name+" parou", msg)
}

// healthChanged records a container's health and reports whether it differs from the last
// one seen. "" forgets the container.
func (a *App) healthChanged(name, health string) bool {
	a.alertMu.Lock()
	defer a.alertMu.Unlock()
	if health == "" {
		delete(a.health, name)
		return true
	}
	changed := a.health[name] != health
	a.health[name] = health
	return changed
}

// alert notifies about a problem with a container, at most once per alertCooldown each.
// Unlike action results, alerts ignore the "notify finished actions" setting.
func (a *App) alert(name, title, msg string) {
	a.alertMu.Lock()
	last, ok := a.alertedAt[name]
	if ok && time.Since(last) < alertCooldown {
		a.alertMu.Unlock()
		return
	}
	a.alertedAt[name] = time.Now()
	a.alertMu.Unlock()
	macos.Notify(title, msg)
}
