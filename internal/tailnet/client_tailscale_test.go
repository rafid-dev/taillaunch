//go:build !stub

package tailnet

import (
	"fmt"
	"strings"
	"testing"
)

func TestUserLogfSurfacesAuthorizationWithoutLoggingURL(t *testing.T) {
	const authURL = "https://login.tailscale.com/a/short-lived-capability"
	var opened []string
	var logs []string
	client := &tsClient{
		onAuthURL: func(rawURL string) { opened = append(opened, rawURL) },
		logf:      func(format string, args ...any) { logs = append(logs, fmt.Sprintf(format, args...)) },
	}

	client.userLogf("To start this tsnet server, go to: %s", authURL)
	client.userLogf("To start this tsnet server, go to: %s", authURL)
	client.userLogf("To start this tsnet server, go to: https://headscale.example.test/register?key=placeholder-key")

	if len(opened) != 2 || opened[0] != authURL || opened[1] != "https://headscale.example.test/register?key=placeholder-key" {
		t.Fatalf("authorization callbacks = %#v", opened)
	}
	for _, line := range logs {
		if strings.Contains(line, authURL) || strings.Contains(line, "short-lived-capability") {
			t.Fatalf("authorization URL was written to logs: %q", line)
		}
	}
}
