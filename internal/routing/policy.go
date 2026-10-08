package routing

import (
	"net/netip"
	"strings"
	"sync"
)

var (
	tailscaleIPv4 = netip.MustParsePrefix("100.64.0.0/10")
	tailscaleIPv6 = netip.MustParsePrefix("fd7a:115c:a1e0::/48")
)

// Snapshot is the routing information learned from the embedded Tailscale node.
type Snapshot struct {
	MagicDNSSuffix string
	PeerNames      []string
	Routes         []netip.Prefix
}

// Policy decides whether a destination should be dialed through the tailnet or
// directly through the host network.
type Policy struct {
	mu       sync.RWMutex
	suffix   string
	peers    map[string]struct{}
	routes   []netip.Prefix
	shortDNS bool
}

func New(shortDNS bool) *Policy {
	return &Policy{peers: make(map[string]struct{}), shortDNS: shortDNS}
}

func (p *Policy) Update(s Snapshot) {
	peers := make(map[string]struct{}, len(s.PeerNames))
	for _, name := range s.PeerNames {
		name = normalizeHost(name)
		if name != "" {
			peers[name] = struct{}{}
		}
	}

	p.mu.Lock()
	p.suffix = strings.TrimPrefix(normalizeHost(s.MagicDNSSuffix), ".")
	p.peers = peers
	p.routes = append(p.routes[:0], s.Routes...)
	p.mu.Unlock()
}

func (p *Policy) ShouldTailnet(host string) bool {
	host = normalizeHost(host)
	if host == "" || host == "localhost" || strings.HasSuffix(host, ".localhost") {
		return false
	}

	if ip, err := netip.ParseAddr(strings.Trim(host, "[]")); err == nil {
		if ip.IsLoopback() {
			return false
		}
		if tailscaleIPv4.Contains(ip) || tailscaleIPv6.Contains(ip) {
			return true
		}

		p.mu.RLock()
		defer p.mu.RUnlock()
		for _, prefix := range p.routes {
			if prefix.Contains(ip) {
				return true
			}
		}
		return false
	}

	// *.ts.net names are Tailscale-issued names. Routing them through tsnet is
	// harmless even if the current tailnet cannot resolve/reach them.
	if host == "ts.net" || strings.HasSuffix(host, ".ts.net") {
		return true
	}

	p.mu.RLock()
	defer p.mu.RUnlock()

	if _, ok := p.peers[host]; ok {
		return true
	}
	if p.suffix != "" && (host == p.suffix || strings.HasSuffix(host, "."+p.suffix)) {
		return true
	}

	// MagicDNS supports single-label peer names. A dedicated TailLaunch browser
	// is expected to prefer tailnet resolution for such names.
	return p.shortDNS && !strings.Contains(host, ".")
}

func normalizeHost(host string) string {
	host = strings.TrimSpace(strings.ToLower(host))
	host = strings.TrimSuffix(host, ".")
	return strings.Trim(host, "[]")
}
