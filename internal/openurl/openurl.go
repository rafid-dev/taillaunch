package openurl

import (
	"fmt"
	"os/exec"
	"runtime"
)

func Open(rawURL string) error {
	var cmd *exec.Cmd
	switch runtime.GOOS {
	case "windows":
		cmd = exec.Command("rundll32", "url.dll,FileProtocolHandler", rawURL)
	case "darwin":
		cmd = exec.Command("open", rawURL)
	default:
		cmd = exec.Command("xdg-open", rawURL)
	}
	if err := cmd.Start(); err != nil {
		return fmt.Errorf("open URL: %w", err)
	}
	go func() { _ = cmd.Wait() }()
	return nil
}
