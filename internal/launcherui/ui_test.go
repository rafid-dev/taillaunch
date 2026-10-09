package launcherui

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/rafid-dev/taillaunch/internal/app"
)

// isolateSettingsDir points the per-user config directory at a temp dir so a
// test can never touch the real settings file, and returns where it would go.
func isolateSettingsDir(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	t.Setenv("APPDATA", dir)
	t.Setenv("XDG_CONFIG_HOME", dir)
	t.Setenv("HOME", dir)
	path, err := app.SettingsPath()
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(path, dir) {
		t.Fatalf("settings path %q is outside the isolated dir %q", path, dir)
	}
	return path
}

func newTestModel(settings app.Settings) *model {
	return &model{settings: settings, status: app.StatusDisconnected, message: "Not connected"}
}

func TestSaveSettingsRejectsInvalidControlURLAndKeepsOtherSettings(t *testing.T) {
	path := isolateSettingsDir(t)
	m := newTestModel(app.Settings{
		Browser:    "chrome",
		ControlURL: "http://headscale.example.com",
		Verbose:    true,
		MemoryMode: app.MemoryLow,
	})

	m.saveSettings(nil)

	s := m.snapshot()
	if !strings.Contains(s.settingsErr, "must use https://") {
		t.Fatalf("settings error = %q, want the control URL error", s.settingsErr)
	}
	if s.settings.Browser != "chrome" || !s.settings.Verbose || s.settings.MemoryMode != app.MemoryLow {
		t.Fatalf("other settings were lost: %#v", s.settings)
	}
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Fatalf("invalid settings were written to %s (stat error: %v)", filepath.Base(path), err)
	}

	// Fixing the URL clears the error and saves everything.
	m.mu.Lock()
	m.settings.ControlURL = "https://headscale.example.com"
	m.mu.Unlock()
	m.saveSettings(nil)
	if got := m.snapshot().settingsErr; got != "" {
		t.Fatalf("settings error after fix = %q, want none", got)
	}
	saved, err := app.LoadSettingsFrom(path)
	if err != nil {
		t.Fatal(err)
	}
	if saved.ControlURL != "https://headscale.example.com" || saved.Browser != "chrome" || saved.MemoryMode != app.MemoryLow {
		t.Fatalf("saved settings = %#v", saved)
	}
}

func TestSaveSettingsStoresTrimmedControlURLInModelAndFile(t *testing.T) {
	path := isolateSettingsDir(t)
	m := newTestModel(app.Settings{ControlURL: "  https://headscale.example.com\t", MemoryMode: app.MemoryAuto})

	m.saveSettings(nil)

	const want = "https://headscale.example.com"
	if got := m.snapshot().settings.ControlURL; got != want {
		t.Errorf("model control URL after save = %q, want %q", got, want)
	}
	saved, err := app.LoadSettingsFrom(path)
	if err != nil {
		t.Fatal(err)
	}
	if saved.ControlURL != want {
		t.Errorf("saved control URL = %q, want %q", saved.ControlURL, want)
	}
}

func TestConnectOptionsUseTrimmedControlURL(t *testing.T) {
	opts, err := connectOptions(app.Settings{ControlURL: " https://headscale.example.com ", Browser: "chrome", Persist: true})
	if err != nil {
		t.Fatal(err)
	}
	if want := "https://headscale.example.com"; opts.ControlURL != want {
		t.Fatalf("connect options control URL = %q, want %q", opts.ControlURL, want)
	}
	if opts.Browser != "chrome" || !opts.Persist || !opts.AppMode {
		t.Fatalf("other options were dropped: %#v", opts)
	}
	if _, err := connectOptions(app.Settings{ControlURL: "http://headscale.example.com"}); err == nil {
		t.Fatal("connectOptions accepted an http control URL")
	}
}

func TestConnectRejectsInvalidControlURLBeforeStartingSession(t *testing.T) {
	isolateSettingsDir(t)
	m := newTestModel(app.Settings{
		Browser:    "taillaunch-test-no-such-browser",
		ControlURL: "https://user:pass@headscale.example.com",
		MemoryMode: app.MemoryAuto,
	})

	m.connect(nil)

	s := m.snapshot()
	if s.busy || s.connected {
		t.Fatalf("connect started despite an invalid control URL: busy=%t connected=%t", s.busy, s.connected)
	}
	if s.status != app.StatusFailed || !strings.Contains(s.message, "control URL") {
		t.Fatalf("status = %q, message = %q, want a failed status naming the control URL", s.status, s.message)
	}
	if got := connectionStatusText(s); !strings.Contains(got, "control URL") {
		t.Fatalf("connect screen text = %q, want it to show the error", got)
	}
}

func TestConnectionStatusText(t *testing.T) {
	tests := []struct {
		name string
		snap snapshot
		want string
	}{
		{name: "disconnected", snap: snapshot{status: app.StatusDisconnected}, want: "○ Not connected"},
		{name: "authenticating", snap: snapshot{status: app.StatusAuthenticating}, want: "○ Finish signing in in your browser…"},
		{name: "connecting", snap: snapshot{status: app.StatusStarting}, want: "○ Connecting…"},
		{name: "connected", snap: snapshot{connected: true, status: app.StatusConnected}, want: "● Connected"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := connectionStatusText(tt.snap); got != tt.want {
				t.Fatalf("connectionStatusText() = %q, want %q", got, tt.want)
			}
		})
	}
}
