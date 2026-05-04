package report

import (
	"bytes"
	"image"
	"image/color"
	"image/png"
	"math"

	"github.com/openhoangnc/ssd-test/internal/format"
)

// PNG renders the speed-vs-bytes-written chart as a 1600x640 PNG using only
// stdlib. Glyphs come from a minimal 5x7 bitmap font baked below.
func PNG(r Result) ([]byte, error) {
	const (
		W, H = 1600, 640
		padL = 140
		padR = 40
		padT = 70
		padB = 90
	)
	plotW := W - padL - padR
	plotH := H - padT - padB

	bg := color.RGBA{0x0d, 0x11, 0x17, 0xff}
	gridC := color.RGBA{0x30, 0x36, 0x3d, 0xff}
	axisC := color.RGBA{0x8b, 0x94, 0x9e, 0xff}
	fgC := color.RGBA{0xc9, 0xd1, 0xd9, 0xff}
	lineC := color.RGBA{0x58, 0xa6, 0xff, 0xff}
	areaC := color.RGBA{0x58, 0xa6, 0xff, 0x40}
	avgC := color.RGBA{0x3f, 0xb9, 0x50, 0xff}

	img := image.NewRGBA(image.Rect(0, 0, W, H))
	fillRect(img, img.Bounds(), bg)

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

	xAt := func(written int64) int {
		return padL + int(float64(written)/float64(maxBytes)*float64(plotW))
	}
	yAt := func(speed float64) int {
		return padT + int((1.0-speed/maxSpeed)*float64(plotH))
	}

	// Title
	drawText2x(img, "SSD Test - write speed vs bytes written", padL, 22, fgC)
	subtitle := "device " + dashIfEmpty(r.Sys.DiskModel) + " - " +
		r.Sys.OS + "/" + r.Sys.Arch
	drawText(img, subtitle, padL, 52, axisC)

	// Y gridlines + labels
	for i := range 5 {
		gy := padT + i*plotH/4
		val := maxSpeed * (1 - float64(i)/4)
		drawHLine(img, padL, padL+plotW, gy, gridC)
		label := format.BytesPerSec(val)
		tw := textWidth(label)
		drawText(img, label, padL-12-tw, gy-3, fgC)
	}
	// Axes
	drawVLine(img, padL, padT, padT+plotH, axisC)
	drawHLine(img, padL, padL+plotW, padT+plotH, axisC)

	// X tick labels — bytes written
	for i := range 5 {
		gx := padL + i*plotW/4
		bw := int64(float64(maxBytes) * float64(i) / 4)
		label := format.Bytes(bw)
		tw := textWidth(label)
		drawText(img, label, gx-tw/2, padT+plotH+18, fgC)
		drawVLine(img, gx, padT+plotH, padT+plotH+4, axisC)
	}

	if len(r.Bench.Samples) > 0 {
		// Area fill — vertical lines from each point down to baseline
		for _, s := range r.Bench.Samples {
			px := xAt(s.BytesWritten)
			py := yAt(s.BlockSpeed)
			drawVLineAlpha(img, px, py, padT+plotH, areaC)
		}
		// Polyline
		prevX, prevY := -1, -1
		for _, s := range r.Bench.Samples {
			px := xAt(s.BytesWritten)
			py := yAt(s.BlockSpeed)
			if prevX >= 0 {
				drawLine(img, prevX, prevY, px, py, lineC)
				// thicken by 1px above
				drawLine(img, prevX, prevY-1, px, py-1, lineC)
			}
			prevX, prevY = px, py
		}
		// Avg dashed line
		ay := yAt(r.Bench.Avg)
		drawDashedHLine(img, padL, padL+plotW, ay, avgC)
		drawText(img, "avg "+format.BytesPerSec(r.Bench.Avg),
			padL+plotW-200, ay-8, avgC)
	}

	// Footer summary
	footer := "min " + format.BytesPerSec(r.Bench.Min) +
		"   avg " + format.BytesPerSec(r.Bench.Avg) +
		"   max " + format.BytesPerSec(r.Bench.Max) +
		"   written " + format.Bytes(r.Bench.Written) +
		"   in " + format.Duration(r.Bench.Duration)
	drawText(img, footer, padL, H-30, fgC)

	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

// ── primitive drawing ────────────────────────────────────────────────────

func fillRect(img *image.RGBA, r image.Rectangle, c color.RGBA) {
	for y := r.Min.Y; y < r.Max.Y; y++ {
		for x := r.Min.X; x < r.Max.X; x++ {
			img.SetRGBA(x, y, c)
		}
	}
}

func drawHLine(img *image.RGBA, x0, x1, y int, c color.RGBA) {
	if x0 > x1 {
		x0, x1 = x1, x0
	}
	for x := x0; x <= x1; x++ {
		img.SetRGBA(x, y, c)
	}
}

func drawVLine(img *image.RGBA, x, y0, y1 int, c color.RGBA) {
	if y0 > y1 {
		y0, y1 = y1, y0
	}
	for y := y0; y <= y1; y++ {
		img.SetRGBA(x, y, c)
	}
}

func drawVLineAlpha(img *image.RGBA, x, y0, y1 int, c color.RGBA) {
	if y0 > y1 {
		y0, y1 = y1, y0
	}
	for y := y0; y <= y1; y++ {
		blend(img, x, y, c)
	}
}

func drawDashedHLine(img *image.RGBA, x0, x1, y int, c color.RGBA) {
	for x := x0; x <= x1; x++ {
		if (x/8)%2 == 0 {
			img.SetRGBA(x, y, c)
		}
	}
}

// drawLine — Bresenham
func drawLine(img *image.RGBA, x0, y0, x1, y1 int, c color.RGBA) {
	dx := abs(x1 - x0)
	dy := -abs(y1 - y0)
	sx := 1
	if x0 >= x1 {
		sx = -1
	}
	sy := 1
	if y0 >= y1 {
		sy = -1
	}
	err := dx + dy
	for {
		if x0 >= 0 && y0 >= 0 && x0 < img.Rect.Max.X && y0 < img.Rect.Max.Y {
			img.SetRGBA(x0, y0, c)
		}
		if x0 == x1 && y0 == y1 {
			break
		}
		e2 := 2 * err
		if e2 >= dy {
			err += dy
			x0 += sx
		}
		if e2 <= dx {
			err += dx
			y0 += sy
		}
	}
}

func blend(img *image.RGBA, x, y int, c color.RGBA) {
	if x < 0 || y < 0 || x >= img.Rect.Max.X || y >= img.Rect.Max.Y {
		return
	}
	dst := img.RGBAAt(x, y)
	a := float64(c.A) / 255
	img.SetRGBA(x, y, color.RGBA{
		R: uint8(math.Round(float64(c.R)*a + float64(dst.R)*(1-a))),
		G: uint8(math.Round(float64(c.G)*a + float64(dst.G)*(1-a))),
		B: uint8(math.Round(float64(c.B)*a + float64(dst.B)*(1-a))),
		A: 0xff,
	})
}

func abs(n int) int {
	if n < 0 {
		return -n
	}
	return n
}

// ── text rendering ───────────────────────────────────────────────────────

func textWidth(s string) int {
	return len(s) * (glyphW + 1)
}

// drawText writes s at (x, y) where y is the baseline-ish top of the glyph.
func drawText(img *image.RGBA, s string, x, y int, c color.RGBA) {
	for _, ch := range s {
		drawGlyph(img, ch, x, y, c, 1)
		x += (glyphW + 1)
	}
}

// drawText2x doubles the glyph size for the title.
func drawText2x(img *image.RGBA, s string, x, y int, c color.RGBA) {
	for _, ch := range s {
		drawGlyph(img, ch, x, y, c, 2)
		x += (glyphW + 1) * 2
	}
}

func drawGlyph(img *image.RGBA, ch rune, x, y int, c color.RGBA, scale int) {
	if ch < 0x20 || ch > 0x7e {
		return
	}
	g := font5x7[ch-0x20]
	for row := range glyphH {
		bits := g[row]
		for col := range glyphW {
			if bits&(1<<(glyphW-1-col)) != 0 {
				for sy := range scale {
					for sx := range scale {
						img.SetRGBA(x+col*scale+sx, y+row*scale+sy, c)
					}
				}
			}
		}
	}
}
