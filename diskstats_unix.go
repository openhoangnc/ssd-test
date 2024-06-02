//go:build darwin || dragonfly || freebsd || linux || netbsd || openbsd

package main

import (
	"syscall"
)

// getDiskStats returns the disk size and free space in bytes
func getDiskStats(path string) (int64, int64) {
	fs := syscall.Statfs_t{}
	err := syscall.Statfs(path, &fs)
	if err != nil {
		panic(err)
	}

	diskSize := fs.Blocks * uint64(fs.Bsize)

	freeSpace := fs.Bavail * uint64(fs.Bsize)

	return int64(diskSize), int64(freeSpace)
}
