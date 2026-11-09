package tray

import (
	"errors"
	"fmt"
	"slices"
	"strings"
	"sync"

	"podman-tray/internal/i18n"

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
			desktop.Notify(i18n.F("Error: %s %s", action, name), err.Error())
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
		desktop.Notify(i18n.T("Error listing containers"), err.Error())
		return
	}
	title := "Podman Tray"
	if g != nil {
		cs = slices.DeleteFunc(cs, func(c podman.Container) bool { return c.Group() != *g })
		title = g.Name
	}
	targets := bulkTargets(cs, action)
	if len(targets) == 0 {
		desktop.Notify(title, i18n.T(bulkMessages[action].none))
		return
	}
	go a.runBulk(title, targets, action)
}

// bulkMessages are the notifications of a bulk action, by action. Whole sentences, so
// each language can word them its own way.
var bulkMessages = map[string]struct{ none, done, failed string }{
	"start":  {"No containers to start", "%d containers started", "%s: failed to start %d of %d containers"},
	"stop":   {"No containers to stop", "%d containers stopped", "%s: failed to stop %d of %d containers"},
	"remove": {"No containers to remove", "%d containers removed", "%s: failed to remove %d of %d containers"},
}

// runBulk runs the targets of a bulk action (start, stop or remove) in parallel and sends
// a single notification at the end. It reports whether all of them succeeded.
func (a *App) runBulk(title string, targets []bulkTarget, action string) bool {
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

	msgs := bulkMessages[action]
	if len(failed) == 0 {
		a.notifyDone(title, i18n.F(msgs.done, ok))
		return true
	}
	desktop.Notify(
		i18n.F(msgs.failed, title, len(failed), len(targets)),
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
		desktop.Notify(i18n.T("Error listing containers"), err.Error())
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
			desktop.Notify(i18n.F("Error: pod %s %s", action, g.Name), err.Error())
			return
		}
		a.notifyDone("Podman Tray", fmt.Sprintf("pod %s: %s ok", g.Name, action))
	}()
}

// removeContainer stops and removes a container, after confirmation.
func (a *App) removeContainer(name string) {
	if !desktop.Confirm(i18n.F("Remove container %s?", name), i18n.T("The container will be stopped and removed. Volumes are kept."), i18n.T("Remove")) {
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
		desktop.Notify(i18n.T("Error listing containers"), err.Error())
		return
	}
	members := slices.DeleteFunc(slices.Clone(cs), func(c podman.Container) bool { return c.Group() != g })

	if g.Kind == podman.GroupPod {
		msg := i18n.F("The pod and its %d containers will be stopped and removed. Volumes are kept.", len(members))
		if desktop.Confirm(i18n.F("Remove pod %s?", g.Name), msg, i18n.T("Remove")) {
			a.podAction("remove", g, func() error { return a.pm.RemovePod(g.Name) })
		}
		return
	}

	if len(members) == 0 {
		desktop.Notify(g.Name, i18n.T(bulkMessages["remove"].none))
		return
	}
	msg := i18n.F("%d containers of the project will be stopped and removed. Networks and volumes are kept; a compose up recreates everything.", len(members))
	if !desktop.Confirm(i18n.F("Remove the containers of %s?", g.Name), msg, i18n.T("Remove")) {
		return
	}
	targets := make([]bulkTarget, len(members))
	for i, c := range members {
		targets[i] = bulkTarget{c.Name(), "remove"}
	}
	pods := projectPods(cs, g)
	go func() {
		if !a.runBulk(g.Name, targets, "remove") {
			return
		}
		for _, pod := range pods {
			if err := a.pm.RemovePod(pod); err != nil {
				desktop.Notify(i18n.F("Error removing pod %s", pod), err.Error())
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
		desktop.Notify(i18n.T("Error checking the Podman machine"), err.Error())
		return "", false
	case m == nil:
		desktop.Notify("Podman Tray", i18n.T("No Podman machine found"))
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
			desktop.Notify(i18n.T("Error starting the machine"), err.Error())
		default:
			a.notifyDone("Podman Tray", i18n.T("Podman machine started"))
		}
	}()
}

func (a *App) stopMachine() {
	name, ok := a.machineName()
	if !ok {
		return
	}
	if !desktop.Confirm(i18n.T("Stop the Podman machine?"), i18n.T("All running containers will be stopped."), i18n.T("Stop")) {
		return
	}
	go func() {
		ran, err := a.runMachineAction("stop", func() error { return a.pm.StopMachine(name) })
		switch {
		case !ran:
		case err != nil:
			desktop.Notify(i18n.T("Error stopping the machine"), err.Error())
		default:
			a.notifyDone("Podman Tray", i18n.T("Podman machine stopped"))
		}
	}()
}

func (a *App) openNotificationSettings() {
	if err := desktop.OpenNotificationSettings(); err != nil {
		desktop.Notify(i18n.T("Error opening the settings"), err.Error())
	}
}

func (a *App) openURL(url string) {
	if err := desktop.OpenURL(url); err != nil {
		desktop.Notify(i18n.F("Error opening %s", url), err.Error())
	}
}

// terminal runs podman with args in a new terminal window, through a pager if pager.
func (a *App) terminal(pager bool, args ...string) {
	if err := desktop.OpenInTerminal(append([]string{a.pm.Bin()}, args...), pager); err != nil {
		desktop.Notify(i18n.T("Error opening a terminal"), err.Error())
	}
}

func (a *App) toggleLogin(item *systray.MenuItem) {
	err := desktop.SetLoginItem(!item.Checked())
	switch {
	case errors.Is(err, desktop.ErrLoginItemNeedsApproval):
		desktop.Notify(i18n.T("Open at login"), err.Error())
	case err != nil:
		desktop.Notify(i18n.T("Error"), err.Error())
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
		desktop.Notify(i18n.T("Error saving the setting"), err.Error())
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
		desktop.Notify(i18n.T("Error copying"), err.Error())
	}
}

// pruneImages removes every image no container uses, after confirmation.
func (a *App) pruneImages() {
	if !desktop.Confirm(i18n.T("Remove unused images?"),
		i18n.T("Every image no container uses will be deleted. They are downloaded again when needed."),
		i18n.T("Remove")) {
		return
	}
	go func() {
		n, err := a.pm.PruneImages()
		a.diskStale.Store(true)
		a.triggerRefresh()
		if err != nil {
			desktop.Notify(i18n.T("Error removing images"), err.Error())
			return
		}
		a.notifyDone("Podman Tray", i18n.F("%d images removed", n))
	}()
}

// removeEmptyPods removes the pods with no containers, after confirmation. It lists
// them at click time, since the menu may be up to diskInterval old.
func (a *App) removeEmptyPods() {
	pods, err := a.pm.EmptyPods()
	switch {
	case err != nil:
		desktop.Notify(i18n.T("Error listing pods"), err.Error())
		return
	case len(pods) == 0:
		desktop.Notify("Podman Tray", i18n.T("No empty pods"))
		return
	}
	if !desktop.Confirm(i18n.T("Remove empty pods?"), truncate(strings.Join(pods, ", "), 300), i18n.T("Remove")) {
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
			desktop.Notify(i18n.F("Error removing %d of %d pods", len(failed), len(pods)), truncate(failed[0], 200))
			return
		}
		a.notifyDone("Podman Tray", i18n.F("%d empty pods removed", len(pods)))
	}()
}
