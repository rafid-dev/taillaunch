package main

import "github.com/rafid-dev/taillaunch/internal/launcherui"

var version = "dev"

func main() {
	if err := launcherui.Run(version); err != nil {
		panic(err)
	}
}
