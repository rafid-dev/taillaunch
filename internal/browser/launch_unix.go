//go:build !windows

package browser

import (
	"fmt"
	"os/exec"
)

type commandSession struct {
	cmd *exec.Cmd
}

func launch(executable string, args []string) (Session, error) {
	cmd := exec.Command(executable, args...)
	if err := cmd.Start(); err != nil {
		return nil, fmt.Errorf("start browser %q: %w", executable, err)
	}
	return &commandSession{cmd: cmd}, nil
}

func (s *commandSession) Wait() error { return s.cmd.Wait() }

func (s *commandSession) Kill() error {
	if s.cmd.Process == nil {
		return nil
	}
	return s.cmd.Process.Kill()
}

func (s *commandSession) Close() error { return nil }
