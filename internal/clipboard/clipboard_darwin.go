//go:build darwin

package clipboard

import "os/exec"

func copyImpl(data string) error {
	return runWithStdin(exec.Command("pbcopy"), data)
}
