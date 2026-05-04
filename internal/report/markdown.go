package report

import (
	"fmt"
	"strings"

	"github.com/openhoangnc/ssd-test/internal/bench"
	"github.com/openhoangnc/ssd-test/internal/format"
)

// Markdown returns a short clipboard-friendly summary.
func Markdown(r Result) string {
	var b strings.Builder
	fmt.Fprintln(&b, "**SSD Test results**")
	fmt.Fprintln(&b)
	fmt.Fprintf(&b, "- Device: %s\n", dashIfEmpty(r.Sys.DiskModel))
	fmt.Fprintf(&b, "- Disk: %s total · %s free\n",
		format.Bytes(r.Sys.DiskSizeBytes), format.Bytes(r.Sys.DiskFreeBytes))
	fmt.Fprintf(&b, "- System: %s · %d cores · %s RAM\n",
		dashIfEmpty(r.Sys.CPUModel),
		r.Sys.CPUCores, format.Bytes(r.Sys.RAMBytes))
	fmt.Fprintf(&b, "- OS: %s/%s\n", r.Sys.OS, r.Sys.Arch)
	fmt.Fprintln(&b)
	fmt.Fprintf(&b, "- Test size: %s · written: %s · duration: %s\n",
		format.Bytes(r.Bench.FileSize), format.Bytes(r.Bench.Written),
		format.Duration(r.Bench.Duration))
	fmt.Fprintf(&b, "- Speed — min %s · avg **%s** · max %s\n",
		format.BytesPerSec(r.Bench.Min),
		format.BytesPerSec(r.Bench.Avg),
		format.BytesPerSec(r.Bench.Max))
	if cache := bench.EstimateCache(r.Bench.Samples); cache.Detected {
		fmt.Fprintf(&b, "- Cache estimate: ~%s (burst %s → steady %s)\n",
			format.Bytes(cache.Bytes),
			format.BytesPerSec(cache.BurstSpeed),
			format.BytesPerSec(cache.SteadySpeed))
	}
	if r.Bench.Cancelled {
		fmt.Fprintln(&b, "- _Cancelled before completion._")
	}
	return b.String()
}

func dashIfEmpty(s string) string {
	if s == "" {
		return "—"
	}
	return s
}
