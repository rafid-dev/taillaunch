package app

import (
	"context"
	"io"
	"log"
	"net/url"
	"strings"
	"sync"
	"sync/atomic"
)

// AuthOpener validates and opens the short-lived official Tailscale sign-in
// URL. The URL is intentionally never included in log output.
type AuthOpener struct {
	openURL  func(string) error
	logger   *log.Logger
	cancel   context.CancelFunc
	onStatus func(string)
	once     sync.Once
	failed   atomic.Bool
}

// NewAuthOpener is exported so the legacy CLI adapter can keep its existing
// focused auth tests while the GUI uses the same implementation through
// Connect.
func NewAuthOpener(openURL func(string) error, logger *log.Logger, cancel context.CancelFunc) *AuthOpener {
	if logger == nil {
		logger = log.New(io.Discard, "", 0)
	}
	return &AuthOpener{openURL: openURL, logger: logger, cancel: cancel}
}

func newAuthOpener(openURL func(string) error, logger *log.Logger, cancel context.CancelFunc, onStatus func(string)) *AuthOpener {
	a := NewAuthOpener(openURL, logger, cancel)
	a.onStatus = onStatus
	return a
}

// Open is used as tailnet.Options.OnAuthURL.
func (a *AuthOpener) Open(rawURL string) {
	u, err := url.Parse(rawURL)
	if err != nil || !strings.EqualFold(u.Scheme, "https") || u.Hostname() == "" || u.User != nil {
		a.logger.Printf("The sign-in page address could not be validated, so TailLaunch did not open it.")
		a.stopStartup()
		return
	}

	a.once.Do(func() {
		if a.onStatus != nil {
			a.onStatus("Finish signing in in your normal browser; TailLaunch will continue automatically.")
		}
		a.logger.Printf("The official Tailscale sign-in page is open in your normal browser. Finish signing in there; TailLaunch will continue automatically.")
		if err := a.openURL(u.String()); err != nil {
			a.logger.Printf("TailLaunch couldn't open the sign-in page automatically. Check your normal browser and try again.")
			a.stopStartup()
		}
	})
}

func (a *AuthOpener) stopStartup() {
	a.failed.Store(true)
	if a.cancel != nil {
		a.cancel()
	}
}

func (a *AuthOpener) Failed() bool { return a.failed.Load() }
