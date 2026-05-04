//go:build darwin || dragonfly || freebsd || linux || netbsd || openbsd

package tui

import (
	"errors"
	"os"
	"os/exec"
	"strings"
)

// RawState holds the saved terminal settings to restore on exit. We shell out
// to /bin/stty rather than reach into termios syscalls so the same code works
// across darwin/linux/bsd without per-OS ioctl constants.
type RawState struct {
	saved string
}

// EnterRaw puts the controlling terminal in cbreak-ish mode: characters arrive
// without waiting for Enter, no local echo. Signals (Ctrl+C) still work so the
// user can always abort.
func EnterRaw() (*RawState, error) {
	if !IsTTY() {
		return &RawState{}, nil
	}
	saved, err := sttyGet()
	if err != nil {
		return nil, err
	}
	if err := sttyApply("-icanon", "-echo", "min", "1", "time", "0"); err != nil {
		return nil, err
	}
	return &RawState{saved: saved}, nil
}

// Restore reverts the terminal to the state saved by EnterRaw. Safe to call
// twice or with a zero RawState.
func (s *RawState) Restore() {
	if s == nil || s.saved == "" {
		return
	}
	_ = sttyApply(s.saved)
	s.saved = ""
}

// ReadKey returns the next byte from stdin. Multi-byte escape sequences (e.g.
// arrow keys) are not interpreted — the first byte is enough for our menu.
func ReadKey() (byte, error) {
	buf := make([]byte, 1)
	for {
		n, err := os.Stdin.Read(buf)
		if err != nil {
			return 0, err
		}
		if n > 0 {
			return buf[0], nil
		}
	}
}

func sttyGet() (string, error) {
	cmd := exec.Command("stty", "-g")
	cmd.Stdin = os.Stdin
	out, err := cmd.Output()
	if err != nil {
		return "", errors.New("stty -g: " + err.Error())
	}
	return strings.TrimSpace(string(out)), nil
}

func sttyApply(args ...string) error {
	cmd := exec.Command("stty", args...)
	cmd.Stdin = os.Stdin
	return cmd.Run()
}
