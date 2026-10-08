//go:build !stub

package tailnet

import (
	"context"
	"fmt"
	"net"
	"regexp"
	"strings"
	"sync"

	"github.com/rafid-dev/taillaunch/internal/routing"
	"tailscale.com/ipn/ipnstate"
	"tailscale.com/tsnet"
)

var authURLPattern = regexp.MustCompile(`https?://[^\s]+`)

type tsClient struct {
	srv         *tsnet.Server
	onAuthURL   func(string)
	logf        func(string, ...any)
	authURLMu   sync.Mutex
	lastAuthURL string
}

func New(opts Options) (Client, error) {
	c := &tsClient{onAuthURL: opts.OnAuthURL, logf: opts.Logf}
	c.srv = &tsnet.Server{
		Hostname:   opts.Hostname,
		Dir:        opts.StateDir,
		ControlURL: opts.ControlURL,
		Ephemeral:  opts.Ephemeral,
		UserLogf:   c.userLogf,
	}
	if opts.Verbose {
		c.srv.Logf = func(format string, args ...any) { c.log(format, args...) }
	}
	return c, nil
}

func (c *tsClient) Up(ctx context.Context) (routing.Snapshot, error) {
	status, err := c.srv.Up(ctx)
	if err != nil {
		return routing.Snapshot{}, err
	}
	return snapshotFromStatus(status), nil
}

func (c *tsClient) Snapshot(ctx context.Context) (routing.Snapshot, error) {
	lc, err := c.srv.LocalClient()
	if err != nil {
		return routing.Snapshot{}, err
	}
	status, err := lc.Status(ctx)
	if err != nil {
		return routing.Snapshot{}, err
	}
	return snapshotFromStatus(status), nil
}

func (c *tsClient) DialContext(ctx context.Context, network, address string) (net.Conn, error) {
	return c.srv.Dial(ctx, network, address)
}

func (c *tsClient) Close() error { return c.srv.Close() }

func (c *tsClient) userLogf(format string, args ...any) {
	msg := fmt.Sprintf(format, args...)
	lower := strings.ToLower(msg)
	if strings.Contains(lower, "auth") || strings.Contains(lower, "login") || strings.Contains(lower, "to start this tsnet server") {
		url := strings.TrimRight(authURLPattern.FindString(msg), ".,;)")
		if url != "" {
			if c.onAuthURL != nil {
				c.authURLMu.Lock()
				if url != c.lastAuthURL {
					c.lastAuthURL = url
					c.authURLMu.Unlock()
					c.onAuthURL(url)
				} else {
					c.authURLMu.Unlock()
				}
			}
			// Auth URLs are short-lived authorization capabilities. Keep them
			// out of console logs and expose the URL only through the UI callback.
			c.log("Tailscale authorization is required")
			return
		}
	}
	c.log("%s", msg)
}

func (c *tsClient) log(format string, args ...any) {
	if c.logf != nil {
		c.logf(format, args...)
	}
}

func snapshotFromStatus(status *ipnstate.Status) routing.Snapshot {
	var snap routing.Snapshot
	if status == nil {
		return snap
	}
	if status.CurrentTailnet != nil {
		snap.MagicDNSSuffix = status.CurrentTailnet.MagicDNSSuffix
	} else {
		snap.MagicDNSSuffix = status.MagicDNSSuffix
	}

	seenNames := make(map[string]struct{})
	for _, peer := range status.Peer {
		if peer == nil {
			continue
		}
		for _, name := range []string{peer.HostName, strings.TrimSuffix(peer.DNSName, ".")} {
			name = strings.TrimSpace(name)
			if name == "" {
				continue
			}
			if _, ok := seenNames[name]; !ok {
				seenNames[name] = struct{}{}
				snap.PeerNames = append(snap.PeerNames, name)
			}
		}
		if peer.PrimaryRoutes != nil {
			snap.Routes = append(snap.Routes, peer.PrimaryRoutes.AsSlice()...)
		}
	}
	return snap
}
