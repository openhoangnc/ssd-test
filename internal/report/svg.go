package report

import (
	"fmt"
	"strings"

	"github.com/openhoangnc/ssd-test/internal/bench"
	"github.com/openhoangnc/ssd-test/internal/format"
)

// SVG renders the speed-vs-bytes-written chart as a self-contained SVG string.
// X axis = cumulative bytes written (so cache cliffs line up with where they
// actually happened on the drive, not when on the wall clock).
func SVG(r Result) string {
	const (
		W, H = 880, 360
		padL = 90
		padR = 24
		padT = 36
		padB = 56
	)
	plotW := W - padL - padR
	plotH := H - padT - padB

	samples := r.Bench.Samples
	maxSpeed := r.Bench.Max
	if maxSpeed <= 0 {
		maxSpeed = 1
	}
	maxBytes := r.Bench.Written
	if maxBytes <= 0 {
		maxBytes = r.Bench.FileSize
	}
	if maxBytes <= 0 {
		maxBytes = 1
	}

	x := func(written int64) float64 {
		return float64(padL) + float64(written)/float64(maxBytes)*float64(plotW)
	}
	y := func(speed float64) float64 {
		return float64(padT) + (1.0-speed/maxSpeed)*float64(plotH)
	}

	var b strings.Builder
	fmt.Fprintf(&b, `<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 %d %d" role="img" aria-label="Write speed by bytes written">`, W, H)
	b.WriteString(`<style>
.bg{fill:var(--bg,#0d1117)}
.grid{stroke:var(--grid,#30363d);stroke-width:1}
.axis{stroke:var(--axis,#8b949e);stroke-width:1.5}
.label{fill:var(--fg,#c9d1d9);font:12px ui-sans-serif,system-ui,sans-serif}
.title{fill:var(--fg,#c9d1d9);font:600 14px ui-sans-serif,system-ui,sans-serif}
.line{fill:none;stroke:var(--line,#58a6ff);stroke-width:2;stroke-linejoin:round}
.area{fill:var(--area,rgba(88,166,255,0.18));stroke:none}
.avg{stroke:var(--avg,#3fb950);stroke-width:1.5;stroke-dasharray:4 4;fill:none}
.cache{stroke:var(--cache,#d29922);stroke-width:1.5;stroke-dasharray:6 4;fill:none}
.cache-label{fill:var(--cache,#d29922);font:600 12px ui-sans-serif,system-ui,sans-serif}
@media (prefers-color-scheme: light){
  .bg{fill:#ffffff}.grid{stroke:#e1e4e8}.axis{stroke:#57606a}
  .label,.title{fill:#1f2328}.line{stroke:#0969da}.area{fill:rgba(9,105,218,0.16)}
  .avg{stroke:#1a7f37}.cache{stroke:#9a6700}.cache-label{fill:#9a6700}
}
</style>`)
	fmt.Fprintf(&b, `<rect class="bg" width="%d" height="%d"/>`, W, H)
	fmt.Fprintf(&b, `<text class="title" x="%d" y="22">Write speed vs bytes written</text>`, padL)

	// Y-axis gridlines and labels (speed)
	for i := range 5 {
		gy := padT + i*plotH/4
		val := maxSpeed * (1 - float64(i)/4)
		fmt.Fprintf(&b, `<line class="grid" x1="%d" y1="%d" x2="%d" y2="%d"/>`,
			padL, gy, padL+plotW, gy)
		fmt.Fprintf(&b, `<text class="label" x="%d" y="%d" text-anchor="end">%s</text>`,
			padL-8, gy+4, format.BytesPerSec(val))
	}
	// Axes
	fmt.Fprintf(&b, `<line class="axis" x1="%d" y1="%d" x2="%d" y2="%d"/>`,
		padL, padT+plotH, padL+plotW, padT+plotH)
	fmt.Fprintf(&b, `<line class="axis" x1="%d" y1="%d" x2="%d" y2="%d"/>`,
		padL, padT, padL, padT+plotH)

	// X-axis ticks (5 evenly spaced, in bytes)
	for i := range 5 {
		gx := padL + i*plotW/4
		bw := int64(float64(maxBytes) * float64(i) / 4)
		fmt.Fprintf(&b, `<text class="label" x="%d" y="%d" text-anchor="middle">%s</text>`,
			gx, padT+plotH+18, format.Bytes(bw))
	}

	if len(samples) > 0 {
		var line, area strings.Builder
		first := true
		for _, s := range samples {
			px := x(s.BytesWritten)
			py := y(s.BlockSpeed)
			if first {
				fmt.Fprintf(&area, "M %.2f %.2f ", px, float64(padT+plotH))
				fmt.Fprintf(&area, "L %.2f %.2f ", px, py)
				fmt.Fprintf(&line, "M %.2f %.2f ", px, py)
				first = false
			} else {
				fmt.Fprintf(&area, "L %.2f %.2f ", px, py)
				fmt.Fprintf(&line, "L %.2f %.2f ", px, py)
			}
		}
		if !first {
			fmt.Fprintf(&area, "L %.2f %.2f Z", float64(padL+plotW), float64(padT+plotH))
		}
		fmt.Fprintf(&b, `<path class="area" d="%s"/>`, area.String())
		fmt.Fprintf(&b, `<path class="line" d="%s"/>`, line.String())

		ay := y(r.Bench.Avg)
		fmt.Fprintf(&b, `<line class="avg" x1="%d" y1="%.2f" x2="%d" y2="%.2f"/>`,
			padL, ay, padL+plotW, ay)
		fmt.Fprintf(&b, `<text class="label" x="%d" y="%.2f">avg %s</text>`,
			padL+plotW-90, ay-6, format.BytesPerSec(r.Bench.Avg))

		if cache := bench.EstimateCache(r.Bench.Samples); cache.Detected {
			cx := x(cache.Bytes)
			fmt.Fprintf(&b, `<line class="cache" x1="%.2f" y1="%d" x2="%.2f" y2="%d"/>`,
				cx, padT, cx, padT+plotH)
			anchor := "start"
			tx := cx + 6
			if cx > float64(padL+plotW)-120 {
				anchor = "end"
				tx = cx - 6
			}
			fmt.Fprintf(&b, `<text class="cache-label" x="%.2f" y="%d" text-anchor="%s">cache ~%s</text>`,
				tx, padT+14, anchor, format.Bytes(cache.Bytes))
		}
	}

	b.WriteString(`</svg>`)
	return b.String()
}
