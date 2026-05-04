//go:build windows

package tui

import (
	"errors"
	"os"
	"syscall"
	"unsafe"
)

const (
	enableLineInput      = 0x0002
	enableEchoInput      = 0x0004
	enableProcessedInput = 0x0001
)

type RawState struct {
	prev   uint32
	hasPrev bool
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
	var prev uint32
	r1, _, _ := getMode.Call(os.Stdin.Fd(), uintptr(unsafe.Pointer(&prev)))
	if r1 == 0 {
		return nil, errors.New("GetConsoleMode failed")
	}
	// Drop line-buffer + echo; keep processed input so Ctrl+C still raises.
	newMode := (prev &^ (enableLineInput | enableEchoInput)) | enableProcessedInput
	r1, _, _ = setMode.Call(os.Stdin.Fd(), uintptr(newMode))
	if r1 == 0 {
		return nil, errors.New("SetConsoleMode failed")
	}
	return &RawState{prev: prev, hasPrev: true}, nil
}

func (s *RawState) Restore() {
	if s == nil || !s.hasPrev {
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
	_, _, _ = setMode.Call(os.Stdin.Fd(), uintptr(s.prev))
	s.hasPrev = false
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
