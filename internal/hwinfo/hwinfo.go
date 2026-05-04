// Package hwinfo collects best-effort hardware metadata for the report.
// All probes are non-fatal: missing fields render as empty strings.
package hwinfo

import (
	"context"
	"os/exec"
	"runtime"
	"strings"
	"time"

	"github.com/openhoangnc/ssd-test/internal/diskstats"
)

// SystemInfo intentionally omits hostname to avoid leaking personal info into
// shared reports. The full path being tested is also excluded from this struct
// — the bench result carries the directory it actually used.
type SystemInfo struct {
	OS            string
	Arch          string
	CPUModel      string
	CPUCores      int
	RAMBytes      int64
	DiskModel     string
	DiskSizeBytes int64
	DiskFreeBytes int64
	Timestamp     time.Time
}

// Collect probes the host and returns whatever we can determine. Path is the
// directory whose underlying storage device we want to identify.
func Collect(path string) SystemInfo {
	info := SystemInfo{
		OS:        runtime.GOOS,
		Arch:      runtime.GOARCH,
		CPUCores:  runtime.NumCPU(),
		Timestamp: time.Now(),
	}
	info.DiskSizeBytes, info.DiskFreeBytes = diskstats.Stats(path)

	platformProbe(&info, path)
	return info
}

// runCmd executes cmd with a 2s timeout and returns trimmed stdout, or empty
// string on any failure.
func runCmd(name string, args ...string) string {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	out, err := exec.CommandContext(ctx, name, args...).Output()
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(out))
}
