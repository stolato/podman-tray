package podman

import (
	"fmt"
	"net"
	"strconv"
	"strings"
	"time"
)

type Port struct {
	HostIP        string `json:"host_ip"`
	ContainerPort int    `json:"container_port"`
	HostPort      int    `json:"host_port"`
	Range         int    `json:"range"`
	Protocol      string `json:"protocol"`
}

// Host is the address to reach the port from the Mac; wildcard binds map to localhost.
func (p Port) Host() string {
	switch p.HostIP {
	case "", "0.0.0.0", "::":
		return "localhost"
	}
	return p.HostIP
}

func (p Port) Label() string {
	host, cont := portRange(p.HostPort, p.Range), portRange(p.ContainerPort, p.Range)
	return fmt.Sprintf("%s:%s → %s/%s", p.Host(), host, cont, p.Protocol)
}

// URL is the http address of the (first) host port, or "" for non-tcp ports.
func (p Port) URL() string {
	if p.Protocol != "tcp" || p.HostPort == 0 {
		return ""
	}
	return "http://" + net.JoinHostPort(p.Host(), fmt.Sprint(p.HostPort))
}

func portRange(start, n int) string {
	if n > 1 {
		return fmt.Sprintf("%d-%d", start, start+n-1)
	}
	return fmt.Sprint(start)
}

type Container struct {
	ID      string            `json:"Id"`
	Names   []string          `json:"Names"`
	Image   string            `json:"Image"`
	State   string            `json:"State"`  // running, exited, created, paused...
	Status  string            `json:"Status"` // human readable, e.g. "Up 4 hours"
	Ports   []Port            `json:"Ports"`
	Labels  map[string]string `json:"Labels"`
	PodName string            `json:"PodName"`
	IsInfra bool              `json:"IsInfra"` // a pod's placeholder container
}

// Group kinds.
const (
	GroupCompose = "compose"
	GroupPod     = "pod"
)

// Group is a set of containers managed together: a compose project or a pod.
// The zero value means the container isn't in one.
type Group struct {
	Kind string // GroupCompose or GroupPod
	Name string
}

// composeLabels hold the project name, set by docker compose and podman-compose.
var composeLabels = []string{"com.docker.compose.project", "io.podman.compose.project"}

// Group is the compose project the container belongs to, or else its pod. podman-compose
// also puts a project's containers in a pod (pod_<project>), so the project wins.
func (c Container) Group() Group {
	for _, l := range composeLabels {
		if p := c.Labels[l]; p != "" {
			return Group{Kind: GroupCompose, Name: p}
		}
	}
	if c.PodName != "" {
		return Group{Kind: GroupPod, Name: c.PodName}
	}
	return Group{}
}

func (c Container) Name() string {
	if len(c.Names) > 0 {
		return c.Names[0]
	}
	if len(c.ID) > 12 {
		return c.ID[:12]
	}
	return c.ID
}

func (c Container) Running() bool {
	return c.State == "running"
}

// Health is the healthcheck result shown in the status ("Up 5 minutes (unhealthy)"):
// healthy, unhealthy, starting, or "" without a healthcheck.
func (c Container) Health() string {
	for _, h := range []string{"unhealthy", "healthy", "starting"} {
		if strings.HasSuffix(c.Status, "("+h+")") {
			return h
		}
	}
	return ""
}

// Paused containers can't be started, only unpaused (or stopped).
func (c Container) Paused() bool {
	return c.State == "paused"
}

func (c Container) PortList() string {
	var ps []string
	for _, p := range c.Ports {
		ps = append(ps, p.Label())
	}
	return strings.Join(ps, ", ")
}

type Machine struct {
	Name     string `json:"Name"`
	Default  bool   `json:"Default"`
	Running  bool   `json:"Running"`
	Starting bool   `json:"Starting"`
	CPUs     int    `json:"CPUs"`
	Memory   string `json:"Memory"` // bytes, as a string
}

func (m Machine) Stopped() bool {
	return !m.Running && !m.Starting
}

// Info is a short description like "podman-machine-default · 4 CPUs · 2 GB".
func (m Machine) Info() string {
	s := m.Name
	if m.CPUs > 0 {
		s += fmt.Sprintf(" · %d CPUs", m.CPUs)
	}
	if b, err := strconv.ParseInt(m.Memory, 10, 64); err == nil && b > 0 {
		s += fmt.Sprintf(" · %s GB", strconv.FormatFloat(float64(b)/(1<<30), 'f', -1, 64))
	}
	return s
}

// Stats is one entry of `podman stats --no-stream --format json`.
type Stats struct {
	ID         string `json:"id"`
	Name       string `json:"name"`
	CPUTime    string `json:"cpu_time"`    // e.g. "1.916333s"
	CPUPercent string `json:"cpu_percent"` // e.g. "0.31%"
	MemUsage   string `json:"mem_usage"`   // e.g. "4.821MB / 2.034GB"
}

func (s Stats) CPUDuration() (time.Duration, error) {
	return time.ParseDuration(s.CPUTime)
}

// Event is one line of `podman events --format json`.
type Event struct {
	ID       string `json:"ID"`
	Name     string `json:"Name"`
	Status   string `json:"Status"` // start, died, create, remove, health_status, sync...
	Type     string `json:"Type"`
	ExitCode int    `json:"ContainerExitCode"` // set on died
	Health   string `json:"health_status"`     // set on health_status: healthy, unhealthy, starting
}

// ContainerState is the part of `podman inspect` that explains why a container exited.
type ContainerState struct {
	ExitCode      int  `json:"ExitCode"`
	OOMKilled     bool `json:"OOMKilled"`
	StoppedByUser bool `json:"StoppedByUser"` // podman stop/restart/pod stop, from anywhere
}

// DiskUsage is one entry of `podman system df --format json`.
type DiskUsage struct {
	Type        string `json:"Type"`        // Images, Containers, Local Volumes
	Size        string `json:"Size"`        // e.g. "12.43GB"
	Reclaimable string `json:"Reclaimable"` // e.g. "10.42GB (84%)"
}

// Pod is one entry of `podman pod ls --format json`.
type Pod struct {
	Name       string `json:"Name"`
	InfraID    string `json:"InfraId"`
	Containers []struct {
		ID string `json:"Id"`
	} `json:"Containers"`
}

// Empty reports whether the pod has no containers besides its infra one.
func (p Pod) Empty() bool {
	for _, c := range p.Containers {
		if c.ID != p.InfraID {
			return false
		}
	}
	return true
}
