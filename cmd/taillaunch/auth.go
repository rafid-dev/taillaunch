package main

import (
	"context"
	"log"
	"net/url"
	"strings"
	"sync"
	"sync/atomic"
)

type authOpener struct {
	openURL func(string) error
	logger  *log.Logger
	cancel  context.CancelFunc
	once    sync.Once
	failed  atomic.Bool
}

func newAuthOpener(openURL func(string) error, logger *log.Logger, cancel context.CancelFunc) *authOpener {
	return &authOpener{openURL: openURL, logger: logger, cancel: cancel}
}

func (a *authOpener) Open(rawURL string) {
	u, err := url.Parse(rawURL)
	if err != nil || !strings.EqualFold(u.Scheme, "https") || u.Hostname() == "" || u.User != nil {
		a.logger.Printf("The sign-in page address could not be validated, so TailLaunch did not open it.")
		a.stopStartup()
		return
	}

	a.once.Do(func() {
		a.logger.Printf("The official Tailscale sign-in page is open in your normal browser. Finish signing in there; TailLaunch will continue automatically.")
		if err := a.openURL(u.String()); err != nil {
			a.logger.Printf("TailLaunch couldn't open the sign-in page automatically. Check your normal browser and try again.")
			a.stopStartup()
		}
	})
}

func (a *authOpener) stopStartup() {
	a.failed.Store(true)
	a.cancel()
}

func (a *authOpener) Failed() bool { return a.failed.Load() }
