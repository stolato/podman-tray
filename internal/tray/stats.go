package tray

import (
	"fmt"
	"time"

	"podman-tray/internal/podman"
)

// cpuSampleMaxAge bounds how old the previous sample may be for a delta to count as "current".
const cpuSampleMaxAge = 10 * time.Second

type cpuSample struct {
	cpu time.Duration
	at  time.Time
}

// statsLabel formats CPU and memory usage. CPU comes from the cpu_time delta since the
// previous sample in prev; without a recent one it falls back to podman's own figure.
// The new sample is written to next.
func statsLabel(st podman.Stats, at time.Time, prev, next map[string]cpuSample) string {
	cpu := st.CPUPercent
	if d, err := st.CPUDuration(); err == nil {
		if p, ok := prev[st.Name]; ok && at.Sub(p.at) > 0 && at.Sub(p.at) < cpuSampleMaxAge && d >= p.cpu {
			cpu = fmt.Sprintf("%.1f%%", 100*float64(d-p.cpu)/float64(at.Sub(p.at)))
		}
		next[st.Name] = cpuSample{cpu: d, at: at}
	}
	return fmt.Sprintf("CPU %s · Mem %s", cpu, st.MemUsage)
}
