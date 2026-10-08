//go:build !linux && !windows && !darwin

package app

import "errors"

func totalRAMBytes() (uint64, error) {
	return 0, errors.New("total memory is unavailable on this platform")
}
