// Package tray implements the menu bar app.
package tray

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"fyne.io/systray"

	"podman-tray/internal/macos"
	"podman-tray/internal/podman"
)

const (
	pollInterval  = 5 * time.Second  // when the events stream is unavailable
	idleInterval  = 60 * time.Second // safety net while events drive refreshes
	statsInterval = 2 * time.Second  // while the menu was recently opened
	statsWindow   = 30 * time.Second // how long after opening the menu stats keep refreshing
	maxBackoff    = 60 * time.Second // ceiling for retries while podman is unreachable
)

type App struct {
	pm    *podman.Client
	pmErr error

	refreshCh chan struct{}

	// Canceled on exit so the `podman events` process doesn't outlive the app.
	ctx        context.Context
	stop       context.CancelFunc
	eventsDone chan struct{} // closed when watchEvents returns; nil if it never started

	mu            sync.Mutex
	busy          map[string]string // container name -> action in progress
	machineAction string            // machine action in progress ("start"/"stop"), if any

	eventsLive   atomic.Bool
	menuOpenedAt atomic.Int64 // unix nanos

	debounceMu sync.Mutex
	debounce   *time.Timer

	alertMu   sync.Mutex
	health    map[string]string    // container name -> last health_status seen
	alertedAt map[string]time.Time // container name -> last crash/unhealthy alert

	settingsPath string
	quiet        atomic.Bool             // settings.QuietSuccess
	newRelease   atomic.Pointer[release] // newer release, once found
	diskStale    atomic.Bool             // refetch disk usage on the next refresh

	// Owned by the refresh loop goroutine.
	layoutKey  string
	done       chan struct{}
	header     *systray.MenuItem
	notifyOff  *systray.MenuItem // shown while notifications are denied in System Settings
	iconSet    bool
	iconActive bool
	machine    *machineMenu
	groups     map[podman.Group]*groupMenu
	containers map[string]*containerMenu
	cpuPrev    map[string]cpuSample
	updateItem *systray.MenuItem
	cleanup    *cleanupMenu
	disk       []podman.DiskUsage // last fetched; refreshed every diskInterval while the menu is open
	emptyPods  []string
	diskAt     time.Time
}

func New() *App {
	pm, err := podman.New()
	ctx, stop := context.WithCancel(context.Background())
	a := &App{
		pm:           pm,
		pmErr:        err,
		refreshCh:    make(chan struct{}, 1),
		ctx:          ctx,
		stop:         stop,
		busy:         map[string]string{},
		health:       map[string]string{},
		alertedAt:    map[string]time.Time{},
		cpuPrev:      map[string]cpuSample{},
		settingsPath: settingsPath(),
	}
	a.quiet.Store(loadSettings(a.settingsPath).QuietSuccess)
	return a
}

func (a *App) OnReady() {
	a.setActiveIcon(false)
	systray.SetTooltip(VersionString())
	macos.SetupNotifications()
	if err := macos.SyncLoginItem(); err != nil {
		macos.Notify("Erro ao atualizar login item", err.Error())
	}
	go a.loop()
	go a.watchMenuOpen()
	go a.watchUpdates()
	if a.pm != nil {
		a.eventsDone = make(chan struct{})
		go a.watchEvents()
	}
}

// OnExit runs before the app terminates. Canceling the context kills `podman events`;
// waiting for watchEvents makes sure that happened before the process is gone.
func (a *App) OnExit() {
	a.stop()
	if a.eventsDone == nil {
		return
	}
	select {
	case <-a.eventsDone:
	case <-time.After(2 * time.Second):
	}
}

func (a *App) triggerRefresh() {
	select {
	case a.refreshCh <- struct{}{}:
	default:
	}
}

func (a *App) loop() {
	var backoff time.Duration
	for {
		s := a.refresh()

		var wait time.Duration
		wait, backoff = a.nextWait(s.err != nil, backoff)
		timer := time.NewTimer(wait)
		select {
		case <-timer.C:
		case <-a.refreshCh:
			timer.Stop()
		}
	}
}

// nextWait picks the delay until the next refresh. Failures back off exponentially, so a
// stopped machine doesn't cost a `podman ps` + `machine list` every few seconds.
func (a *App) nextWait(failed bool, backoff time.Duration) (wait, nextBackoff time.Duration) {
	if failed {
		backoff = min(max(2*backoff, pollInterval), maxBackoff)
		return backoff, backoff
	}
	switch {
	case a.menuRecentlyOpened():
		return statsInterval, 0
	case a.eventsLive.Load():
		return idleInterval, 0
	default:
		return pollInterval, 0
	}
}

func (a *App) watchMenuOpen() {
	for range systray.TrayOpenedCh {
		a.menuOpenedAt.Store(time.Now().UnixNano())
		a.triggerRefresh()
	}
}

func (a *App) menuRecentlyOpened() bool {
	return time.Since(time.Unix(0, a.menuOpenedAt.Load())) < statsWindow
}

type snapshot struct {
	containers     []podman.Container
	err            error
	machine        *podman.Machine         // nil if there is none or it couldn't be listed
	machineStopped bool                    // podman is unreachable because the machine is down
	stats          map[string]podman.Stats // by container name; nil when not fetched
	statsAt        time.Time
}

// diskInterval is how often `system df` (slow on big stores) runs while the menu is open.
const diskInterval = time.Minute

func (a *App) fetch() snapshot {
	if a.pmErr != nil {
		return snapshot{err: a.pmErr}
	}
	cs, err := a.pm.ListContainers()
	s := snapshot{containers: cs, err: err}
	s.machine, _ = a.pm.Machine()
	if err != nil {
		s.machineStopped = s.machine != nil && s.machine.Stopped()
		return s
	}
	// Disk usage and stats are only shown in the menu, so skip them while nobody is looking.
	if a.menuRecentlyOpened() && (a.diskStale.Swap(false) || time.Since(a.diskAt) > diskInterval) {
		a.diskAt = time.Now()
		if du, err := a.pm.DiskUsage(); err == nil {
			a.disk = du
		}
		if pods, err := a.pm.EmptyPods(); err == nil {
			a.emptyPods = pods
		}
	}
	if a.menuRecentlyOpened() && anyRunning(cs) {
		if ss, err := a.pm.Stats(); err == nil {
			s.statsAt = time.Now()
			s.stats = make(map[string]podman.Stats, len(ss))
			for _, st := range ss {
				s.stats[st.Name] = st
			}
		}
	}
	return s
}

func (a *App) refresh() snapshot {
	s := a.fetch()

	keys := make([]string, len(s.containers))
	for i, c := range s.containers {
		g := c.Group()
		keys[i] = g.Kind + ":" + g.Name + "/" + c.Name() + "[" + c.PortList() + "]"
	}
	key := fmt.Sprintf("%s|%t|%t", strings.Join(keys, ","), s.err != nil, s.machineStopped)
	if key != a.layoutKey {
		a.build(s)
		a.layoutKey = key
	}
	a.update(s)
	return s
}

func anyRunning(cs []podman.Container) bool {
	for _, c := range cs {
		if c.Running() {
			return true
		}
	}
	return false
}
