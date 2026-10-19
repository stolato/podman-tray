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

// Apps launched from Finder/launchd get a minimal PATH, so look in the usual install locations too.
var candidates = []string{
	"/opt/homebrew/bin/podman",
	"/usr/local/bin/podman",
	"/opt/podman/bin/podman",
}

func New() (*Client, error) {
	bin, err := exec.LookPath("podman")
	if err != nil {
		for _, c := range candidates {
			if _, statErr := os.Stat(c); statErr == nil {
				bin = c
				break
			}
		}
	}
	if bin == "" {
		return nil, errors.New("podman não encontrado no PATH")
	}

	path := strings.Join([]string{filepath.Dir(bin), "/opt/homebrew/bin", "/usr/local/bin", "/usr/bin", "/bin", "/usr/sbin", "/sbin", os.Getenv("PATH")}, ":")
	env := append(os.Environ(), "PATH="+path)
	return &Client{bin: bin, env: env}, nil
}

func (p *Client) run(timeout time.Duration, args ...string) ([]byte, error) {
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	cmd := exec.CommandContext(ctx, p.bin, args...)
	cmd.Env = p.env
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
	var cs []Container
	if err := json.Unmarshal(out, &cs); err != nil {
		return nil, fmt.Errorf("parse ps: %w", err)
	}
	sort.Slice(cs, func(i, j int) bool { return cs[i].Name() < cs[j].Name() })
	return cs, nil
}

// ContainerAction runs start, stop or restart on a container.
func (p *Client) ContainerAction(action, name string) error {
	_, err := p.run(2*time.Minute, action, name)
	return err
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
	cmd := exec.CommandContext(ctx, p.bin, "events", "--format", "json", "--filter", "type=container")
	cmd.Env = p.env
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

// Command renders a shell command line invoking podman, for use in Terminal.
func (p *Client) Command(args ...string) string {
	parts := []string{shellQuote(p.bin)}
	for _, a := range args {
		parts = append(parts, shellQuote(a))
	}
	return strings.Join(parts, " ")
}

func shellQuote(s string) string {
	return "'" + strings.ReplaceAll(s, "'", `'\''`) + "'"
}
