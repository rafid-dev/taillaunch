package main

import (
	"bytes"
	"context"
	"errors"
	"log"
	"strings"
	"testing"
)

func TestAuthOpenerOpensSignInURLOnceWithoutLoggingIt(t *testing.T) {
	const authURL = "https://login.tailscale.com/a/short-lived-capability"
	var opened []string
	var output bytes.Buffer
	opener := newAuthOpener(func(rawURL string) error {
		opened = append(opened, rawURL)
		return nil
	}, log.New(&output, "", 0), func() {})

	opener.Open(authURL)
	opener.Open(authURL)

	if len(opened) != 1 || opened[0] != authURL {
		t.Fatalf("opened URLs = %#v, want one direct open of the Tailscale URL", opened)
	}
	if !strings.Contains(output.String(), "official Tailscale sign-in page") || !strings.Contains(output.String(), "continue automatically") {
		t.Fatalf("status output = %q, want clear sign-in and continuation guidance", output.String())
	}
	if opener.Failed() {
		t.Fatal("successful browser open should not mark sign-in as failed")
	}
	if strings.Contains(output.String(), "short-lived-capability") {
		t.Fatalf("sign-in capability was written to logs: %q", output.String())
	}
}

func TestAuthOpenerReportsBrowserOpenFailure(t *testing.T) {
	var output bytes.Buffer
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	opener := newAuthOpener(func(string) error { return errors.New("test open failure") }, log.New(&output, "", 0), cancel)
	opener.Open("https://login.tailscale.com/a/token")

	if !strings.Contains(output.String(), "couldn't open the sign-in page automatically") {
		t.Fatalf("status output = %q, want browser open failure", output.String())
	}
	if ctx.Err() == nil || !opener.Failed() {
		t.Fatal("failed browser open should stop connection setup")
	}
}

func TestAuthOpenerRejectsNonHTTPSAndCredentialURLs(t *testing.T) {
	for _, rawURL := range []string{
		"http://login.tailscale.com/a/token",
		"https://user:password@login.tailscale.com/a/token",
		"not a URL",
	} {
		t.Run(rawURL, func(t *testing.T) {
			var output bytes.Buffer
			opened := false
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			opener := newAuthOpener(func(string) error {
				opened = true
				return nil
			}, log.New(&output, "", 0), cancel)
			opener.Open(rawURL)
			if opened {
				t.Fatal("invalid sign-in URL was opened")
			}
			if !strings.Contains(output.String(), "did not open it") {
				t.Fatalf("status output = %q, want invalid URL feedback", output.String())
			}
			if ctx.Err() == nil || !opener.Failed() {
				t.Fatal("invalid sign-in address should stop connection setup")
			}
		})
	}
}
