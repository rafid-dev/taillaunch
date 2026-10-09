package app

import (
	"bytes"
	"context"
	"log"
	"strings"
	"testing"
)

// authOutcome runs rawURL through an AuthOpener configured for controlURL and
// reports whether the URL was opened and what was logged.
func authOutcome(t *testing.T, controlURL, rawURL string) (opened bool, output string) {
	t.Helper()
	var buf bytes.Buffer
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	a := newAuthOpener(func(string) error {
		opened = true
		return nil
	}, log.New(&buf, "", 0), cancel, controlURL, nil)
	a.Open(rawURL)
	if !opened {
		if !a.Failed() || ctx.Err() == nil {
			t.Errorf("rejected %q must fail and cancel startup", rawURL)
		}
		if !strings.Contains(buf.String(), "did not open it") {
			t.Errorf("rejected %q: output = %q, want rejection feedback", rawURL, buf.String())
		}
	}
	return opened, buf.String()
}

// Mirrors Tailscale v1.102.5 ipn/ipnlocal TestValidPopBrowserURL: with the
// official control plane only tailscale.com and its subdomains are allowed.
func TestAuthOpenerOfficialControlServerRequiresTailscaleHost(t *testing.T) {
	const (
		unset        = ""
		defaultURL   = "https://controlplane.tailscale.com"
		loginSynonym = "https://login.tailscale.com"
	)
	tests := []struct {
		name       string
		controlURL string
		rawURL     string
		want       bool
	}{
		{"login subdomain", unset, "https://login.tailscale.com/a/token", true},
		{"apex domain", unset, "https://tailscale.com/a/token", true},
		{"other real subdomain", unset, "https://controlplane.tailscale.com/a/token", true},
		{"nested subdomain", unset, "https://a.b.tailscale.com/a/token", true},
		{"suffix lookalike subdomain", unset, "https://tailscale.com.evil.example/a/token", false},
		{"prefix lookalike", unset, "https://eviltailscale.com/a/token", false},
		{"dash lookalike", unset, "https://evil-tailscale.com/a/token", false},
		{"unrelated host", unset, "https://example.com/a/token", false},
		{"tailscale.com in path only", unset, "https://example.com/tailscale.com", false},
		{"tailscale.com in query only", unset, "https://example.com/?next=login.tailscale.com", false},
		{"empty label before suffix", unset, "https://.tailscale.com/a/token", false},

		// Hostname comparison is case-insensitive.
		{"mixed-case subdomain", unset, "https://LoGiN.TaIlScAlE.CoM/a/token", true},
		{"mixed-case apex", unset, "https://Tailscale.COM/a/token", true},
		{"mixed-case lookalike", unset, "https://EvilTailscale.com/a/token", false},

		// Ports never affect the hostname decision.
		{"explicit default port", unset, "https://login.tailscale.com:443/a/token", true},
		{"non-default port", unset, "https://login.tailscale.com:8443/a/token", true},
		{"lookalike with default port", unset, "https://tailscale.com.evil.example:443/a/token", false},
		{"lookalike with port", unset, "https://eviltailscale.com:8443/a/token", false},
		{"port-like userinfo before tailscale.com", unset, "https://evil.example:443@tailscale.com/a/token", false},

		// Trailing-dot hostnames are rejected, as Tailscale's own check does
		// (its host comparison is exact, so "login.tailscale.com." matches
		// neither "tailscale.com" nor the ".tailscale.com" suffix).
		{"trailing-dot subdomain", unset, "https://login.tailscale.com./a/token", false},
		{"trailing-dot apex", unset, "https://tailscale.com./a/token", false},
		{"trailing-dot with port", unset, "https://login.tailscale.com.:443/a/token", false},

		// Remaining baseline checks.
		{"http", unset, "http://login.tailscale.com/a/token", false},
		{"missing host", unset, "https:///a/token", false},
		{"userinfo", unset, "https://user:placeholder@login.tailscale.com/a/token", false},
		{"username only", unset, "https://user@login.tailscale.com/a/token", false},
		{"not a URL", unset, "not a URL", false},

		// Explicitly configuring Tailscale's own control URL (or its login
		// synonym, which Tailscale treats identically) is the same as unset.
		{"explicit default accepts tailscale host", defaultURL, "https://login.tailscale.com/a/token", true},
		{"explicit default rejects lookalike", defaultURL, "https://tailscale.com.evil.example/a/token", false},
		{"explicit default rejects prefix lookalike", defaultURL, "https://eviltailscale.com/a/token", false},
		{"explicit default rejects other host", defaultURL, "https://auth.example.com/a/token", false},
		{"explicit login synonym rejects other host", loginSynonym, "https://auth.example.com/a/token", false},
		{"explicit login synonym accepts tailscale host", loginSynonym, "https://login.tailscale.com/a/token", true},
		{"explicit default with spaces and slash", " https://controlplane.tailscale.com/ ", "https://auth.example.com/a/token", false},
		{"explicit default in mixed case", "HTTPS://ControlPlane.Tailscale.com", "https://auth.example.com/a/token", false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, _ := authOutcome(t, tt.controlURL, tt.rawURL)
			if got != tt.want {
				t.Fatalf("control %q, auth URL %q: opened = %v, want %v", tt.controlURL, tt.rawURL, got, tt.want)
			}
		})
	}
}

// Tailscale v1.102.5 deliberately allows any host for the sign-in URL when a
// custom ControlURL is used (alternate control servers may use an external
// authentication host), so TailLaunch must not require the hosts to match.
func TestAuthOpenerCustomControlServerMayUseAnotherAuthHost(t *testing.T) {
	const control = "https://headscale.example.test"
	tests := []struct {
		name   string
		rawURL string
		want   bool
	}{
		{"same host", "https://headscale.example.test/register/key", true},
		{"same host with port", "https://headscale.example.test:8443/register/key", true},
		{"different HTTPS auth host", "https://auth.idp.example.net/login?state=placeholder", true},
		{"tailscale host is fine too", "https://login.tailscale.com/a/token", true},
		{"lookalike host is not special-cased", "https://eviltailscale.com/a/token", true},

		// HTTPS, a host, and no userinfo are still required.
		{"http same host", "http://headscale.example.test/register/key", false},
		{"http other host", "http://auth.idp.example.net/login", false},
		{"missing host", "https:///register/key", false},
		{"userinfo same host", "https://user:placeholder@headscale.example.test/register/key", false},
		{"userinfo other host", "https://user@auth.idp.example.net/login", false},
		{"not a URL", "not a URL", false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, _ := authOutcome(t, control, tt.rawURL)
			if got != tt.want {
				t.Fatalf("auth URL %q: opened = %v, want %v", tt.rawURL, got, tt.want)
			}
		})
	}
}

func TestAuthOpenerStatusNamesTheTrustedSource(t *testing.T) {
	_, official := authOutcome(t, "", "https://login.tailscale.com/a/token")
	if !strings.Contains(official, "official Tailscale sign-in page") {
		t.Fatalf("default control server output = %q, want official Tailscale wording", official)
	}
	_, custom := authOutcome(t, "https://headscale.example.test", "https://auth.idp.example.net/login")
	if strings.Contains(custom, "official Tailscale") || !strings.Contains(custom, "control server") {
		t.Fatalf("custom control server output = %q, want control-server wording without claiming it is the official Tailscale page", custom)
	}
	if strings.Contains(custom, "auth.idp.example.net") {
		t.Fatalf("sign-in host was written to logs: %q", custom)
	}
}
