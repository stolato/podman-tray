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

type containerMenu struct {
	root, status, stats, image *systray.MenuItem
	ports                      []*systray.MenuItem // clickable (tcp) port links
	start, stop, restart       *systray.MenuItem
	logs, shell, inspect       *systray.MenuItem
}

// build recreates the whole menu. Only called when the set of containers (or error state)
// changes; otherwise update() edits the existing items in place.
func (a *App) build(s snapshot) {
	if a.done != nil {
		close(a.done)
	}
	a.done = make(chan struct{})
	done := a.done

	systray.ResetMenu()
	a.machine = nil
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
	for _, c := range s.containers {
		a.containers[c.Name()] = a.buildContainer(c, done)
	}

	if a.pm != nil {
		systray.AddSeparator()
		a.on(systray.AddMenuItem("▶ Iniciar todos", ""), done, func() { a.bulk("start") })
		a.on(systray.AddMenuItem("■ Parar todos", ""), done, func() { a.bulk("stop") })

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

	systray.AddSeparator()
	systray.AddMenuItem(VersionString(), "").Disable()
	systray.AddSeparator()
	a.on(systray.AddMenuItem("Sair", ""), done, systray.Quit)
}

func (a *App) buildContainer(c podman.Container, done <-chan struct{}) *containerMenu {
	name := c.Name()
	m := &containerMenu{root: systray.AddMenuItem(name, "")}
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
	m.root.AddSeparator()
	m.logs = m.root.AddSubMenuItem("Logs…", "podman logs -f no Terminal")
	m.shell = m.root.AddSubMenuItem("Shell…", "podman exec -it no Terminal")
	m.inspect = m.root.AddSubMenuItem("Inspect…", "podman inspect no Terminal")

	a.on(m.start, done, func() { a.containerAction("start", name) })
	a.on(m.stop, done, func() { a.containerAction("stop", name) })
	a.on(m.restart, done, func() { a.containerAction("restart", name) })
	a.on(m.logs, done, func() {
		a.terminal(a.pm.Command("logs", "-f", "--tail", "200", name))
	})
	a.on(m.shell, done, func() {
		a.terminal(a.pm.Command("exec", "-it", name, "sh", "-c", "command -v bash >/dev/null && exec bash || exec sh"))
	})
	a.on(m.inspect, done, func() {
		a.terminal(a.pm.Command("inspect", name) + " | less")
	})
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

	running := 0
	for _, c := range s.containers {
		if c.Running() {
			running++
		}
		m := a.containers[c.Name()]
		if m == nil {
			continue
		}

		action, isBusy := busy[c.Name()]
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

		setEnabled(m.start, !isBusy && !c.Running())
		setEnabled(m.stop, !isBusy && c.Running())
		setEnabled(m.restart, !isBusy)
		setEnabled(m.shell, c.Running())
	}

	if s.err != nil {
		a.header.SetTitle("⚠️ Podman indisponível")
		systray.SetTitle("!")
		return
	}
	a.header.SetTitle(fmt.Sprintf("%d/%d containers rodando", running, len(s.containers)))
	systray.SetTitle(fmt.Sprintf("%d/%d", running, len(s.containers)))
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
