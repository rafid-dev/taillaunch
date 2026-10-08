//go:build !windows

package privatefs

import (
	"os"
)

// EnsureDir creates dir and restricts it to the current user.
func EnsureDir(dir string) error {
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return err
	}
	return os.Chmod(dir, 0o700)
}
