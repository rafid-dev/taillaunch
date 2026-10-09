//go:build !stub

package app

import (
	"testing"

	"tailscale.com/ipn"
)

// The stub build can't import Tailscale, so auth.go copies these URLs. Fail if
// a Tailscale upgrade changes what counts as the official control plane.
func TestOfficialControlURLsMatchTailscale(t *testing.T) {
	if tailscaleDefaultControlURL != ipn.DefaultControlURL {
		t.Fatalf("tailscaleDefaultControlURL = %q, ipn.DefaultControlURL = %q", tailscaleDefaultControlURL, ipn.DefaultControlURL)
	}
	if !ipn.IsLoginServerSynonym(tailscaleLoginControlURL) || !ipn.IsLoginServerSynonym(tailscaleDefaultControlURL) {
		t.Fatal("ipn.IsLoginServerSynonym no longer matches the official control URLs copied into auth.go")
	}
}
