package tray

import (
	"fmt"
	"strings"

	"fyne.io/systray"

	"podman-tray/internal/macos"
	"podman-tray/internal/podman"
)

type machineMenu struct {
	root, info, start, stop *systray.MenuItem
}

type groupMenu struct {
	root, start, stop, remove *systray.MenuItem
}

type containerMenu struct {
	root, status, stats, image *systray.MenuItem
	ports                      []*systray.MenuItem // clickable (tcp) port links
	start, stop, restart       *systray.MenuItem
	pause, unpause             *systray.MenuItem // only one is shown, depending on the state
	logs, shell, inspect       *systray.MenuItem
	remove                     *systray.MenuItem
}

// build recreates the whole menu. Only called when the set of containers (their groups or
// ports, or the error state) changes; otherwise update() edits the existing items in place.
func (a *App) build(s snapshot) {
	if a.done != nil {
		close(a.done)
	}
	a.done = make(chan struct{})
	done := a.done

	systray.ResetMenu()
	a.machine = nil
	a.groups = map[podman.Group]*groupMenu{}
	a.containers = map[string]*containerMenu{}

	a.header = systray.AddMenuItem("", "")
	a.header.Disable()

	if s.err != nil {
		errItem := systray.AddMenuItem(truncate(s.err.Error(), 70), s.err.Error())
		errItem.Disable()
		if s.machineStopped {
			a.on(systray.AddMenuItem("▶ Iniciar Podman machine", ""), done, a.startMachine)
		}
	}

	systray.AddSeparator()

	if s.err == nil && len(s.containers) == 0 {
		systray.AddMenuItem("Nenhum container encontrado", "").Disable()
	}
	groups, loose := groupContainers(s.containers)
	for _, g := range groups {
		a.groups[g.Group] = a.buildGroup(g, done)
	}
	for _, c := range loose {
		a.containers[c.Name()] = a.buildContainer(c, nil, done)
	}

	if a.pm != nil {
		systray.AddSeparator()
		a.on(systray.AddMenuItem("▶ Iniciar todos", ""), done, func() { a.bulk("start", nil) })
		a.on(systray.AddMenuItem("■ Parar todos", ""), done, func() { a.bulk("stop", nil) })

		systray.AddSeparator()
		a.machine = &machineMenu{root: systray.AddMenuItem("Podman machine", "")}
		a.machine.info = a.machine.root.AddSubMenuItem("", "")
		a.machine.info.Disable()
		a.machine.root.AddSeparator()
		a.machine.start = a.machine.root.AddSubMenuItem("▶ Iniciar", "")
		a.machine.stop = a.machine.root.AddSubMenuItem("■ Parar", "")
		a.on(a.machine.start, done, a.startMachine)
		a.on(a.machine.stop, done, a.stopMachine)
		a.on(systray.AddMenuItem("↻ Atualizar", ""), done, a.triggerRefresh)
	}

	login := systray.AddMenuItemCheckbox("Abrir ao iniciar sessão", "", macos.LoginItemEnabled())
	if !macos.CanRunAtLogin() {
		login.SetTooltip("Disponível apenas no binário compilado (make app)")
		login.Disable()
	}
	a.on(login, done, func() { a.toggleLogin(login) })

	a.notifyOff = systray.AddMenuItem("🔕 Notificações desativadas, abrir Ajustes", "")
	a.on(a.notifyOff, done, a.openNotificationSettings)

	systray.AddSeparator()
	systray.AddMenuItem(VersionString(), "").Disable()
	systray.AddSeparator()
	a.on(systray.AddMenuItem("Sair", ""), done, systray.Quit)
}

// buildGroup adds a compose project or pod, with its containers in the submenu.
func (a *App) buildGroup(g containerGroup, done <-chan struct{}) *groupMenu {
	kind := "Projeto compose"
	if g.Kind == podman.GroupPod {
		kind = "Pod"
	}
	m := &groupMenu{root: systray.AddMenuItem(g.Name, "")}
	m.root.AddSubMenuItem(kind+": "+g.Name, "").Disable()
	m.start = m.root.AddSubMenuItem("▶ Iniciar grupo", "")
	m.stop = m.root.AddSubMenuItem("■ Parar grupo", "")
	m.root.AddSeparator()
	for _, c := range g.containers {
		a.containers[c.Name()] = a.buildContainer(c, m.root, done)
	}
	m.root.AddSeparator()
	remove := "🗑 Remover containers…"
	if g.Kind == podman.GroupPod {
		remove = "🗑 Remover pod…"
	}
	m.remove = m.root.AddSubMenuItem(remove, "")
	a.on(m.start, done, func() { a.groupAction("start", g.Group) })
	a.on(m.stop, done, func() { a.groupAction("stop", g.Group) })
	a.on(m.remove, done, func() { a.removeGroup(g.Group) })
	return m
}

