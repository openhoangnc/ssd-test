//go:build windows

package tui

import (
	"os"
	"syscall"
	"unsafe"
)

type coord struct {
	X, Y int16
}
type smallRect struct {
	Left, Top, Right, Bottom int16
}
type consoleScreenBufferInfo struct {
	Size              coord
	CursorPosition    coord
	Attributes        uint16
	Window            smallRect
	MaximumWindowSize coord
}

func termSize(f *os.File) (cols, rows int) {
	h, err := syscall.LoadDLL("kernel32.dll")
	if err != nil {
		return 0, 0
	}
	proc, err := h.FindProc("GetConsoleScreenBufferInfo")
	if err != nil {
		return 0, 0
	}
	var info consoleScreenBufferInfo
	r1, _, _ := proc.Call(f.Fd(), uintptr(unsafe.Pointer(&info)))
	if r1 == 0 {
		return 0, 0
	}
	return int(info.Window.Right-info.Window.Left) + 1,
		int(info.Window.Bottom-info.Window.Top) + 1
}
