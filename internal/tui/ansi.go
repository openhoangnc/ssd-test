package tui

// ANSI escape sequences. Avoiding any third-party term library — these work
// on every modern terminal: macOS Terminal, iTerm2, gnome-terminal, Alacritty,
// Windows Terminal, ConEmu, mintty, the new Windows 10+ console.
const (
	altScreenOn  = "\x1b[?1049h"
	altScreenOff = "\x1b[?1049l"
	cursorHide   = "\x1b[?25l"
	cursorShow   = "\x1b[?25h"
	cursorHome   = "\x1b[H"
	clearLine    = "\x1b[2K"
	clearBelow   = "\x1b[J"
	resetStyle   = "\x1b[0m"

	dim    = "\x1b[2m"
	bold   = "\x1b[1m"
	cyan   = "\x1b[36m"
	green  = "\x1b[32m"
	yellow = "\x1b[33m"
	red    = "\x1b[31m"
	gray   = "\x1b[90m"
)