// buildContainer adds a container to the top-level menu, or under parent (its group).
func (a *App) buildContainer(c podman.Container, parent *systray.MenuItem, done <-chan struct{}) *containerMenu {
	name := c.Name()
	m := &containerMenu{}
	if parent != nil {
		m.root = parent.AddSubMenuItem(name, "")
	} else {
		m.root = systray.AddMenuItem(name, "")
	}
	m.status = m.root.AddSubMenuItem("", "")
	m.status.Disable()
	m.stats = m.root.AddSubMenuItem("", "")
	m.stats.Disable()
	m.stats.Hide()
	m.image = m.root.AddSubMenuItem("Imagem: "+c.Image, "")
	m.image.Disable()
	for _, p := range c.Ports {
		item := m.root.AddSubMenuItem("🌐 "+p.Label(), p.URL())
		if url := p.URL(); url != "" {
			m.ports = append(m.ports, item)
			a.on(item, done, func() { a.openURL(url) })
		} else {
			item.Disable()
		}
	}
	m.root.AddSeparator()
	m.start = m.root.AddSubMenuItem("▶ Start", "podman start")
	m.stop = m.root.AddSubMenuItem("■ Stop", "podman stop")
	m.restart = m.root.AddSubMenuItem("↻ Restart", "podman restart")
	m.pause = m.root.AddSubMenuItem("⏸ Pause", "podman pause")
	m.unpause = m.root.AddSubMenuItem("⏯ Unpause", "podman unpause")
	m.unpause.Hide()
	m.root.AddSeparator()
	m.logs = m.root.AddSubMenuItem("Logs…", "podman logs -f no Terminal")
	m.shell = m.root.AddSubMenuItem("Shell…", "podman exec -it no Terminal")
	m.inspect = m.root.AddSubMenuItem("Inspect…", "podman inspect no Terminal")
	m.root.AddSeparator()
	m.remove = m.root.AddSubMenuItem("🗑 Remover…", "podman rm -f")

	a.on(m.start, done, func() { a.containerAction("start", name) })
	a.on(m.stop, done, func() { a.containerAction("stop", name) })
	a.on(m.restart, done, func() { a.containerAction("restart", name) })
	a.on(m.pause, done, func() { a.containerAction("pause", name) })
	a.on(m.unpause, done, func() { a.containerAction("unpause", name) })
	a.on(m.logs, done, func() {
		a.terminal(a.pm.Command("logs", "-f", "--tail", "200", name))
	})
	a.on(m.shell, done, func() {
		a.terminal(a.pm.Command("exec", "-it", name, "sh", "-c", "command -v bash >/dev/null && exec bash || exec sh"))
	})
	a.on(m.inspect, done, func() {
		a.terminal(a.pm.Command("inspect", name) + " | less")
	})
	a.on(m.remove, done, func() { a.removeContainer(name) })
	return m
}

