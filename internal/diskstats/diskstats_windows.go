//go:build windows

package diskstats

import (
	"syscall"
	"unsafe"
)

func getDiskStats(path string) (int64, int64) {
	h, err := syscall.LoadDLL("kernel32.dll")
	if err != nil {
		return 0, 0
	}
	c, err := h.FindProc("GetDiskFreeSpaceExW")
	if err != nil {
		return 0, 0
	}

	var freeBytes, totalBytes, availBytes int64
	pathPtr, err := syscall.UTF16PtrFromString(path)
	if err != nil {
		return 0, 0
	}

	_, _, _ = c.Call(
		uintptr(unsafe.Pointer(pathPtr)),
		uintptr(unsafe.Pointer(&freeBytes)),
		uintptr(unsafe.Pointer(&totalBytes)),
		uintptr(unsafe.Pointer(&availBytes)),
	)
	return totalBytes, availBytes
}
