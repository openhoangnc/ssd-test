//go:build windows

package selfupdate

import (
	"fmt"
	"os"
	"os/exec"
)

// platformReplace works around the fact that Windows won't let you delete or
// overwrite a running .exe — but it does allow renaming. So:
//  1. rename current.exe → current.exe.old (succeeds while the process runs)
//  2. rename newBin → current.exe
//  3. spawn the new exe with our argv and exit
//
// Stale .exe.old files are removed on the next launch (CleanupStaleOld).
func platformReplace(currentExe, newBin string) error {
	old := currentExe + ".old"
	_ = os.Remove(old)
	if err := os.Rename(currentExe, old); err != nil {
		_ = os.Remove(newBin)
		return fmt.Errorf("rename current to .old: %w", err)
	}
	if err := os.Rename(newBin, currentExe); err != nil {
		// try to roll back
		_ = os.Rename(old, currentExe)
		return fmt.Errorf("rename new into place: %w", err)
	}
	cmd := exec.Command(currentExe, os.Args[1:]...)
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Start(); err != nil {
		return fmt.Errorf("start new binary: %w", err)
	}
	os.Exit(0)
	return nil
}

// CleanupStaleOld removes any leftover *.exe.old next to the current binary.
// Call once at startup. Errors are ignored.
func CleanupStaleOld() {
	exe, err := os.Executable()
	if err != nil {
		return
	}
	_ = os.Remove(exe + ".old")
}
