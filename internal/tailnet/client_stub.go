//go:build stub

package tailnet

import (
	"context"
	"net"
	"net/netip"

	"github.com/rafid-dev/taillaunch/internal/routing"
)

type stubClient struct{}

func InitProcess() {}

func New(Options) (Client, error) { return &stubClient{}, nil }
func (c *stubClient) Up(context.Context) (routing.Snapshot, error) {
	return routing.Snapshot{
		MagicDNSSuffix: "stub-tailnet.ts.net",
		PeerNames:      []string{"stub-peer"},
		Routes:         []netip.Prefix{netip.MustParsePrefix("10.99.0.0/16")},
	}, nil
}
func (c *stubClient) Snapshot(ctx context.Context) (routing.Snapshot, error) { return c.Up(ctx) }
func (c *stubClient) DialContext(ctx context.Context, network, address string) (net.Conn, error) {
	var d net.Dialer
	return d.DialContext(ctx, network, address)
}
func (c *stubClient) Close() error { return nil }
