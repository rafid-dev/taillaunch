//go:build !stub

package tailnet

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestRemoveTsnetLogFilesRemovesOnlyLogFiles(t *testing.T) {
	dir := t.TempDir()
	logFiles := []string{"tailscaled.log.conf", "tailscaled.log1.txt", "tailscaled.log2.txt"}
	keep := []string{"tailscaled.state", "tailscaled.log.conf.bak", "tailscaled.log3.txt", "tailscaled.log1.txt.old", "notes.txt"}
	for _, name := range append(append([]string{}, logFiles...), keep...) {
		if err := os.WriteFile(filepath.Join(dir, name), []byte("data"), 0o600); err != nil {
			t.Fatal(err)
		}
	}

	if err := removeTsnetLogFiles(dir); err != nil {
		t.Fatalf("removeTsnetLogFiles: %v", err)
	}

	for _, name := range logFiles {
		if _, err := os.Stat(filepath.Join(dir, name)); !os.IsNotExist(err) {
			t.Errorf("%s should have been removed (stat err = %v)", name, err)
		}
	}
	for _, name := range keep {
		if _, err := os.Stat(filepath.Join(dir, name)); err != nil {
			t.Errorf("%s should have been kept: %v", name, err)
		}
	}
}

func TestRemoveTsnetLogFilesLeavesDirectories(t *testing.T) {
	dir := t.TempDir()
	sub := filepath.Join(dir, "tailscaled.log1.txt")
	if err := os.Mkdir(sub, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(sub, "inner"), []byte("data"), 0o600); err != nil {
		t.Fatal(err)
	}
	_ = removeTsnetLogFiles(dir)
	if _, err := os.Stat(filepath.Join(sub, "inner")); err != nil {
		t.Fatalf("directory contents must be untouched: %v", err)
	}
}

func TestRemoveTsnetLogFilesMissingFilesAndEmptyDir(t *testing.T) {
	if err := removeTsnetLogFiles(t.TempDir()); err != nil {
		t.Fatalf("missing files should not be an error: %v", err)
	}
	if err := removeTsnetLogFiles(""); err != nil {
		t.Fatalf("empty dir should be a no-op: %v", err)
	}
}

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
