package browser

import (
	"errors"
	"reflect"
	"strings"
	"testing"
)

func TestBuildArgsUsesDedicatedProfileAndProxy(t *testing.T) {
	args := buildArgs(Options{
		ProfileDir: "C:\\Users\\tester\\TailLaunch\\browser profile",
		ProxyAddr:  "127.0.0.1:43127",
		URL:        "https://nas.example.ts.net/",
		LowMemory:  true,
		Incognito:  true,
	})
	want := []string{
		"--user-data-dir=C:\\Users\\tester\\TailLaunch\\browser profile",
		"--proxy-server=http://127.0.0.1:43127",
		"--no-first-run",
		"--no-default-browser-check",
		"--disable-background-mode",
		"--process-per-site",
		"--renderer-process-limit=2",
		"--disk-cache-size=67108864",
		"--media-cache-size=33554432",
		"--incognito",
		"https://nas.example.ts.net/",
	}
	if !reflect.DeepEqual(args, want) {
		t.Fatalf("buildArgs() = %#v, want %#v", args, want)
	}
}

func TestBuildArgsDefaultsToBlankPage(t *testing.T) {
	args := buildArgs(Options{ProfileDir: "profile", ProxyAddr: "127.0.0.1:1234", AppMode: true})
	if got, want := args[len(args)-1], "--app=about:blank"; got != want {
		t.Fatalf("last argument = %q, want %q", got, want)
	}
}

func TestBuildArgsUsesAppWindowWhenRequested(t *testing.T) {
	args := buildArgs(Options{
		ProfileDir: "profile",
		ProxyAddr:  "127.0.0.1:1234",
		URL:        "https://nas.example.ts.net/",
		AppMode:    true,
	})
	if got, want := args[len(args)-1], "--app=https://nas.example.ts.net/"; got != want {
		t.Fatalf("last argument = %q, want %q", got, want)
	}
}

func TestResolveCandidatesUsesFirstAvailableBrowser(t *testing.T) {
	var tried []string
	got, err := resolveCandidates([]string{"edge", "chrome", "brave"}, func(name string) (string, error) {
		tried = append(tried, name)
		if name == "chrome" {
			return `C:\Browsers\chrome.exe`, nil
		}
		return "", errors.New("not installed")
	})
	if err != nil {
		t.Fatal(err)
	}
	if got != `C:\Browsers\chrome.exe` {
		t.Fatalf("resolved browser = %q", got)
	}
	if !reflect.DeepEqual(tried, []string{"edge", "chrome"}) {
		t.Fatalf("candidate search order = %#v", tried)
	}
}

func TestResolveCandidatesReturnsActionableFailure(t *testing.T) {
	_, err := resolveCandidates([]string{"edge", "chrome"}, func(string) (string, error) {
		return "", errors.New("not installed")
	})
	if err == nil {
		t.Fatal("expected a missing browser error")
	}
	for _, want := range []string{"TailLaunch", "Microsoft Edge", "Google Chrome", "Install", "try again"} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("error %q does not contain %q", err, want)
		}
	}
}
