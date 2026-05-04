//go:build darwin || dragonfly || freebsd || linux || netbsd || openbsd

package diskstats

import "syscall"

func getDiskStats(path string) (int64, int64) {
	fs := syscall.Statfs_t{}
	if err := syscall.Statfs(path, &fs); err != nil {
		return 0, 0
	}
	total := int64(fs.Blocks) * int64(fs.Bsize)
	free := int64(fs.Bavail) * int64(fs.Bsize)
	return total, free
}
