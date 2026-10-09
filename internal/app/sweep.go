package app

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"time"
)

// legacyMaxAge is how old an unmarked (v0.2.0) folder must be before it is
// deleted. v0.2.0 wrote no marker or lock, so age is the only evidence.
const legacyMaxAge = 24 * time.Hour

// sessionDirName matches what os.MkdirTemp produces for the two patterns in
// PrepareDirs: the pattern followed by a decimal random number. It is
// deliberately strict; anything else in the temp dir is not ours to delete.
var sessionDirName = regexp.MustCompile(`^taillaunch-(?:state|browser)-[0-9]+$`)

// SweepStaleSessionDirs removes disposable session folders left behind by
// crashed sessions from the OS temp dir and returns how many it removed.
// Failures are reported through logf and never abort the sweep.
func SweepStaleSessionDirs(logf func(format string, args ...any)) int {
	return newSweeper(os.TempDir(), logf).run()
}

// sweeper deletes a folder only when it is a real directory directly under
// root, owned by the current user, with a name TailLaunch generates, and
// either
//   - carries the marker and its lock can be taken (no live session holds
//     it), or
//   - carries no valid marker and has not been modified for legacyAge.
//
// The state and browser folders get independent random names, so they cannot
// be paired; each is judged on its own. Each session holds a lock on both.
type sweeper struct {
	root      string
	now       func() time.Time
	legacyAge time.Duration
	logf      func(format string, args ...any)
	removeAll func(path string) error
}

func newSweeper(root string, logf func(format string, args ...any)) *sweeper {
	if logf == nil {
		logf = func(string, ...any) {}
	}
	return &sweeper{root: root, now: time.Now, legacyAge: legacyMaxAge, logf: logf, removeAll: os.RemoveAll}
}

func (s *sweeper) run() int {
	entries, err := os.ReadDir(s.root)
	if err != nil {
		if !errors.Is(err, os.ErrNotExist) {
			s.logf("stale session cleanup: read %q: %v", s.root, err)
		}
		return 0
	}
	removed := 0
	for _, entry := range entries {
		if !sessionDirName.MatchString(entry.Name()) {
			continue
		}
		dir := filepath.Join(s.root, entry.Name())
		ok, err := s.eligible(dir)
		if err != nil {
			s.logf("stale session cleanup: skipping %q: %v", dir, err)
			continue
		}
		if !ok {
			continue
		}
		if s.sweepDir(dir) {
			removed++
		}
	}
	return removed
}

// eligible reports whether dir is a real directory (not a symlink or
// junction) owned by the current user.
func (s *sweeper) eligible(dir string) (bool, error) {
	fi, err := os.Lstat(dir)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return false, nil
		}
		return false, err
	}
	if !fi.IsDir() || fi.Mode()&(os.ModeSymlink|os.ModeIrregular) != 0 {
		return false, nil
	}
	return ownedByCurrentUser(dir, fi)
}

func (s *sweeper) sweepDir(dir string) bool {
	if !hasMarker(dir) {
		age, err := dirAge(dir, s.now())
		if err != nil {
			s.logf("stale session cleanup: skipping %q: %v", dir, err)
			return false
		}
		if age < s.legacyAge {
			return false
		}
		if err := s.removeAll(dir); err != nil {
			s.logf("stale session cleanup: could not remove %q: %v", dir, err)
			return false
		}
		return true
	}

	lock, err := acquireSessionLock(dir)
	if errors.Is(err, errSessionLocked) {
		return false // a live session owns it
	}
	if err != nil {
		s.logf("stale session cleanup: skipping %q: %v", dir, err)
		return false
	}
	// Hold the lock while emptying the folder so nothing can claim it halfway.
	// The marker goes last, so a folder that could not be emptied (for example
	// a browser process still has files open) is retried at the next startup.
	if err := s.empty(dir); err != nil {
		s.logf("stale session cleanup: could not remove %q: %v", dir, err)
		if err := lock.Release(); err != nil {
			s.logf("stale session cleanup: release lock in %q: %v", dir, err)
		}
		return false
	}
	// Windows cannot delete the lock file or folder while the handle is open.
	if err := lock.Release(); err != nil {
		s.logf("stale session cleanup: release lock in %q: %v", dir, err)
	}
	if err := s.removeAll(dir); err != nil {
		s.logf("stale session cleanup: could not remove %q: %v", dir, err)
		return false
	}
	return true
}

// empty removes everything in dir except the lock file and the marker, then
// the marker.
func (s *sweeper) empty(dir string) error {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return err
	}
	var errs []error
	for _, entry := range entries {
		switch entry.Name() {
		case lockFileName, markerFileName:
			continue
		}
		if err := s.removeAll(filepath.Join(dir, entry.Name())); err != nil {
			errs = append(errs, err)
		}
	}
	if len(errs) > 0 {
		return errors.Join(errs...)
	}
	return s.removeAll(filepath.Join(dir, markerFileName))
}

// dirAge is the time since dir or any of its direct children was last
// modified. A folder from a still-running v0.2.0 session keeps getting fresh
// files (Tailscale state, browser profile data), which keeps it from being
// treated as abandoned just because the folder itself is old.
func dirAge(dir string, now time.Time) (time.Duration, error) {
	fi, err := os.Lstat(dir)
	if err != nil {
		return 0, err
	}
	newest := fi.ModTime()
	entries, err := os.ReadDir(dir)
	if err != nil {
		return 0, fmt.Errorf("read folder: %w", err)
	}
	for _, entry := range entries {
		info, err := entry.Info()
		if err != nil {
			continue
		}
		if info.ModTime().After(newest) {
			newest = info.ModTime()
		}
	}
	return now.Sub(newest), nil
}
