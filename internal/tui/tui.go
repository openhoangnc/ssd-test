package tui

import (
	"fmt"
	"io"
	"os"
	"strings"
	"time"

	"github.com/openhoangnc/ssd-test/internal/bench"
	"github.com/openhoangnc/ssd-test/internal/format"
	"github.com/openhoangnc/ssd-test/internal/hwinfo"
)

// Screen owns the terminal state for the duration of the run.
type Screen struct {
	w        io.Writer
	restored bool
}

// Enter switches to the alternate screen buffer and hides the cursor. The
// caller must call Restore (typically via defer) to leave the terminal usable.
func Enter(w io.Writer) *Screen {
	if w == nil {
		w = os.Stdout
	}
	fmt.Fprint(w, altScreenOn, cursorHide, cursorHome)
	return &Screen{w: w}
}

// Restore returns the terminal to its prior state. Safe to call multiple times.
func (s *Screen) Restore() {
	if s.restored {
		return
	}
	s.restored = true
	fmt.Fprint(s.w, resetStyle, cursorShow, altScreenOff)
}

// IsTTY reports whether stdout is connected to a terminal.
func IsTTY() bool {
	stat, err := os.Stdout.Stat()
	if err != nil {
		return false
	}
	return (stat.Mode() & os.ModeCharDevice) != 0
}

// Phase distinguishes pre-test, running, and complete states so Render can
// adjust labels and the footer hint without the caller juggling flags.
type Phase int

const (
	PhaseConfirm Phase = iota
	PhaseRunning
	PhaseDone
)

// Frame holds everything Render needs to draw a single full-screen update.
type Frame struct {
	Phase    Phase
	Sys      hwinfo.SystemInfo
	Dir      string
	FileSize int64
	Sample   bench.Sample
	History  []float64 // BlockSpeed values, oldest → newest
	Min      float64
	Max      float64
	// PhaseDone fields:
	Result bench.Result
	Status string // ephemeral status line ("Copied to clipboard", "Saved to ...")
	Cache  bench.CacheEstimate
}

func (s *Screen) Render(f Frame) {
	cols, rows := Size()
	if cols < 40 {
		cols = 40
	}
	if rows < 12 {
		rows = 12
	}

	var b strings.Builder
	b.WriteString(cursorHome)

	// ── Header pane ──────────────────────────────────────────────────
	writeBox(&b, "SSD Test", cols, []string{
		fmt.Sprintf("%sDevice%s  %s",
			dim, resetStyle, valueOrDash(f.Sys.DiskModel)),
		fmt.Sprintf("%sStorage%s %s total · %s free · test size %s in %s",
			dim, resetStyle,
			format.Bytes(f.Sys.DiskSizeBytes),
			format.Bytes(f.Sys.DiskFreeBytes),
			format.Bytes(f.FileSize),
			f.Dir),
		fmt.Sprintf("%sSystem%s  %s · %d cores · %s RAM · %s/%s",
			dim, resetStyle,
			valueOrDash(f.Sys.CPUModel),
			f.Sys.CPUCores,
			format.Bytes(f.Sys.RAMBytes),
			f.Sys.OS, f.Sys.Arch),
	})

	// Chart pane fills remaining vertical space.
	headerH := 6
	metricsH := 5
	if f.Cache.Detected {
		metricsH = 6
	}
	footerH := 1
	chartH := rows - headerH - metricsH - footerH
	if chartH < 4 {
		chartH = 4
	}

	// ── Chart pane (or pre-test message) ────────────────────────────
	switch f.Phase {
	case PhaseConfirm:
		writeBox(&b, "", cols, confirmBody(chartH, f))
	default:
		title := "Speed vs bytes written"
		chartLines := SparklineLines(f.History, cols-2, chartH-2, 0)
		writeBox(&b, "", cols, append([]string{dim + title + resetStyle}, chartLines...))
	}

	// ── Metrics pane ────────────────────────────────────────────────
	switch f.Phase {
	case PhaseConfirm:
		writeBox(&b, "Plan", cols, []string{
			fmt.Sprintf("Will write %s%s%s of random data and remove it on completion.",
				yellow, format.Bytes(f.FileSize), resetStyle),
			"The drive's cache will saturate and you'll see the real sustained speed.",
			"",
		})
	case PhaseRunning, PhaseDone:
		writeBox(&b, "Metrics", cols, runningMetrics(f))
	}

	// ── Footer ──────────────────────────────────────────────────────
	b.WriteString(clearLine)
	b.WriteString(dim)
	b.WriteString(footerFor(f))
	b.WriteString(resetStyle)
	b.WriteString("\n")

	b.WriteString(clearBelow)
	fmt.Fprint(s.w, b.String())
}

