package clipboard

import (
	"fmt"
	"io"
	"os/exec"
	"strings"
)

func runWithStdin(cmd *exec.Cmd, data string) error {
	stdin, err := cmd.StdinPipe()
	if err != nil {
		return fmt.Errorf("clipboard: stdin pipe: %w", err)
	}
	if err := cmd.Start(); err != nil {
		return fmt.Errorf("clipboard: start %s: %w", cmd.Path, err)
	}
	if _, err := io.Copy(stdin, strings.NewReader(data)); err != nil {
		_ = stdin.Close()
		_ = cmd.Wait()
		return fmt.Errorf("clipboard: write: %w", err)
	}
	if err := stdin.Close(); err != nil {
		_ = cmd.Wait()
		return fmt.Errorf("clipboard: close stdin: %w", err)
	}
	if err := cmd.Wait(); err != nil {
		return fmt.Errorf("clipboard: %s exited: %w", cmd.Path, err)
	}
	return nil
}
