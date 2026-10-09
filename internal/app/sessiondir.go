package app

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
)

// Every disposable session folder holds an exclusive OS file lock on lockFileName
// for as long as the session lives and carries markerFileName once the lock is
// held. The OS drops the lock when the process exits for any reason, so an
// unlocked marked folder is one whose session is gone. See sweep.go.
const (
	lockFileName   = ".taillaunch-lock"
	markerFileName = ".taillaunch-session"
	markerContents = "TailLaunch disposable session folder v1\n"
)

var errSessionLocked = errors.New("session directory is locked by another process")

type sessionLock struct{ f *os.File }

// Release drops the lock. It is safe to call more than once.
func (l *sessionLock) Release() error {
	if l == nil || l.f == nil {
		return nil
	}
	f := l.f
	l.f = nil
	return errors.Join(unlockFile(f), f.Close())
}

// lockSessionDir takes the folder's exclusive lock and only then writes the
// marker, so a marked folder is never observed without a live holder unless
// the holder has gone away.
func lockSessionDir(dir string) (*sessionLock, error) {
	lock, err := acquireSessionLock(dir)
	if err != nil {
		return nil, err
	}
	if err := os.WriteFile(filepath.Join(dir, markerFileName), []byte(markerContents), 0o600); err != nil {
		return nil, errors.Join(fmt.Errorf("write session marker: %w", err), lock.Release())
	}
	return lock, nil
}

// acquireSessionLock takes the lock without insisting on the marker. It
// returns errSessionLocked when another holder has it.
func acquireSessionLock(dir string) (*sessionLock, error) {
	path := filepath.Join(dir, lockFileName)
	if fi, err := os.Lstat(path); err == nil && !fi.Mode().IsRegular() {
		return nil, fmt.Errorf("lock file %q is not a regular file", path)
	}
	f, err := openLockFile(path)
	if err != nil {
		return nil, fmt.Errorf("open lock file: %w", err)
	}
	if err := tryLockFile(f); err != nil {
		f.Close()
		return nil, err
	}
	return &sessionLock{f: f}, nil
}

// hasMarker reports whether dir carries TailLaunch's marker file.
func hasMarker(dir string) bool {
	path := filepath.Join(dir, markerFileName)
	fi, err := os.Lstat(path)
	if err != nil || !fi.Mode().IsRegular() || fi.Size() != int64(len(markerContents)) {
		return false
	}
	data, err := os.ReadFile(path)
	return err == nil && string(data) == markerContents
}