func confirmBody(chartH int, f Frame) []string {
	lines := []string{
		"",
		fmt.Sprintf("  This test will write %s%s%s of random data to %s%s%s",
			yellow, format.Bytes(f.FileSize), resetStyle,
			cyan, f.Dir, resetStyle),
		"  and remove the file when finished. Ctrl+C cancels at any time.",
		"",
		fmt.Sprintf("  %s⚠ Heads up:%s sustained writes consume SSD endurance (TBW).", red+bold, resetStyle),
		"  A single run is fine on a healthy drive; don't loop this test or",
		"  run it on small, cheap, or already-worn SSDs you want to keep.",
		"",
	}
	for len(lines) < chartH {
		lines = append(lines, "")
	}
	return lines
}

func runningMetrics(f Frame) []string {
	pct := 0.0
	if f.FileSize > 0 {
		pct = float64(f.Sample.BytesWritten) / float64(f.FileSize) * 100
	}
	eta := "—"
	if f.Phase == PhaseRunning && f.Sample.AvgSpeed > 0 && f.Sample.BytesWritten < f.FileSize {
		remaining := f.FileSize - f.Sample.BytesWritten
		eta = format.Duration(time.Duration(float64(remaining) / f.Sample.AvgSpeed * float64(time.Second)))
	}
	if f.Phase == PhaseDone {
		eta = "done"
	}

	written := f.Sample.BytesWritten
	avg := f.Sample.AvgSpeed
	if f.Phase == PhaseDone {
		written = f.Result.Written
		avg = f.Result.Avg
	}

	status := f.Status
	if status == "" && f.Phase == PhaseDone {
		status = green + "✓ Test complete." + resetStyle
	}

	lines := []string{
		fmt.Sprintf("Written  %s%s%s  (%s%.1f%%%s)   ETA  %s%s%s",
			green, format.Bytes(written), resetStyle,
			yellow, pct, resetStyle,
			cyan, eta, resetStyle),
		fmt.Sprintf("Current  %s%s%s    Avg  %s%s%s",
			cyan, format.BytesPerSec(f.Sample.BlockSpeed), resetStyle,
			cyan, format.BytesPerSec(avg), resetStyle),
		fmt.Sprintf("Max      %s%s%s    Min  %s%s%s    Elapsed  %s",
			green, format.BytesPerSec(f.Max), resetStyle,
			red, format.BytesPerSec(f.Min), resetStyle,
			format.Duration(f.Sample.Elapsed)),
	}
	if f.Cache.Detected {
		lines = append(lines, fmt.Sprintf("Cache    %s~%s%s   (burst %s → steady %s)",
			yellow, format.Bytes(f.Cache.Bytes), resetStyle,
			format.BytesPerSec(f.Cache.BurstSpeed),
			format.BytesPerSec(f.Cache.SteadySpeed)))
	}
	lines = append(lines, status)
	return lines
}

func footerFor(f Frame) string {
	switch f.Phase {
	case PhaseConfirm:
		return " " + bold + "[Enter]" + resetStyle + dim + " start   " +
			bold + "[q]" + resetStyle + dim + " quit"
	case PhaseRunning:
		return " [Ctrl+C] cancel  ·  the report opens after completion"
	case PhaseDone:
		return " " + bold + "[c]" + resetStyle + dim + " copy summary   " +
			bold + "[h]" + resetStyle + dim + " save HTML report   " +
			bold + "[q]" + resetStyle + dim + " quit"
	}
	return ""
}

func valueOrDash(s string) string {
	if s == "" {
		return "—"
	}
	return s
}

// writeBox draws a titled rounded rectangle. lines are pre-rendered (may
// contain ANSI styling).
func writeBox(b *strings.Builder, title string, cols int, lines []string) {
	if cols < 4 {
		cols = 4
	}
	// Top border
	b.WriteString(clearLine)
	b.WriteString(gray + "╭")
	if title != "" {
		b.WriteString("─ " + bold + title + resetStyle + gray + " ")
		left := cols - len(title) - 4 - 1
		if left < 0 {
			left = 0
		}
		b.WriteString(strings.Repeat("─", left))
	} else {
		b.WriteString(strings.Repeat("─", cols-2))
	}
	b.WriteString("╮" + resetStyle + "\n")

	for _, ln := range lines {
		b.WriteString(clearLine)
		b.WriteString(gray + "│" + resetStyle + " ")
		b.WriteString(ln)
		b.WriteString("\n")
	}

	b.WriteString(clearLine)
	b.WriteString(gray + "╰")
	b.WriteString(strings.Repeat("─", cols-2))
	b.WriteString("╯" + resetStyle + "\n")
}
