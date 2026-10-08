package main

import (
	"context"
	"log"

	"github.com/rafid-dev/taillaunch/internal/app"
)

type authOpener = app.AuthOpener

func newAuthOpener(openURL func(string) error, logger *log.Logger, cancel context.CancelFunc) *authOpener {
	return app.NewAuthOpener(openURL, logger, cancel)
}
