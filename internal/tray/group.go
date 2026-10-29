package tray

import (
	"fmt"
	"sort"

	"podman-tray/internal/podman"
)

// containerGroup is a compose project or pod with its containers, in name order.
type containerGroup struct {
	podman.Group
	containers []podman.Container
}

// groupContainers splits cs (sorted by name) into groups, sorted by name, and the
// containers that aren't in any group.
func groupContainers(cs []podman.Container) (groups []containerGroup, loose []podman.Container) {
	index := map[podman.Group]int{}
	for _, c := range cs {
		g := c.Group()
		if g == (podman.Group{}) {
			loose = append(loose, c)
			continue
		}
		i, ok := index[g]
		if !ok {
			i = len(groups)
			index[g] = i
			groups = append(groups, containerGroup{Group: g})
		}
		groups[i].containers = append(groups[i].containers, c)
	}
	sort.SliceStable(groups, func(i, j int) bool { return groups[i].Name < groups[j].Name })
	return groups, loose
}

// groupState tallies a group's containers for its menu entry.
type groupState struct {
	running, total int
	busy           bool
	unhealthy      bool // some container fails its healthcheck
	startable      bool // some container isn't running (stopped or paused)
	stoppable      bool // some container is running or paused
}

func (st *groupState) add(c podman.Container, busy bool) {
	st.total++
	if c.Running() {
		st.running++
	} else {
		st.startable = true
	}
	if c.Running() || c.Paused() {
		st.stoppable = true
	}
	st.busy = st.busy || busy
	st.unhealthy = st.unhealthy || (c.Running() && c.Health() == "unhealthy")
}

// icon is 🟢 when everything runs, 🟡 when only part of it does, ⚪️ when nothing does,
// and 🟠 when something fails its healthcheck.
func (st *groupState) icon() string {
	switch {
	case st.busy:
		return "⏳"
	case st.unhealthy:
		return "🟠"
	case st.running == st.total:
		return "🟢"
	case st.running > 0:
		return "🟡"
	default:
		return "⚪️"
	}
}

func (st *groupState) apply(g podman.Group, m *groupMenu) {
	m.root.SetTitle(fmt.Sprintf("%s %s (%d/%d)", st.icon(), g.Name, st.running, st.total))
	setEnabled(m.start, !st.busy && st.startable)
	setEnabled(m.stop, !st.busy && st.stoppable)
	setEnabled(m.remove, !st.busy)
}
