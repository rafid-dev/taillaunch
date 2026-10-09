//go:build !stub

package app

import (
	"testing"

	"tailscale.com/ipn"
)

// The stub build can't import Tailscale, so auth.go copies these URLs. Fail if
// a Tailscale upgrade changes what counts as the official control plane.
func TestOfficialControlURLsMatchTailscale(t *testing.T) {
	defaultURL := "https://" + tailscaleDefaultControlHost
	loginURL := "https://" + tailscaleLoginControlHost
	if defaultURL != ipn.DefaultControlURL {
		t.Fatalf("default control URL = %q, ipn.DefaultControlURL = %q", defaultURL, ipn.DefaultControlURL)
	}
	if !ipn.IsLoginServerSynonym(loginURL) || !ipn.IsLoginServerSynonym(defaultURL) {
		t.Fatal("ipn.IsLoginServerSynonym no longer matches the official control URLs copied into auth.go")
	}
}
