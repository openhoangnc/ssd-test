//go:build darwin || dragonfly || freebsd || linux || netbsd || openbsd

package selfupdate

import (
	"fmt"
	"os"
	"syscall"
)

// platformReplace replaces the current executable inode with newBin and
// re-execs the process. On Unix you can rename over a running executable.
func platformReplace(currentExe, newBin string) error {
	if err := os.Rename(newBin, currentExe); err != nil {
		_ = os.Remove(newBin)
		return fmt.Errorf("rename: %w", err)
	}
	return syscall.Exec(currentExe, os.Args, os.Environ())
}

// CleanupStaleOld is a no-op on Unix; the Windows build needs it to clean up
// leftover .exe.old files. Defined here so main.go can call it unconditionally.
func CleanupStaleOld() {}
