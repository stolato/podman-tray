// Package podman wraps the podman CLI.
package podman

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

type Client struct {
	bin string
	env []string
}

func New() (*Client, error) {
	bin, err := exec.LookPath("podman")
	if err != nil {
		for _, c := range candidates { // per OS, in paths_*.go
			if _, statErr := os.Stat(c); statErr == nil {
				bin = c
				break
			}
		}
	}
	if bin == "" {
		return nil, errors.New("podman não encontrado no PATH")
	}

	// podman finds its helpers (e.g. gvproxy, vfkit) through PATH.
	dirs := append([]string{filepath.Dir(bin)}, extraPath...)
	path := strings.Join(append(dirs, os.Getenv("PATH")), string(os.PathListSeparator))
	env := append(os.Environ(), "PATH="+path)
	return &Client{bin: bin, env: env}, nil
}

// Bin is the podman executable, for commands run in a terminal.
func (p *Client) Bin() string {
	return p.bin
}

// command builds a podman invocation that, on Windows, doesn't flash a console window.
func (p *Client) command(ctx context.Context, args ...string) *exec.Cmd {
	cmd := exec.CommandContext(ctx, p.bin, args...)
	cmd.Env = p.env
	hideWindow(cmd)
	return cmd
}

func (p *Client) run(timeout time.Duration, args ...string) ([]byte, error) {
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	cmd := p.command(ctx, args...)
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	out, err := cmd.Output()
	if err != nil {
		msg := strings.TrimSpace(stderr.String())
		if msg == "" {
			msg = err.Error()
		}
		return out, fmt.Errorf("podman %s: %s", strings.Join(args, " "), msg)
	}
	return out, nil
}

func (p *Client) ListContainers() ([]Container, error) {
	out, err := p.run(15*time.Second, "ps", "-a", "--format", "json")
	if err != nil {
		return nil, err
	}
	var all []Container
	if err := json.Unmarshal(out, &all); err != nil {
		return nil, fmt.Errorf("parse ps: %w", err)
	}
	// Infra containers are pod plumbing: `pod start/stop` take care of them.
	cs := all[:0]
	for _, c := range all {
		if !c.IsInfra {
			cs = append(cs, c)
		}
	}
	sort.Slice(cs, func(i, j int) bool { return cs[i].Name() < cs[j].Name() })
	return cs, nil
}

// ContainerAction runs start, stop, restart, pause or unpause on a container.
func (p *Client) ContainerAction(action, name string) error {
	_, err := p.run(2*time.Minute, action, name)
	return err
}

// PodAction runs start or stop on a pod, including its infra container.
func (p *Client) PodAction(action, pod string) error {
	_, err := p.run(2*time.Minute, "pod", action, pod)
	return err
}

// RemoveContainer stops and removes a container. Volumes are kept.
func (p *Client) RemoveContainer(name string) error {
	_, err := p.run(2*time.Minute, "rm", "-f", name)
	return err
}

// RemovePod stops and removes a pod with all its containers. Volumes are kept.
func (p *Client) RemovePod(name string) error {
	_, err := p.run(2*time.Minute, "pod", "rm", "-f", name)
	return err
}

// State inspects a container, to tell why it exited.
func (p *Client) State(name string) (ContainerState, error) {
	out, err := p.run(15*time.Second, "inspect", "--type", "container", "--format", "json", name)
	if err != nil {
		return ContainerState{}, err
	}
	var cs []struct {
		State ContainerState `json:"State"`
	}
	if err := json.Unmarshal(out, &cs); err != nil {
		return ContainerState{}, fmt.Errorf("parse inspect: %w", err)
	}
	if len(cs) == 0 {
		return ContainerState{}, fmt.Errorf("podman inspect %s: no result", name)
	}
	return cs[0].State, nil
}

// DiskUsage reports the space used by images, containers and volumes.
func (p *Client) DiskUsage() ([]DiskUsage, error) {
	out, err := p.run(30*time.Second, "system", "df", "--format", "json")
	if err != nil {
		return nil, err
	}
	var du []DiskUsage
	return du, json.Unmarshal(out, &du)
}

// EmptyPods lists the pods without containers (besides their infra one), e.g. the ones
// podman-compose leaves behind.
func (p *Client) EmptyPods() ([]string, error) {
	out, err := p.run(15*time.Second, "pod", "ls", "--format", "json")
	if err != nil {
		return nil, err
	}
	var pods []Pod
	if err := json.Unmarshal(out, &pods); err != nil {
		return nil, fmt.Errorf("parse pod ls: %w", err)
	}
	var names []string
	for _, pod := range pods {
		if pod.Empty() {
			names = append(names, pod.Name)
		}
	}
	sort.Strings(names)
	return names, nil
}

// PruneImages removes every image no container uses, returning how many were removed.
func (p *Client) PruneImages() (int, error) {
	out, err := p.run(5*time.Minute, "image", "prune", "--all", "--force")
	if err != nil {
		return 0, err
	}
	return len(strings.Fields(string(out))), nil
}

// Stats returns resource usage of the running containers.
func (p *Client) Stats() ([]Stats, error) {
	out, err := p.run(15*time.Second, "stats", "--no-stream", "--format", "json")
	if err != nil {
		return nil, err
	}
	var ss []Stats
	if err := json.Unmarshal(out, &ss); err != nil {
		return nil, fmt.Errorf("parse stats: %w", err)
	}
	return ss, nil
}

// Events streams container events to onEvent until the podman process exits or ctx is done.
// started is called once the process is running.
func (p *Client) Events(ctx context.Context, started func(), onEvent func(Event)) error {
	cmd := p.command(ctx, "events", "--format", "json", "--filter", "type=container")
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return err
	}
	if err := cmd.Start(); err != nil {
		return err
	}
	started()

	sc := bufio.NewScanner(stdout)
	sc.Buffer(make([]byte, 64*1024), 1024*1024) // label-heavy images produce long lines
	for sc.Scan() {
		var e Event
		if json.Unmarshal(sc.Bytes(), &e) == nil {
			onEvent(e)
		}
	}

	if err := cmd.Wait(); err != nil {
		if msg := strings.TrimSpace(stderr.String()); msg != "" {
			return fmt.Errorf("podman events: %s", msg)
		}
		return fmt.Errorf("podman events: %w", err)
	}
	return errors.New("podman events: stream ended")
}

func (p *Client) Machines() ([]Machine, error) {
	out, err := p.run(15*time.Second, "machine", "list", "--format", "json")
	if err != nil {
		return nil, err
	}
	var ms []Machine
	return ms, json.Unmarshal(out, &ms)
}

// Machine returns the machine the tray controls, or nil if there is none (e.g. on Linux).
func (p *Client) Machine() (*Machine, error) {
	ms, err := p.Machines()
	if err != nil {
		return nil, err
	}
	return pickMachine(ms), nil
}

// pickMachine prefers the default machine, then a running one, then the first. "Default"
// isn't reliable: a lone machine can be listed with Default=false.
func pickMachine(ms []Machine) *Machine {
	for _, want := range []func(Machine) bool{
		func(m Machine) bool { return m.Default },
		func(m Machine) bool { return m.Running || m.Starting },
		func(Machine) bool { return true },
	} {
		for i := range ms {
			if want(ms[i]) {
				return &ms[i]
			}
		}
	}
	return nil
}

func (p *Client) StartMachine(name string) error {
	_, err := p.run(5*time.Minute, "machine", "start", name)
	return err
}

func (p *Client) StopMachine(name string) error {
	_, err := p.run(2*time.Minute, "machine", "stop", name)
	return err
}
