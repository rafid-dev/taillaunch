package tailnet

import (
	"context"
	"net"

	"github.com/rafid-dev/taillaunch/internal/routing"
)

type Options struct {
	Hostname   string
	StateDir   string
	ControlURL string
	Ephemeral  bool
	Verbose    bool
	OnAuthURL  func(string)
	Logf       func(format string, args ...any)
}

type Client interface {
	Up(context.Context) (routing.Snapshot, error)
	Snapshot(context.Context) (routing.Snapshot, error)
	DialContext(context.Context, string, string) (net.Conn, error)
	Close() error
}
