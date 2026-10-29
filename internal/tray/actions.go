package tray

import (
	"errors"
	"fmt"
	"slices"
	"strings"
	"sync"

	"fyne.io/systray"

	"podman-tray/internal/desktop"
	"podman-tray/internal/podman"
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
	if action == "remove" {
		return true, a.pm.RemoveContainer(name)
	}
	return true, a.pm.ContainerAction(action, name)
}

func (a *App) containerAction(action, name string) {
	go func() {
		ran, err := a.runAction(action, name)
		switch {
		case !ran:
		case err != nil:
			desktop.Notify("Erro: "+action+" "+name, err.Error())
		default:
			a.notifyDone("Podman Tray", fmt.Sprintf("%s: %s ok", name, action))
		}
	}()
}

// bulk starts every stopped container, or stops every running one, and sends a single
// notification at the end. With g set, only the containers in that group. It lists
// containers at click time, since the menu may have been built long before the current
// states.
func (a *App) bulk(action string, g *podman.Group) {
	cs, err := a.pm.ListContainers()
	if err != nil {
		desktop.Notify("Erro ao listar containers", err.Error())
		return
	}
	title := "Podman Tray"
	if g != nil {
		cs = slices.DeleteFunc(cs, func(c podman.Container) bool { return c.Group() != *g })
		title = g.Name
	}
	targets := bulkTargets(cs, action)

	done, failVerb := "parados", "parar"
	if action == "start" {
		done, failVerb = "iniciados", "iniciar"
	}
	if len(targets) == 0 {
		desktop.Notify(title, "Nenhum container para "+failVerb)
		return
	}
	go a.runBulk(title, targets, done, failVerb)
}

// runBulk runs the targets in parallel and sends a single notification at the end.
// It reports whether all of them succeeded.
func (a *App) runBulk(title string, targets []bulkTarget, done, failVerb string) bool {
	var (
		wg     sync.WaitGroup
		mu     sync.Mutex
		ok     int
		failed []string
		errs   []string
	)
	for _, t := range targets {
		wg.Add(1)
		go func() {
			defer wg.Done()
			ran, err := a.runAction(t.action, t.name)
			mu.Lock()
			defer mu.Unlock()
			switch {
			case err != nil:
				failed = append(failed, t.name)
				errs = append(errs, err.Error())
			case ran:
				ok++
			}
		}()
	}
	wg.Wait()

	if len(failed) == 0 {
		a.notifyDone(title, fmt.Sprintf("%d containers %s", ok, done))
		return true
	}
	desktop.Notify(
		fmt.Sprintf("%s: erro ao %s %d de %d containers", title, failVerb, len(failed), len(targets)),
		truncate(strings.Join(failed, ", ")+" — "+errs[0], 200),
	)
	return false
}

// groupAction starts or stops a compose project or a pod. Pods go through `podman pod`,
// which also handles their infra container; compose projects act on each container.
func (a *App) groupAction(action string, g podman.Group) {
	if g.Kind == podman.GroupPod {
		a.podAction(action, g, func() error { return a.pm.PodAction(action, g.Name) })
		return
	}
	a.bulk(action, &g)
}

// podAction runs fn (a `podman pod` command), marking the pod's containers busy meanwhile.
func (a *App) podAction(action string, g podman.Group, fn func() error) {
	cs, err := a.pm.ListContainers()
	if err != nil {
		desktop.Notify("Erro ao listar containers", err.Error())
		return
	}
	var marked []string
	for _, c := range cs {
		if c.Group() == g && a.setBusy(c.Name(), action) {
			marked = append(marked, c.Name())
		}
	}
	go func() {
		a.triggerRefresh()
		err := fn()
		for _, name := range marked {
			a.clearBusy(name)
		}
		a.triggerRefresh()
		if err != nil {
			desktop.Notify("Erro: pod "+action+" "+g.Name, err.Error())
			return
		}
		a.notifyDone("Podman Tray", fmt.Sprintf("pod %s: %s ok", g.Name, action))
	}()
}

