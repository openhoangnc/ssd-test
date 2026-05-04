// Package report renders bench results to HTML, PNG, SVG, and Markdown.
package report

import (
	"github.com/openhoangnc/ssd-test/internal/bench"
	"github.com/openhoangnc/ssd-test/internal/hwinfo"
)

// Result is the inputs every exporter consumes.
type Result struct {
	Sys   hwinfo.SystemInfo
	Bench bench.Result
}
