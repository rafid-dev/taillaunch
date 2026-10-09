//go:build !stub

package tailnet

import (
	"context"
	"errors"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"sync"

	"github.com/rafid-dev/taillaunch/internal/routing"
	"tailscale.com/envknob"
	"tailscale.com/ipn/ipnstate"
	"tailscale.com/logtail"
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

// tsnetLogFiles are the files tsnet's startLogger creates in the state
// directory (tailscale.com v1.102.5, tsnet/tsnet.go): the logtail config
// "tailscaled.log.conf" and the filch disk buffer "tailscaled" + ".log1.txt" /
// ".log2.txt" (logtail/filch/filch.go). Keep tailscaled.state out of this list.
var tsnetLogFiles = []string{
	"tailscaled.log.conf",
	"tailscaled.log1.txt",
	"tailscaled.log2.txt",
}

// removeTsnetLogFiles deletes tsnet's on-disk log files from dir. Only regular
// files with the exact names in tsnetLogFiles are removed.
func removeTsnetLogFiles(dir string) error {
	if dir == "" {
		return nil
	}
	var errs []error
	for _, name := range tsnetLogFiles {
		path := filepath.Join(dir, name)
		fi, err := os.Lstat(path)
		if err != nil {
			if !os.IsNotExist(err) {
				errs = append(errs, err)
			}
			continue
		}
		if !fi.Mode().IsRegular() {
			continue
		}
		if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
			errs = append(errs, err)
		}
	}
	return errors.Join(errs...)
}

func New(opts Options) (Client, error) {
	// tsnet always starts logtail, which uploads logs (including the sign-in
	// URL) to log.tailscale.com even with a custom ControlURL. Turn it off
	// before the server starts, and drop any logs written by earlier versions.
	logtail.Disable()
	envknob.Setenv("TS_NO_LOGS_NO_SUPPORT", "true")
	if err := removeTsnetLogFiles(opts.StateDir); err != nil && opts.Logf != nil {
		opts.Logf("could not remove old Tailscale log files: %v", err)
	}

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
			// out of console logs and expose the URL only through the UI
			// callback. New also disables tsnet's logtail upload and local log
			// buffer, so the URL is not sent to or stored by Tailscale logging.
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
