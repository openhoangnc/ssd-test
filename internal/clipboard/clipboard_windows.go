//go:build windows

package clipboard

import "os/exec"

func copyImpl(data string) error {
	return runWithStdin(exec.Command("clip.exe"), data)
}
