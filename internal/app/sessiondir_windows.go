//go:build windows

package app

import (
	"errors"
	"os"
	"unsafe"

	"golang.org/x/sys/windows"
)

func openLockFile(path string) (*os.File, error) {
	return os.OpenFile(path, os.O_RDWR|os.O_CREATE, 0o600)
}

// tryLockFile takes a non-blocking exclusive LockFileEx lock on the first byte.
// The lock belongs to the file handle, so it conflicts with any other open of
// the file, in this process or another, and is released when the handle is
// closed or the process dies.
func tryLockFile(f *os.File) error {
	ol := new(windows.Overlapped)
	err := windows.LockFileEx(windows.Handle(f.Fd()), windows.LOCKFILE_EXCLUSIVE_LOCK|windows.LOCKFILE_FAIL_IMMEDIATELY, 0, 1, 0, ol)
	if errors.Is(err, windows.ERROR_LOCK_VIOLATION) {
		return errSessionLocked
	}
	return err
}

// unlockFile unlocks explicitly because Windows may release locks on a closed
// handle lazily.
func unlockFile(f *os.File) error {
	return windows.UnlockFileEx(windows.Handle(f.Fd()), 0, 1, 0, new(windows.Overlapped))
}

// ownedByCurrentUser reports whether the folder's owner is the current user or
// the process token's default owner (the Administrators group when elevated).
func ownedByCurrentUser(path string, _ os.FileInfo) (bool, error) {
	sd, err := windows.GetNamedSecurityInfo(path, windows.SE_FILE_OBJECT, windows.OWNER_SECURITY_INFORMATION)
	if err != nil {
		return false, err
	}
	owner, _, err := sd.Owner()
	if err != nil {
		return false, err
	}
	if owner == nil {
		return false, errors.New("folder has no owner")
	}

	token := windows.GetCurrentProcessToken()
	user, err := token.GetTokenUser()
	if err != nil {
		return false, err
	}
	if windows.EqualSid(owner, user.User.Sid) {
		return true, nil
	}
	var n uint32
	_ = windows.GetTokenInformation(token, windows.TokenOwner, nil, 0, &n)
	if n == 0 {
		return false, errors.New("token owner is unavailable")
	}
	buf := make([]byte, n)
	if err := windows.GetTokenInformation(token, windows.TokenOwner, &buf[0], n, &n); err != nil {
		return false, err
	}
	// TOKEN_OWNER is a struct holding a single SID pointer.
	tokenOwner := (*struct{ owner *windows.SID })(unsafe.Pointer(&buf[0]))
	return windows.EqualSid(owner, tokenOwner.owner), nil
}
