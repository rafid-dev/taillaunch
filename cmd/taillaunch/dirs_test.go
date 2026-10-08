package main

import (
	"os"
	"path/filepath"
	"testing"
)

func TestPrepareDirsDefaultsToDisposablePrivateDirectories(t *testing.T) {
	stateDir, profileDir, cleanup, err := prepareDirs(config{})
	if err != nil {
		t.Fatal(err)
	}
	if stateDir == profileDir || stateDir == "" || profileDir == "" {
		t.Fatalf("temporary directories = %q, %q", stateDir, profileDir)
	}
	if _, err := os.Stat(stateDir); err != nil {
		t.Fatalf("state directory was not created: %v", err)
	}
	if _, err := os.Stat(profileDir); err != nil {
		t.Fatalf("profile directory was not created: %v", err)
	}

	if err := cleanup(); err != nil {
		t.Fatal(err)
	}
	for _, dir := range []string{stateDir, profileDir} {
		if _, err := os.Stat(dir); !os.IsNotExist(err) {
			t.Fatalf("temporary directory %q still exists; stat error = %v", dir, err)
		}
	}
}

func TestPrepareDirsRequiresPersistForExplicitDirectories(t *testing.T) {
	root := t.TempDir()
	_, _, _, err := prepareDirs(config{
		stateDir:   filepath.Join(root, "state"),
		profileDir: filepath.Join(root, "profile"),
	})
	if err == nil {
		t.Fatal("explicit directories were accepted without --persist")
	}
}

func TestPrepareDirsPersistKeepsExplicitDirectories(t *testing.T) {
	root := t.TempDir()
	stateDir := filepath.Join(root, "state")
	profileDir := filepath.Join(root, "profile")
	gotState, gotProfile, cleanup, err := prepareDirs(config{
		persist:    true,
		stateDir:   stateDir,
		profileDir: profileDir,
	})
	if err != nil {
		t.Fatal(err)
	}
	if gotState != stateDir || gotProfile != profileDir {
		t.Fatalf("persistent directories = %q, %q; want %q, %q", gotState, gotProfile, stateDir, profileDir)
	}
	if err := cleanup(); err != nil {
		t.Fatal(err)
	}
	for _, dir := range []string{stateDir, profileDir} {
		if _, err := os.Stat(dir); err != nil {
			t.Fatalf("persistent directory %q was removed: %v", dir, err)
		}
	}
}
