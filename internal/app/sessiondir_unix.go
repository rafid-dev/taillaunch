//go:build !windows

package app

import (
	"errors"
	"os"
	"syscall"

	"golang.org/x/sys/unix"
)

// openLockFile never follows a symlink planted at the lock path.
func openLockFile(path string) (*os.File, error) {
	return os.OpenFile(path, os.O_RDWR|os.O_CREATE|unix.O_NOFOLLOW, 0o600)
}

// tryLockFile takes a non-blocking exclusive flock. The lock belongs to the
// open file description, so it conflicts with any other open of the file, in
// this process or another, and is released when the descriptor is closed or
// the process dies.
func tryLockFile(f *os.File) error {
	for {
		err := unix.Flock(int(f.Fd()), unix.LOCK_EX|unix.LOCK_NB)
		switch {
		case err == nil:
			return nil
		case errors.Is(err, unix.EINTR):
			continue
		case errors.Is(err, unix.EWOULDBLOCK):
			return errSessionLocked
		default:
			return err
		}
	}
}

func unlockFile(f *os.File) error {
	return unix.Flock(int(f.Fd()), unix.LOCK_UN)
}

// ownedByCurrentUser reports whether the folder belongs to the current user.
func ownedByCurrentUser(_ string, fi os.FileInfo) (bool, error) {
	st, ok := fi.Sys().(*syscall.Stat_t)
	if !ok {
		return false, errors.New("owner information is unavailable")
	}
	return int(st.Uid) == os.Getuid(), nil
}
