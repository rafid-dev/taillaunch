package app

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"github.com/rafid-dev/taillaunch/internal/privatefs"
)

// PrepareDirs selects reusable directories only when persistence was
// explicitly requested. Otherwise it creates private per-run directories and
// returns a cleanup function for them.
func PrepareDirs(opts Options) (stateDir, profileDir string, cleanup func() error, err error) {
	if !opts.Persist && (opts.StateDir != "" || opts.ProfileDir != "") {
		return "", "", nil, errors.New("--state-dir and --profile-dir require --persist")
	}

	if opts.Persist {
		stateDir, profileDir, err = resolveDirs(opts)
		if err != nil {
			return "", "", nil, err
		}
		if err := ensureDirs(stateDir, profileDir); err != nil {
			return "", "", nil, err
		}
		return stateDir, profileDir, func() error { return nil }, nil
	}

	stateDir, err = os.MkdirTemp("", "taillaunch-state-")
	if err != nil {
		return "", "", nil, fmt.Errorf("create temporary Tailscale state directory: %w", err)
	}
	profileDir, err = os.MkdirTemp("", "taillaunch-browser-")
	if err != nil {
		cleanupErr := os.RemoveAll(stateDir)
		return "", "", nil, errors.Join(fmt.Errorf("create temporary browser profile directory: %w", err), cleanupErr)
	}
	if err := ensureDirs(stateDir, profileDir); err != nil {
		return "", "", nil, errors.Join(err, removeDirs(profileDir, stateDir))
	}

	// Each folder is locked for the life of the session so that a later
	// startup can tell it from one left behind by a crash.
	var locks []*sessionLock
	releaseLocks := func() error {
		var errs []error
		for _, lock := range locks {
			errs = append(errs, lock.Release())
		}
		return errors.Join(errs...)
	}
	for _, dir := range []string{stateDir, profileDir} {
		lock, err := lockSessionDir(dir)
		if err != nil {
			return "", "", nil, errors.Join(fmt.Errorf("lock temporary directory %q: %w", dir, err), releaseLocks(), removeDirs(profileDir, stateDir))
		}
		locks = append(locks, lock)
	}

	return stateDir, profileDir, func() error {
		// Release first: Windows cannot delete a file that is still open.
		return errors.Join(releaseLocks(), removeDirs(profileDir, stateDir))
	}, nil
}

func removeDirs(dirs ...string) error {
	var errs []error
	for _, dir := range dirs {
		if err := os.RemoveAll(dir); err != nil {
			errs = append(errs, fmt.Errorf("remove temporary directory %q: %w", dir, err))
		}
	}
	return errors.Join(errs...)
}

func ensureDirs(stateDir, profileDir string) error {
	if err := privatefs.EnsureDir(stateDir); err != nil {
		return fmt.Errorf("prepare state directory: %w", err)
	}
	if err := privatefs.EnsureDir(profileDir); err != nil {
		return fmt.Errorf("prepare browser profile directory: %w", err)
	}
	return nil
}

func resolveDirs(opts Options) (stateDir, profileDir string, err error) {
	if opts.Portable {
		exe, err := os.Executable()
		if err != nil {
			return "", "", err
		}
		base := filepath.Join(filepath.Dir(exe), "taillaunch-data")
		stateDir = opts.StateDir
		if stateDir == "" {
			stateDir = filepath.Join(base, "tailscale-state")
		}
		profileDir = opts.ProfileDir
		if profileDir == "" {
			profileDir = filepath.Join(base, "browser")
		}
		return stateDir, profileDir, nil
	}

	configDir, err := os.UserConfigDir()
	if err != nil {
		return "", "", err
	}
	base := filepath.Join(configDir, "TailLaunch")
	stateDir = opts.StateDir
	if stateDir == "" {
		stateDir = filepath.Join(base, "tailscale-state")
	}
	profileDir = opts.ProfileDir
	if profileDir == "" {
		profileDir = filepath.Join(base, "browser")
	}
	return stateDir, profileDir, nil
}
