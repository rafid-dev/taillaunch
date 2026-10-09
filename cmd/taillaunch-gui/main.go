package main

import (
	"github.com/rafid-dev/taillaunch/internal/launcherui"
	"github.com/rafid-dev/taillaunch/internal/tailnet"
)

var version = "dev"

func main() {
	// Must be first: sets a process-wide Tailscale knob before any goroutines start.
	tailnet.InitProcess()
	if err := launcherui.Run(version); err != nil {
		panic(err)
	}
}
