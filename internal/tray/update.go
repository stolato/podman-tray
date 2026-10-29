package tray

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"
)

const (
	releasesURL   = "https://api.github.com/repos/stolato/podman-tray/releases/latest"
	updateDelay   = 10 * time.Second // first check, once startup settles
	updatePeriod  = 24 * time.Hour
	updateTimeout = 15 * time.Second
)

type release struct {
	Version string // without the leading "v"
	URL     string
}

// watchUpdates checks GitHub for a newer release at startup and then daily. Dev builds
// (`make run`, plain `go build`) have no version to compare, so they skip it.
func (a *App) watchUpdates() {
	if version == "dev" {
		return
	}
	wait := updateDelay
	for {
		select {
		case <-a.ctx.Done():
			return
		case <-time.After(wait):
		}
		wait = updatePeriod
		if r, err := latestRelease(a.ctx); err == nil && newerVersion(r.Version, version) {
			a.newRelease.Store(&r)
			a.triggerRefresh()
		}
	}
}

func latestRelease(ctx context.Context) (release, error) {
	ctx, cancel := context.WithTimeout(ctx, updateTimeout)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, releasesURL, nil)
	if err != nil {
		return release{}, err
	}
	req.Header.Set("Accept", "application/vnd.github+json")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return release{}, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return release{}, fmt.Errorf("GitHub: %s", resp.Status)
	}
	var r struct {
		Tag string `json:"tag_name"`
		URL string `json:"html_url"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&r); err != nil {
		return release{}, err
	}
	return release{Version: strings.TrimPrefix(r.Tag, "v"), URL: r.URL}, nil
}

// newerVersion compares dotted versions (1.10.0 > 1.9.2). Anything unparsable is not newer.
func newerVersion(latest, current string) bool {
	l, okL := parseVersion(latest)
	c, okC := parseVersion(current)
	if !okL || !okC {
		return false
	}
	for i := range max(len(l), len(c)) {
		var x, y int
		if i < len(l) {
			x = l[i]
		}
		if i < len(c) {
			y = c[i]
		}
		if x != y {
			return x > y
		}
	}
	return false
}

func parseVersion(v string) ([]int, bool) {
	var parts []int
	for _, p := range strings.Split(v, ".") {
		n, err := strconv.Atoi(p)
		if err != nil {
			return nil, false
		}
		parts = append(parts, n)
	}
	return parts, true
}
