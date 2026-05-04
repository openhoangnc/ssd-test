//go:build darwin || dragonfly || freebsd || linux || netbsd || openbsd

package tui

import (
	"os"
	"syscall"
	"unsafe"
)

type winsize struct {
	Row, Col, X, Y uint16
}

func termSize(f *os.File) (cols, rows int) {
	var ws winsize
	_, _, errno := syscall.Syscall(
		syscall.SYS_IOCTL,
		f.Fd(),
		uintptr(syscall.TIOCGWINSZ),
		uintptr(unsafe.Pointer(&ws)),
	)
	if errno != 0 {
		return 0, 0
	}
	return int(ws.Col), int(ws.Row)
}
