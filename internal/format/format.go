// Package format provides shared human-readable formatters for sizes,
// throughput, and durations.
package format

import (
	"fmt"
	"time"
)

// Bytes formats a byte count in IEC units (KiB/MiB/GiB/TiB).
func Bytes(b int64) string {
	switch {
	case b < 1024:
		return fmt.Sprintf("%d B", b)
	case b < 1<<20:
		return fmt.Sprintf("%.2f KiB", float64(b)/1024)
	case b < 1<<30:
		return fmt.Sprintf("%.2f MiB", float64(b)/(1<<20))
	case b < 1<<40:
		return fmt.Sprintf("%.2f GiB", float64(b)/(1<<30))
	default:
		return fmt.Sprintf("%.2f TiB", float64(b)/(1<<40))
	}
}

// BytesPerSec formats a throughput value (bytes/second).
func BytesPerSec(bps float64) string {
	return Bytes(int64(bps)) + "/s"
}

// Duration formats a duration in ms / s / m depending on magnitude.
func Duration(d time.Duration) string {
	switch {
	case d < time.Second:
		return fmt.Sprintf("%d ms", d/time.Millisecond)
	case d < time.Minute:
		return fmt.Sprintf("%.2f s", d.Seconds())
	case d < time.Hour:
		return fmt.Sprintf("%.2f m", d.Minutes())
	default:
		return fmt.Sprintf("%.2f h", d.Hours())
	}
}
