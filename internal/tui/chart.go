package tui

import (
	"strings"

	"github.com/openhoangnc/ssd-test/internal/format"
)

// blocks are Unicode lower-eighth blocks U+2581..U+2588. Index 0 is "no block"
// (used for an empty column).
var blocks = []rune{' ', '▁', '▂', '▃', '▄', '▅', '▆', '▇', '█'}

// SparklineLines renders a multi-row vertical bar chart of values fitted to
// (cols, rows). Most-recent value is right-aligned. peak (in the same units as
// values) controls the y-axis ceiling; if peak <= 0 the auto-scale uses max(values).
//
// Each cell encodes one of 8 levels via the block characters; rows stack for
// taller bars. The leftmost two columns are reserved for an axis label (max value).
func SparklineLines(values []float64, cols, rows int, peak float64) []string {
	if cols < 10 {
		cols = 10
	}
	if rows < 1 {
		rows = 1
	}
	axisW := 8 // " 1.2GB/s" worth of space
	if cols-axisW < 4 {
		axisW = 0
	}
	chartW := cols - axisW

	max := peak
	for _, v := range values {
		if v > max {
			max = v
		}
	}
	if max <= 0 {
		max = 1
	}

	// Take the rightmost chartW values
	src := values
	if len(src) > chartW {
		src = src[len(src)-chartW:]
	}

	// Per-column, compute how many "eighths" of total height the bar fills:
	// totalHeight = rows * 8 eighths.
	totalEighths := rows * 8
	heights := make([]int, len(src))
	for i, v := range src {
		ratio := v / max
		if ratio < 0 {
			ratio = 0
		}
		if ratio > 1 {
			ratio = 1
		}
		heights[i] = int(ratio * float64(totalEighths))
	}

	out := make([]string, rows)
	for r := range rows {
		// row 0 is the top; bars grow from the bottom upward
		fromBottom := rows - 1 - r // how many full rows below this row
		var b strings.Builder
		// axis label: top row shows peak, bottom row shows 0; middle rows blank
		if axisW > 0 {
			label := ""
			switch r {
			case 0:
				label = format.BytesPerSec(max)
			case rows - 1:
				label = "0"
			}
			if len(label) > axisW-1 {
				label = label[:axisW-1]
			}
			b.WriteString(strings.Repeat(" ", axisW-1-len(label)))
			b.WriteString(gray)
			b.WriteString(label)
			b.WriteString(resetStyle)
			b.WriteByte(' ')
		}
		// Pad left so that the most recent sample sits at the right edge
		pad := chartW - len(heights)
		if pad < 0 {
			pad = 0
		}
		b.WriteString(strings.Repeat(" ", pad))

		b.WriteString(cyan)
		for _, h := range heights {
			// portion of h that lies inside this row (0..8 eighths)
			rowFloor := fromBottom * 8
			cell := h - rowFloor
			if cell < 0 {
				cell = 0
			}
			if cell > 8 {
				cell = 8
			}
			b.WriteRune(blocks[cell])
		}
		b.WriteString(resetStyle)
		out[r] = b.String()
	}
	return out
}
