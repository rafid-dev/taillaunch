package app

import (
	"os"
	"path/filepath"
	"testing"
)

func TestNormalizeTargetAcceptsHostnameAndPreservesURL(t *testing.T) {
	if got, want := mustNormalizeTarget(t, "nas"), "https://nas"; got != want {
		t.Fatalf("hostname target = %q, want %q", got, want)
	}
	if got, want := mustNormalizeTarget(t, "http://nas.local/app"), "http://nas.local/app"; got != want {
		t.Fatalf("URL target = %q, want %q", got, want)
	}
}

func TestNormalizeTargetRejectsUnsafeTarget(t *testing.T) {
	for _, raw := range []string{"", "file:///tmp/app"} {
		if _, err := NormalizeTarget(raw); err == nil {
			t.Errorf("NormalizeTarget(%q) accepted an unsafe target", raw)
		}
	}
}

func TestMemoryModeExplicitOverridesAuto(t *testing.T) {
	if !LowMemoryFor(MemoryLow, 64<<30) {
		t.Fatal("low-memory override should win on a high-RAM system")
	}
	if LowMemoryFor(MemoryNormal, 2<<30) {
		t.Fatal("normal-memory override should win on a low-RAM system")
	}
	if !LowMemoryFor(MemoryAuto, 4<<30) {
		t.Fatal("Auto should select low-memory behavior at the threshold")
	}
}

func TestSettingsRoundTripAndDefaultMemoryMode(t *testing.T) {
	path := filepath.Join(t.TempDir(), "settings.json")
	want := Settings{
		Persist:    true,
		Browser:    "  chrome  ",
		ControlURL: " https://headscale.example.com ",
		Verbose:    true,
		MemoryMode: MemoryLow,
	}
	if err := SaveSettingsTo(path, want); err != nil {
		t.Fatal(err)
	}
	got, err := LoadSettingsFrom(path)
	if err != nil {
		t.Fatal(err)
	}
	if got.Browser != "chrome" || got.ControlURL != "https://headscale.example.com" || got.MemoryMode != MemoryLow || !got.Persist || !got.Verbose {
		t.Fatalf("settings = %#v, want normalized round trip", got)
	}

	defaults, err := LoadSettingsFrom(filepath.Join(t.TempDir(), "missing.json"))
	if err != nil {
		t.Fatal(err)
	}
	if defaults.MemoryMode != MemoryAuto {
		t.Fatalf("default memory mode = %q, want %q", defaults.MemoryMode, MemoryAuto)
	}
}

func TestSettingsRejectsMalformedJSON(t *testing.T) {
	path := filepath.Join(t.TempDir(), "settings.json")
	if err := os.WriteFile(path, []byte("{not-json"), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := LoadSettingsFrom(path); err == nil {
		t.Fatal("malformed settings were accepted")
	}
}

func mustNormalizeTarget(t *testing.T, raw string) string {
	t.Helper()
	got, err := NormalizeTarget(raw)
	if err != nil {
		t.Fatal(err)
	}
	return got
}
