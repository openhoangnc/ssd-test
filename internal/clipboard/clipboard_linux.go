//go:build linux

package clipboard

import (
	"errors"
	"os/exec"
)

func copyImpl(data string) error {
	candidates := []*exec.Cmd{
		exec.Command("wl-copy"),
		exec.Command("xclip", "-selection", "clipboard"),
		exec.Command("xsel", "-b", "-i"),
	}
	for _, cmd := range candidates {
		if _, err := exec.LookPath(cmd.Path); err == nil {
			return runWithStdin(cmd, data)
		}
	}
	return errors.New("no clipboard tool found (install wl-copy, xclip, or xsel)")
}
