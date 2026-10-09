package app

import (
	"errors"
	"fmt"
	"net/url"
	"strings"
)

// ValidateControlURL checks a custom control server URL before any session
// starts. Empty (after trimming whitespace) selects the default Tailscale
// control plane and is valid. Otherwise the URL must be an absolute https URL
// with a host and no userinfo, query, or fragment; TailLaunch only opens https
// sign-in pages, so an http control server could never complete sign-in.
func ValidateControlURL(raw string) error {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return nil
	}
	u, err := url.Parse(raw)
	if err != nil {
		return fmt.Errorf("control URL is not a valid URL: %w", err)
	}
	if !strings.EqualFold(u.Scheme, "https") {
		return errors.New("control URL must use https://")
	}
	if u.Hostname() == "" {
		return errors.New("control URL must include a host")
	}
	if u.User != nil {
		return errors.New("control URL must not include a username or password")
	}
	// Check the raw text: Go drops an empty "?" or "#" from the parsed URL.
	if i := strings.IndexAny(raw, "?#"); i >= 0 {
		if raw[i] == '?' {
			return errors.New("control URL must not include a query")
		}
		return errors.New("control URL must not include a fragment")
	}
	return nil
}
