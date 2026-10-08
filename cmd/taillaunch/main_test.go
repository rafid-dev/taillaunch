package main

import (
	"flag"
	"strings"
	"testing"
)

func TestDefaultConfigUsesAppWindow(t *testing.T) {
	if !defaultConfig().appMode {
		t.Fatal("default configuration should open a dedicated app window")
	}
	if defaultConfig().persist {
		t.Fatal("default configuration should use a temporary session")
	}
}

func TestValidateTarget(t *testing.T) {
	tests := []struct {
		name      string
		url       string
		proxyOnly bool
		wantError bool
	}{
		{name: "HTTPS target", url: "https://nas.example.ts.net"},
		{name: "uppercase HTTPS scheme", url: "HTTPS://nas.example.ts.net"},
		{name: "HTTP target", url: "http://nas"},
		{name: "proxy only without target", proxyOnly: true},
		{name: "missing target", wantError: true},
		{name: "missing scheme", url: "nas.example.ts.net", wantError: true},
		{name: "unsupported scheme", url: "file:///tmp/app", wantError: true},
		{name: "missing host", url: "https:///app", wantError: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateTarget(tt.url, tt.proxyOnly)
			if (err != nil) != tt.wantError {
				t.Fatalf("validateTarget(%q, proxyOnly=%t) error = %v, wantError %t", tt.url, tt.proxyOnly, err, tt.wantError)
			}
		})
	}
}

func TestWriteUsagePutsPrivateAppFlowBeforeAdvancedSettings(t *testing.T) {
	flags := flag.NewFlagSet("taillaunch", flag.ContinueOnError)
	flags.String("url", "", "HTTP(S) URL for the private app")
	flags.Bool("persist", false, "remember sign-in and browser data")
	flags.String("listen", "127.0.0.1:0", "loopback proxy listen address")
	var output strings.Builder
	writeUsage(&output, flags)
	got := output.String()
	if !strings.Contains(got, "Close the app window to end the session") || !strings.Contains(got, "Data is temporary unless --persist is used") {
		t.Fatalf("usage should explain the normal session flow: %q", got)
	}
	if strings.Index(got, "Options:") > strings.Index(got, "Advanced options:") || !strings.Contains(got, "-listen value") {
		t.Fatalf("usage should separate the simple launch from advanced settings: %q", got)
	}
}
