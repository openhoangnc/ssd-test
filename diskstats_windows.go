//go:build windows

package main

import (
	"syscall"
	"unsafe"
)

// getDiskStats returns the disk size and free space in bytes
func getDiskStats(path string) (int64, int64) {
	h := syscall.MustLoadDLL("kernel32.dll")
	c := h.MustFindProc("GetDiskFreeSpaceExW")

	var freeBytes int64
	var totalBytes int64
	var availBytes int64

	_, _, _ = c.Call(uintptr(unsafe.Pointer(syscall.StringToUTF16Ptr(path))),
		uintptr(unsafe.Pointer(&freeBytes)),
		uintptr(unsafe.Pointer(&totalBytes)),
		uintptr(unsafe.Pointer(&availBytes)))

	return totalBytes, availBytes
}
