//go:build darwin

package app

import "golang.org/x/sys/unix"

func totalRAMBytes() (uint64, error) { return unix.SysctlUint64("hw.memsize") }