// removeContainer stops and removes a container, after confirmation.
func (a *App) removeContainer(name string) {
	if !desktop.Confirm("Remover container "+name+"?", "O container será parado e removido. Volumes não são apagados.", "Remover") {
		return
	}
	a.containerAction("remove", name)
}

// removeGroup removes a pod with its containers, or a compose project's containers,
// after confirmation. Without the compose file there's no `compose down`: networks and
// volumes stay, and the pod podman-compose created for the project is removed too.
func (a *App) removeGroup(g podman.Group) {
	cs, err := a.pm.ListContainers()
	if err != nil {
		desktop.Notify("Erro ao listar containers", err.Error())
		return
	}
	members := slices.DeleteFunc(slices.Clone(cs), func(c podman.Container) bool { return c.Group() != g })

	if g.Kind == podman.GroupPod {
		msg := fmt.Sprintf("O pod e seus %d containers serão parados e removidos. Volumes não são apagados.", len(members))
		if desktop.Confirm("Remover pod "+g.Name+"?", msg, "Remover") {
			a.podAction("remove", g, func() error { return a.pm.RemovePod(g.Name) })
		}
		return
	}

	if len(members) == 0 {
		desktop.Notify(g.Name, "Nenhum container para remover")
		return
	}
	msg := fmt.Sprintf("%d containers do projeto serão parados e removidos. Redes e volumes não são apagados; um compose up recria tudo.", len(members))
	if !desktop.Confirm("Remover containers de "+g.Name+"?", msg, "Remover") {
		return
	}
	targets := make([]bulkTarget, len(members))
	for i, c := range members {
		targets[i] = bulkTarget{c.Name(), "remove"}
	}
	pods := projectPods(cs, g)
	go func() {
		if !a.runBulk(g.Name, targets, "removidos", "remover") {
			return
		}
		for _, pod := range pods {
			if err := a.pm.RemovePod(pod); err != nil {
				desktop.Notify("Erro ao remover pod "+pod, err.Error())
			}
		}
	}()
}

// projectPods lists the pods podman-compose created for project g: the pods its
// containers are in, unless a container from outside the project shares them.
func projectPods(cs []podman.Container, g podman.Group) []string {
	in, out := map[string]bool{}, map[string]bool{}
	for _, c := range cs {
		switch {
		case c.PodName == "":
		case c.Group() == g:
			in[c.PodName] = true
		default:
			out[c.PodName] = true
		}
	}
	var pods []string
	for p := range in {
		if !out[p] {
			pods = append(pods, p)
		}
	}
	slices.Sort(pods)
	return pods
}

type bulkTarget struct{ name, action string }

// bulkTargets lists the containers a bulk action applies to, with the podman command for
// each: "start" starts stopped containers and unpauses paused ones; "stop" stops running
// and paused ones.
func bulkTargets(cs []podman.Container, action string) []bulkTarget {
	var ts []bulkTarget
	for _, c := range cs {
		switch {
		case action == "start" && c.Paused():
			ts = append(ts, bulkTarget{c.Name(), "unpause"})
		case action == "start" && !c.Running():
			ts = append(ts, bulkTarget{c.Name(), "start"})
		case action == "stop" && (c.Running() || c.Paused()):
			ts = append(ts, bulkTarget{c.Name(), "stop"})
		}
	}
	return ts
}

// machineName looks up the machine at click time, so actions target it by name.
func (a *App) machineName() (string, bool) {
	m, err := a.pm.Machine()
	switch {
	case err != nil:
		desktop.Notify("Erro ao consultar Podman machine", err.Error())
		return "", false
	case m == nil:
		desktop.Notify("Podman Tray", "Nenhuma Podman machine encontrada")
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
			desktop.Notify("Erro ao iniciar machine", err.Error())
		default:
			a.notifyDone("Podman Tray", "Podman machine iniciada")
		}
	}()
}

