package routing

import (
	"net/netip"
	"testing"
)

func TestShouldTailnet(t *testing.T) {
	p := New(true)
	p.Update(Snapshot{
		MagicDNSSuffix: "example-tailnet.ts.net",
		PeerNames:      []string{"nas", "devbox.example-tailnet.ts.net."},
		Routes:         []netip.Prefix{netip.MustParsePrefix("10.42.0.0/16")},
	})

	tests := map[string]bool{
		"nas":                            true,
		"devbox.example-tailnet.ts.net":  true,
		"newhost.example-tailnet.ts.net": true,
		"100.100.100.100":                true,
		"fd7a:115c:a1e0::1":              true,
		"10.42.2.9":                      true,
		"printer":                        true,
		"example.com":                    false,
		"1.1.1.1":                        false,
		"localhost":                      false,
		"127.0.0.1":                      false,
		"service.some-other-tail.ts.net": true,
	}

	for host, want := range tests {
		if got := p.ShouldTailnet(host); got != want {
			t.Errorf("ShouldTailnet(%q) = %v, want %v", host, got, want)
		}
	}
}