func (a *App) update(s snapshot) {
	var cpuNext map[string]cpuSample
	if s.stats != nil {
		cpuNext = map[string]cpuSample{}
		defer func() { a.cpuPrev = cpuNext }()
	}

	a.mu.Lock()
	busy := make(map[string]string, len(a.busy))
	for k, v := range a.busy {
		busy[k] = v
	}
	machineAction := a.machineAction
	a.mu.Unlock()

	a.updateMachine(s.machine, machineAction)
	if macos.NotificationsDenied() {
		a.notifyOff.Show()
	} else {
		a.notifyOff.Hide()
	}

	running := 0
	groups := map[podman.Group]*groupState{}
	for _, c := range s.containers {
		action, isBusy := busy[c.Name()]
		if c.Running() {
			running++
		}
		if g := c.Group(); a.groups[g] != nil {
			if groups[g] == nil {
				groups[g] = &groupState{}
			}
			groups[g].add(c, isBusy)
		}

		m := a.containers[c.Name()]
		if m == nil {
			continue
		}

		icon := stateIcon(c.State)
		status := c.Status
		if isBusy {
			icon = "⏳"
			status = action + "…"
		}
		m.root.SetTitle(icon + " " + c.Name())
		m.status.SetTitle("Status: " + status)

		if st, ok := s.stats[c.Name()]; ok && c.Running() {
			m.stats.SetTitle(statsLabel(st, s.statsAt, a.cpuPrev, cpuNext))
			m.stats.Show()
		} else if !c.Running() {
			m.stats.Hide()
		}
		for _, p := range m.ports {
			setEnabled(p, c.Running())
		}

		setEnabled(m.start, !isBusy && !c.Running() && !c.Paused())
		setEnabled(m.stop, !isBusy && (c.Running() || c.Paused()))
		setEnabled(m.restart, !isBusy && c.Running())
		setEnabled(m.pause, !isBusy && c.Running())
		setEnabled(m.unpause, !isBusy)
		if c.Paused() {
			m.pause.Hide()
			m.unpause.Show()
		} else {
			m.unpause.Hide()
			m.pause.Show()
		}
		setEnabled(m.shell, c.Running())
		setEnabled(m.remove, !isBusy)
	}
	for g, m := range a.groups {
		if st := groups[g]; st != nil {
			st.apply(g, m)
		}
	}

	if s.err != nil {
		a.header.SetTitle("⚠️ Podman indisponível")
		systray.SetTitle("!")
		a.setActiveIcon(false)
		return
	}
	a.header.SetTitle(fmt.Sprintf("%d/%d containers rodando", running, len(s.containers)))
	systray.SetTitle("")
	a.setActiveIcon(running > 0)
}

// setActiveIcon adds a green dot to the pod while any container runs. Both
// variants follow the menu bar's light/dark appearance.
func (a *App) setActiveIcon(active bool) {
	if a.iconSet && a.iconActive == active {
		return
	}
	a.iconSet, a.iconActive = true, active
	if active {
		const pt = 16.0 / iconSize // systray shows the icon at 16pt
		macos.SetBadgedIcon(trayIcon(true), dotX*pt, (iconSize-dotY)*pt, dotR*pt)
	} else {
		icon := trayIcon(false)
		systray.SetTemplateIcon(icon, icon)
	}
}

func (a *App) updateMachine(m *podman.Machine, action string) {
	if a.machine == nil {
		return
	}
	state := "não encontrada"
	switch {
	case action == "start":
		state = "⏳ iniciando…"
	case action == "stop":
		state = "⏳ parando…"
	case m == nil:
	case m.Running:
		state = "🟢 rodando"
	case m.Starting:
		state = "🟡 iniciando"
	default:
		state = "⚪️ parada"
	}
	a.machine.root.SetTitle("Podman machine: " + state)

	if m != nil {
		a.machine.info.SetTitle(m.Info())
		a.machine.info.Show()
	} else {
		a.machine.info.Hide()
	}
	idle := action == "" && m != nil
	setEnabled(a.machine.start, idle && m.Stopped())
	setEnabled(a.machine.stop, idle && m.Running)
}

// on dispatches clicks on item to fn until done is closed (menu rebuilt).
func (a *App) on(item *systray.MenuItem, done <-chan struct{}, fn func()) {
	go func() {
		for {
			select {
			case <-item.ClickedCh:
				fn()
			case <-done:
				return
			}
		}
	}()
}

func stateIcon(state string) string {
	switch state {
	case "running":
		return "🟢"
	case "paused", "restarting", "stopping":
		return "🟡"
	case "dead", "unknown":
		return "🔴"
	default: // created, exited, stopped
		return "⚪️"
	}
}

func setEnabled(item *systray.MenuItem, enabled bool) {
	if enabled {
		item.Enable()
	} else {
		item.Disable()
	}
}

func truncate(s string, n int) string {
	s = strings.ReplaceAll(s, "\n", " ")
	if len([]rune(s)) <= n {
		return s
	}
	return string([]rune(s)[:n-1]) + "…"
}