func (a *App) stopMachine() {
	name, ok := a.machineName()
	if !ok {
		return
	}
	if !desktop.Confirm("Parar Podman machine", "Todos os containers em execução serão interrompidos.", "Parar") {
		return
	}
	go func() {
		ran, err := a.runMachineAction("stop", func() error { return a.pm.StopMachine(name) })
		switch {
		case !ran:
		case err != nil:
			desktop.Notify("Erro ao parar machine", err.Error())
		default:
			a.notifyDone("Podman Tray", "Podman machine parada")
		}
	}()
}

func (a *App) openNotificationSettings() {
	if err := desktop.OpenNotificationSettings(); err != nil {
		desktop.Notify("Erro ao abrir Ajustes", err.Error())
	}
}

func (a *App) openURL(url string) {
	if err := desktop.OpenURL(url); err != nil {
		desktop.Notify("Erro ao abrir "+url, err.Error())
	}
}

// terminal runs podman with args in a new terminal window, through a pager if pager.
func (a *App) terminal(pager bool, args ...string) {
	if err := desktop.OpenInTerminal(append([]string{a.pm.Bin()}, args...), pager); err != nil {
		desktop.Notify("Erro ao abrir Terminal", err.Error())
	}
}

func (a *App) toggleLogin(item *systray.MenuItem) {
	err := desktop.SetLoginItem(!item.Checked())
	switch {
	case errors.Is(err, desktop.ErrLoginItemNeedsApproval):
		desktop.Notify("Abrir ao iniciar sessão", err.Error())
	case err != nil:
		desktop.Notify("Erro", err.Error())
	}
	if desktop.LoginItemEnabled() {
		item.Check()
	} else {
		item.Uncheck()
	}
}

// notifyDone reports an action that worked, unless the user turned those off.
func (a *App) notifyDone(title, msg string) {
	if !a.quiet.Load() {
		desktop.Notify(title, msg)
	}
}

func (a *App) toggleNotifyDone(item *systray.MenuItem) {
	quiet := item.Checked() // checked = notify; clicking flips it
	if err := saveSettings(a.settingsPath, settings{QuietSuccess: quiet}); err != nil {
		desktop.Notify("Erro ao salvar preferência", err.Error())
		return
	}
	a.quiet.Store(quiet)
	if quiet {
		item.Uncheck()
	} else {
		item.Check()
	}
}

func (a *App) copy(text string) {
	if err := desktop.CopyToClipboard(text); err != nil {
		desktop.Notify("Erro ao copiar", err.Error())
	}
}

// pruneImages removes every image no container uses, after confirmation.
func (a *App) pruneImages() {
	if !desktop.Confirm("Remover imagens sem uso?",
		"Todas as imagens que nenhum container usa serão apagadas. Elas são baixadas de novo quando precisar.",
		"Remover") {
		return
	}
	go func() {
		n, err := a.pm.PruneImages()
		a.diskStale.Store(true)
		a.triggerRefresh()
		if err != nil {
			desktop.Notify("Erro ao remover imagens", err.Error())
			return
		}
		a.notifyDone("Podman Tray", fmt.Sprintf("%d imagens removidas", n))
	}()
}

// removeEmptyPods removes the pods with no containers, after confirmation. It lists
// them at click time, since the menu may be up to diskInterval old.
func (a *App) removeEmptyPods() {
	pods, err := a.pm.EmptyPods()
	switch {
	case err != nil:
		desktop.Notify("Erro ao listar pods", err.Error())
		return
	case len(pods) == 0:
		desktop.Notify("Podman Tray", "Nenhum pod vazio")
		return
	}
	if !desktop.Confirm("Remover pods vazios?", truncate(strings.Join(pods, ", "), 300), "Remover") {
		return
	}
	go func() {
		var failed []string
		for _, pod := range pods {
			if err := a.pm.RemovePod(pod); err != nil {
				failed = append(failed, pod+": "+err.Error())
			}
		}
		a.diskStale.Store(true)
		a.triggerRefresh()
		if len(failed) > 0 {
			desktop.Notify(fmt.Sprintf("Erro ao remover %d de %d pods", len(failed), len(pods)), truncate(failed[0], 200))
			return
		}
		a.notifyDone("Podman Tray", fmt.Sprintf("%d pods vazios removidos", len(pods)))
	}()
}
