//go:build windows

package tui

import (
	"errors"
	"os"
	"syscall"
	"unsafe"
)

const (
	enableLineInput                = 0x0002
	enableEchoInput                = 0x0004
	enableProcessedInput           = 0x0001
	enableVirtualTerminalProcessing = 0x0004
)

type RawState struct {
	prevIn   uint32
	prevOut  uint32
	hasIn    bool
	hasOut   bool
}

func EnterRaw() (*RawState, error) {
	if !IsTTY() {
		return &RawState{}, nil
	}
	h, err := syscall.LoadDLL("kernel32.dll")
	if err != nil {
		return nil, err
	}
	getMode, err := h.FindProc("GetConsoleMode")
	if err != nil {
		return nil, err
	}
	setMode, err := h.FindProc("SetConsoleMode")
	if err != nil {
		return nil, err
	}

	state := &RawState{}

	var prevIn uint32
	r1, _, _ := getMode.Call(os.Stdin.Fd(), uintptr(unsafe.Pointer(&prevIn)))
	if r1 == 0 {
		return nil, errors.New("GetConsoleMode failed")
	}
	// Drop line-buffer + echo; keep processed input so Ctrl+C still raises.
	newIn := (prevIn &^ (enableLineInput | enableEchoInput)) | enableProcessedInput
	r1, _, _ = setMode.Call(os.Stdin.Fd(), uintptr(newIn))
	if r1 == 0 {
		return nil, errors.New("SetConsoleMode failed")
	}
	state.prevIn = prevIn
	state.hasIn = true

	// Enable ANSI escape processing on stdout so the alt-screen, cursor, and
	// color sequences render correctly in legacy cmd.exe / conhost. Modern
	// hosts (Windows Terminal, VS Code) already have this on; the call is a
	// no-op for them. Failure is non-fatal — the TUI still works wherever VT
	// is on by default.
	var prevOut uint32
	r1, _, _ = getMode.Call(os.Stdout.Fd(), uintptr(unsafe.Pointer(&prevOut)))
	if r1 != 0 {
		newOut := prevOut | enableVirtualTerminalProcessing
		r1, _, _ = setMode.Call(os.Stdout.Fd(), uintptr(newOut))
		if r1 != 0 {
			state.prevOut = prevOut
			state.hasOut = true
		}
	}

	return state, nil
}

func (s *RawState) Restore() {
	if s == nil || (!s.hasIn && !s.hasOut) {
		return
	}
	h, err := syscall.LoadDLL("kernel32.dll")
	if err != nil {
		return
	}
	setMode, err := h.FindProc("SetConsoleMode")
	if err != nil {
		return
	}
	if s.hasIn {
		_, _, _ = setMode.Call(os.Stdin.Fd(), uintptr(s.prevIn))
		s.hasIn = false
	}
	if s.hasOut {
		_, _, _ = setMode.Call(os.Stdout.Fd(), uintptr(s.prevOut))
		s.hasOut = false
	}
}

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
