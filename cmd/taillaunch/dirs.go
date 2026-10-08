package main

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"github.com/rafid-dev/taillaunch/internal/privatefs"
)

// prepareDirs selects reusable directories only when persistence was explicitly
// requested. The default directories are private per-run directories and are
// removed by the returned cleanup function.
func prepareDirs(cfg config) (stateDir, profileDir string, cleanup func() error, err error) {
	if !cfg.persist && (cfg.stateDir != "" || cfg.profileDir != "") {
		return "", "", nil, errors.New("--state-dir and --profile-dir require --persist")
	}

	if cfg.persist {
		stateDir, profileDir, err = resolveDirs(cfg)
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

	return stateDir, profileDir, func() error { return removeDirs(profileDir, stateDir) }, nil
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

func resolveDirs(cfg config) (stateDir, profileDir string, err error) {
	if cfg.portable {
		exe, err := os.Executable()
		if err != nil {
			return "", "", err
		}
		base := filepath.Join(filepath.Dir(exe), "taillaunch-data")
		stateDir = cfg.stateDir
		if stateDir == "" {
			stateDir = filepath.Join(base, "tailscale-state")
		}
		profileDir = cfg.profileDir
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
	stateDir = cfg.stateDir
	if stateDir == "" {
		stateDir = filepath.Join(base, "tailscale-state")
	}
	profileDir = cfg.profileDir
	if profileDir == "" {
		profileDir = filepath.Join(base, "browser")
	}
	return stateDir, profileDir, nil
}
