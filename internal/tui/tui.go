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

// Render draws one full frame using the supplied state. samples are the values
// for the chart; the most recent is rightmost.
type Frame struct {
	Sys      hwinfo.SystemInfo
	FileSize int64
	Sample   bench.Sample
	History  []float64 // BlockSpeed values, oldest → newest
	Min      float64
	Max      float64
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
		fmt.Sprintf("%sStorage%s %s total · %s free · test size %s",
			dim, resetStyle,
			format.Bytes(f.Sys.DiskSizeBytes),
			format.Bytes(f.Sys.DiskFreeBytes),
			format.Bytes(f.FileSize)),
		fmt.Sprintf("%sSystem%s  %s · %d cores · %s RAM · %s/%s",
			dim, resetStyle,
			valueOrDash(f.Sys.CPUModel),
			f.Sys.CPUCores,
			format.Bytes(f.Sys.RAMBytes),
			f.Sys.OS, f.Sys.Arch),
	})

	// ── Chart pane (uses remaining vertical space minus metrics+footer) ─
	headerH := 6
	metricsH := 5
	footerH := 1
	chartH := rows - headerH - metricsH - footerH
	if chartH < 4 {
		chartH = 4
	}
	chartLines := SparklineLines(f.History, cols-2, chartH-2, 0)
	titleLine := fmt.Sprintf("Speed (last %ds)", len(f.History))
	chartContent := append([]string{
		dim + titleLine + resetStyle,
	}, chartLines...)
	writeBox(&b, "", cols, chartContent)

	// ── Metrics pane ────────────────────────────────────────────────
	pct := 0.0
	if f.FileSize > 0 {
		pct = float64(f.Sample.BytesWritten) / float64(f.FileSize) * 100
	}
	eta := "—"
	if f.Sample.AvgSpeed > 0 && f.Sample.BytesWritten < f.FileSize {
		remaining := f.FileSize - f.Sample.BytesWritten
		eta = format.Duration(time.Duration(float64(remaining) / f.Sample.AvgSpeed * float64(time.Second)))
	}
	writeBox(&b, "Metrics", cols, []string{
		fmt.Sprintf("Written  %s%s%s  (%s%.1f%%%s)   ETA  %s%s%s",
			green, format.Bytes(f.Sample.BytesWritten), resetStyle,
			yellow, pct, resetStyle,
			cyan, eta, resetStyle),
		fmt.Sprintf("Current  %s%s%s    Avg  %s%s%s",
			cyan, format.BytesPerSec(f.Sample.BlockSpeed), resetStyle,
			cyan, format.BytesPerSec(f.Sample.AvgSpeed), resetStyle),
		fmt.Sprintf("Max      %s%s%s    Min  %s%s%s    Elapsed  %s",
			green, format.BytesPerSec(f.Max), resetStyle,
			red, format.BytesPerSec(f.Min), resetStyle,
			format.Duration(f.Sample.Elapsed)),
	})

	// ── Footer ──────────────────────────────────────────────────────
	b.WriteString(clearLine)
	b.WriteString(dim)
	b.WriteString(" [Ctrl+C] cancel  ·  report saved on completion")
	b.WriteString(resetStyle)
	b.WriteString("\n")

	b.WriteString(clearBelow)
	fmt.Fprint(s.w, b.String())
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
