package tui

import "os"

// Size returns the terminal (cols, rows) for stdout. Defaults to 80x24 if the
// query fails (e.g. not a TTY, unsupported platform path).
func Size() (cols, rows int) {
	c, r := termSize(os.Stdout)
	if c <= 0 || r <= 0 {
		return 80, 24
	}
	return c, r
}
