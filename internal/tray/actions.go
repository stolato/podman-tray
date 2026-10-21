package tray

import (
	"fmt"
	"strings"
	"sync"

	"fyne.io/systray"

	"podman-tray/internal/macos"
)

func (a *App) setBusy(name, action string) bool {
	a.mu.Lock()
	defer a.mu.Unlock()
	if _, ok := a.busy[name]; ok {
		return false
	}
	a.busy[name] = action
	return true
}

func (a *App) clearBusy(name string) {
	a.mu.Lock()
	delete(a.busy, name)
	a.mu.Unlock()
}

// runAction runs action on a container, marking it busy meanwhile. ran is false, and
// nothing happens, if another action on the same container is still in progress.
func (a *App) runAction(action, name string) (ran bool, err error) {
	if !a.setBusy(name, action) {
		return false, nil
	}
	a.triggerRefresh()
	defer func() {
		a.clearBusy(name)
		a.triggerRefresh()
	}()
	return true, a.pm.ContainerAction(action, name)
}

func (a *App) containerAction(action, name string) {
	go func() {
		ran, err := a.runAction(action, name)
		switch {
		case !ran:
		case err != nil:
			macos.Notify("Erro: "+action+" "+name, err.Error())
		default:
			macos.Notify("Podman Tray", fmt.Sprintf("%s: %s ok", name, action))
		}
	}()
}

// bulk starts every stopped container, or stops every running one, and sends a single
// notification at the end. It lists containers at click time, since the menu may have
// been built long before the current states.
func (a *App) bulk(action string) {
	cs, err := a.pm.ListContainers()
	if err != nil {
		macos.Notify("Erro ao listar containers", err.Error())
		return
	}
	var names []string
	for _, c := range cs {
		if (action == "start") != c.Running() {
			names = append(names, c.Name())
		}
	}

	done, failVerb := "parados", "parar"
	if action == "start" {
		done, failVerb = "iniciados", "iniciar"
	}
	if len(names) == 0 {
		macos.Notify("Podman Tray", "Nenhum container para "+failVerb)
		return
	}

	go func() {
		var (
			wg     sync.WaitGroup
			mu     sync.Mutex
			ok     int
			failed []string
			errs   []string
		)
		for _, name := range names {
			wg.Add(1)
			go func() {
				defer wg.Done()
				ran, err := a.runAction(action, name)
				mu.Lock()
				defer mu.Unlock()
				switch {
				case err != nil:
					failed = append(failed, name)
					errs = append(errs, err.Error())
				case ran:
					ok++
				}
			}()
		}
		wg.Wait()

		if len(failed) == 0 {
			macos.Notify("Podman Tray", fmt.Sprintf("%d containers %s", ok, done))
			return
		}
		macos.Notify(
			fmt.Sprintf("Erro ao %s %d de %d containers", failVerb, len(failed), len(names)),
			truncate(strings.Join(failed, ", ")+" — "+errs[0], 200),
		)
	}()
}

// machineName looks up the machine at click time, so actions target it by name.
func (a *App) machineName() (string, bool) {
	m, err := a.pm.Machine()
	switch {
	case err != nil:
		macos.Notify("Erro ao consultar Podman machine", err.Error())
		return "", false
	case m == nil:
		macos.Notify("Podman Tray", "Nenhuma Podman machine encontrada")
		return "", false
	}
	return m.Name, true
}

// runMachineAction runs fn with the machine marked busy, unless an action is already running.
func (a *App) runMachineAction(action string, fn func() error) (ran bool, err error) {
	a.mu.Lock()
	if a.machineAction != "" {
		a.mu.Unlock()
		return false, nil
	}
	a.machineAction = action
	a.mu.Unlock()
	a.triggerRefresh()

	defer func() {
		a.mu.Lock()
		a.machineAction = ""
		a.mu.Unlock()
		a.triggerRefresh()
	}()
	return true, fn()
}

func (a *App) startMachine() {
	name, ok := a.machineName()
	if !ok {
		return
	}
	go func() {
		ran, err := a.runMachineAction("start", func() error { return a.pm.StartMachine(name) })
		switch {
		case !ran:
		case err != nil:
			macos.Notify("Erro ao iniciar machine", err.Error())
		default:
			macos.Notify("Podman Tray", "Podman machine iniciada")
		}
	}()
}

func (a *App) stopMachine() {
	name, ok := a.machineName()
	if !ok {
		return
	}
	if !macos.Confirm("Parar Podman machine", "Todos os containers em execução serão interrompidos.", "Parar") {
		return
	}
	go func() {
		ran, err := a.runMachineAction("stop", func() error { return a.pm.StopMachine(name) })
		switch {
		case !ran:
		case err != nil:
			macos.Notify("Erro ao parar machine", err.Error())
		default:
			macos.Notify("Podman Tray", "Podman machine parada")
		}
	}()
}

func (a *App) openNotificationSettings() {
	if err := macos.OpenNotificationSettings(); err != nil {
		macos.Notify("Erro ao abrir Ajustes", err.Error())
	}
}

func (a *App) openURL(url string) {
	if err := macos.OpenURL(url); err != nil {
		macos.Notify("Erro ao abrir "+url, err.Error())
	}
}

func (a *App) terminal(cmd string) {
	if err := macos.OpenInTerminal(cmd); err != nil {
		macos.Notify("Erro ao abrir Terminal", err.Error())
	}
}

func (a *App) toggleLogin(item *systray.MenuItem) {
	enable := !item.Checked()
	if err := macos.SetLoginItem(enable); err != nil {
		macos.Notify("Erro", err.Error())
		return
	}
	if enable {
		item.Check()
	} else {
		item.Uncheck()
	}
}
