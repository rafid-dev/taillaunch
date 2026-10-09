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

// Tailscale's hosted control plane, as defined by tailscale.com v1.102.5:
// ipn.DefaultControlURL and the one legacy name ipn.IsLoginServerSynonym treats
// as equivalent. They are copied here so the stub build does not need the
// Tailscale module; a !stub test checks they still match upstream.
const (
	tailscaleDefaultControlURL = "https://controlplane.tailscale.com"
	tailscaleLoginControlURL   = "https://login.tailscale.com"
)

// AuthOpener validates and opens the short-lived sign-in URL sent by the
// control server. The URL is intentionally never included in log output.
//
// With the default Tailscale control plane (an unset ControlURL or one of
// Tailscale's own control URLs) only https URLs on tailscale.com or its
// subdomains are opened, matching Tailscale's validPopBrowserURL. With a
// custom ControlURL the control server decides where authentication happens,
// and Tailscale itself accepts any host there because alternate servers may
// use an external identity provider. TailLaunch therefore only requires https,
// a host, and no userinfo for custom servers, which puts that server's
// authentication flow inside the user's trust boundary.
type AuthOpener struct {
	openURL  func(string) error
	logger   *log.Logger
	cancel   context.CancelFunc
	onStatus func(string)
	official bool
	once     sync.Once
	failed   atomic.Bool
}

// NewAuthOpener is exported so the legacy CLI adapter can keep its existing
// focused auth tests while the GUI uses the same implementation through
// Connect.
func NewAuthOpener(openURL func(string) error, logger *log.Logger, cancel context.CancelFunc, controlURL string) *AuthOpener {
	if logger == nil {
		logger = log.New(io.Discard, "", 0)
	}
	return &AuthOpener{
		openURL:  openURL,
		logger:   logger,
		cancel:   cancel,
		official: isOfficialControlURL(controlURL),
	}
}

// isOfficialControlURL reports whether controlURL selects Tailscale's hosted
// control plane: empty, or one of the URLs Tailscale treats as the default.
// Upstream compares these strings exactly; this also ignores case, surrounding
// space and trailing slashes so a cosmetic variant of Tailscale's own URL can't
// fall into the permissive custom-server path.
func isOfficialControlURL(controlURL string) bool {
	normalized := strings.ToLower(strings.TrimRight(strings.TrimSpace(controlURL), "/"))
	switch normalized {
	case "", tailscaleDefaultControlURL, tailscaleLoginControlURL:
		return true
	}
	return false
}

// isTailscaleHost reports whether host, a parsed hostname with any port
// removed, is tailscale.com or a subdomain of it. The comparison is
// case-insensitive and exact: lookalikes such as eviltailscale.com and
// tailscale.com.evil.example do not match. A trailing-dot hostname
// ("login.tailscale.com.") is rejected, as in Tailscale's own check.
func isTailscaleHost(host string) bool {
	host = strings.ToLower(host)
	if host == "tailscale.com" {
		return true
	}
	return strings.HasSuffix(host, ".tailscale.com") && host != ".tailscale.com"
}

// validSignInURL applies the baseline checks (https, a host, no userinfo) and,
// for the official control plane, the Tailscale host restriction.
func (a *AuthOpener) validSignInURL(rawURL string) (*url.URL, bool) {
	u, err := url.Parse(rawURL)
	if err != nil || !strings.EqualFold(u.Scheme, "https") || u.Hostname() == "" || u.User != nil {
		return nil, false
	}
	if a.official && !isTailscaleHost(u.Hostname()) {
		return nil, false
	}
	return u, true
}

func newAuthOpener(openURL func(string) error, logger *log.Logger, cancel context.CancelFunc, controlURL string, onStatus func(string)) *AuthOpener {
	a := NewAuthOpener(openURL, logger, cancel, controlURL)
	a.onStatus = onStatus
	return a
}

// Open is used as tailnet.Options.OnAuthURL.
func (a *AuthOpener) Open(rawURL string) {
	u, ok := a.validSignInURL(rawURL)
	if !ok {
		a.logger.Printf("The sign-in page address could not be validated, so TailLaunch did not open it.")
		a.stopStartup()
		return
	}

	a.once.Do(func() {
		if a.onStatus != nil {
			a.onStatus("Finish signing in in your normal browser; TailLaunch will continue automatically.")
		}
		if a.official {
			a.logger.Printf("The official Tailscale sign-in page is open in your normal browser. Finish signing in there; TailLaunch will continue automatically.")
		} else {
			a.logger.Printf("The sign-in page from your control server is open in your normal browser. Finish signing in there; TailLaunch will continue automatically.")
		}
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
