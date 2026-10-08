package main

import "github.com/rafid-dev/taillaunch/internal/app"

// prepareDirs selects reusable directories only when persistence was explicitly
// requested. The default directories are private per-run directories and are
// removed by the returned cleanup function.
func prepareDirs(cfg config) (stateDir, profileDir string, cleanup func() error, err error) {
	return app.PrepareDirs(app.Options{
		StateDir:   cfg.stateDir,
		ProfileDir: cfg.profileDir,
		Portable:   cfg.portable,
		Persist:    cfg.persist,
	})
}
