package app

import (
	"context"
	"errors"
	"log"
	"net"
	"strings"
	"testing"

	"github.com/rafid-dev/taillaunch/internal/routing"
	"github.com/rafid-dev/taillaunch/internal/tailnet"
)

func TestNormalizeControlURL(t *testing.T) {
	tests := []struct {
		name    string
		raw     string
		wantErr string // substring of the error; empty means valid
	}{
		{name: "empty means default control plane", raw: ""},
		{name: "whitespace only means default control plane", raw: "  \t "},
		{name: "valid https", raw: "https://headscale.example.com"},
		{name: "valid https with port and path", raw: "https://headscale.example.com:8443/base"},
		{name: "uppercase HTTPS", raw: "HTTPS://headscale.example.com"},
		{name: "surrounding whitespace", raw: "  https://headscale.example.com\t\n"},
		{name: "http", raw: "http://headscale.example.com", wantErr: "must use https://"},
		{name: "missing scheme", raw: "headscale.example.com", wantErr: "must use https://"},
		{name: "missing scheme with port", raw: "headscale.example.com:8080", wantErr: "control URL"},
		{name: "other scheme", raw: "ftp://headscale.example.com", wantErr: "must use https://"},
		{name: "missing host", raw: "https:///path", wantErr: "must include a host"},
		{name: "missing host with port", raw: "https://:8443", wantErr: "must include a host"},
		{name: "no authority", raw: "https:headscale.example.com", wantErr: "must include a host"},
		{name: "userinfo", raw: "https://user:pass@headscale.example.com", wantErr: "must not include a username or password"},
		{name: "username only", raw: "https://user@headscale.example.com", wantErr: "must not include a username or password"},
		{name: "query", raw: "https://headscale.example.com/?a=b", wantErr: "must not include a query"},
		{name: "empty query", raw: "https://headscale.example.com/?", wantErr: "must not include a query"},
		{name: "fragment", raw: "https://headscale.example.com/#frag", wantErr: "must not include a fragment"},
		{name: "empty fragment", raw: "https://headscale.example.com/#", wantErr: "must not include a fragment"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := NormalizeControlURL(tt.raw)
			if tt.wantErr == "" {
				if err != nil {
					t.Fatalf("NormalizeControlURL(%q) error = %v, want nil", tt.raw, err)
				}
				if want := strings.TrimSpace(tt.raw); got != want {
					t.Fatalf("NormalizeControlURL(%q) = %q, want the trimmed value %q", tt.raw, got, want)
				}
				return
			}
			if err == nil {
				t.Fatalf("NormalizeControlURL(%q) = nil error, want error containing %q", tt.raw, tt.wantErr)
			}
			if !strings.Contains(err.Error(), tt.wantErr) {
				t.Fatalf("NormalizeControlURL(%q) error = %q, want it to contain %q", tt.raw, err, tt.wantErr)
			}
			if got != "" {
				t.Fatalf("NormalizeControlURL(%q) = %q alongside an error, want empty", tt.raw, got)
			}
		})
	}
}

// A bad control URL must be rejected by Connect before any session state is
// created, for both frontends.
func TestConnectRejectsInvalidControlURL(t *testing.T) {
	var statuses []Status
	s, err := Connect(context.Background(), Options{
		ControlURL: "http://headscale.example.com",
		ProxyOnly:  true,
	}, Hooks{Notify: func(st Status, _ string) { statuses = append(statuses, st) }})
	if s != nil {
		_ = s.Close()
	}
	if err == nil || !strings.Contains(err.Error(), "must use https://") {
		t.Fatalf("Connect error = %v, want the control URL error", err)
	}
	for _, st := range statuses {
		if st == StatusStarting {
			t.Fatalf("Connect started a session before validating the control URL: %v", statuses)
		}
	}
}

// failingClient stands in for the Tailscale client so Connect stops right
// after handing it the options under test.
type failingClient struct{}

func (failingClient) Up(context.Context) (routing.Snapshot, error) {
	return routing.Snapshot{}, errors.New("stop here")
}
func (failingClient) Snapshot(context.Context) (routing.Snapshot, error) {
	return routing.Snapshot{}, errors.New("stop here")
}
func (failingClient) DialContext(context.Context, string, string) (net.Conn, error) {
	return nil, errors.New("stop here")
}
func (failingClient) Close() error { return nil }

func TestConnectUsesTrimmedControlURL(t *testing.T) {
	const padded = "  https://headscale.example.com  "
	const want = "https://headscale.example.com"

	var authGot, tailnetGot string
	origAuth, origClient := makeAuthOpener, newTailnetClient
	t.Cleanup(func() { makeAuthOpener, newTailnetClient = origAuth, origClient })
	makeAuthOpener = func(openURL func(string) error, logger *log.Logger, cancel context.CancelFunc, controlURL string, onStatus func(string)) *AuthOpener {
		authGot = controlURL
		return origAuth(openURL, logger, cancel, controlURL, onStatus)
	}
	newTailnetClient = func(opts tailnet.Options) (tailnet.Client, error) {
		tailnetGot = opts.ControlURL
		return failingClient{}, nil
	}

	s, err := Connect(context.Background(), Options{ControlURL: padded, ProxyOnly: true}, Hooks{})
	if s != nil {
		_ = s.Close()
	}
	if err == nil {
		t.Fatal("Connect succeeded, but the fake client always fails to come up")
	}
	if authGot != want {
		t.Errorf("auth opener got control URL %q, want %q", authGot, want)
	}
	if tailnetGot != want {
		t.Errorf("tailnet.New got control URL %q, want %q", tailnetGot, want)
	}
}
