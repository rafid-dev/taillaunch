package app

import (
	"bufio"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

type testLog struct{ lines []string }

func (l *testLog) logf(format string, args ...any) {
	l.lines = append(l.lines, fmt.Sprintf(format, args...))
}

func newTestSweeper(root string) (*sweeper, *testLog) {
	l := &testLog{}
	return newSweeper(root, l.logf), l
}

func mkdirWithState(t *testing.T, root, name string) string {
	t.Helper()
	dir := filepath.Join(root, name)
	if err := os.MkdirAll(filepath.Join(dir, "Default", "Cache"), 0o700); err != nil {
		t.Fatal(err)
	}
	for _, rel := range []string{"tailscaled.state", filepath.Join("Default", "Cache", "data_0")} {
		if err := os.WriteFile(filepath.Join(dir, rel), []byte("secret"), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	return dir
}

// makeStale creates a folder exactly as a session does and then drops the
// lock, which is what the OS does for a crashed process.
func makeStale(t *testing.T, root, name string) string {
	t.Helper()
	dir := mkdirWithState(t, root, name)
	lock, err := lockSessionDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	if err := lock.Release(); err != nil {
		t.Fatal(err)
	}
	return dir
}

func ageDir(t *testing.T, dir string, age time.Duration) {
	t.Helper()
	old := time.Now().Add(-age)
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	for _, e := range entries {
		if err := os.Chtimes(filepath.Join(dir, e.Name()), old, old); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.Chtimes(dir, old, old); err != nil {
		t.Fatal(err)
	}
}

func mustExist(t *testing.T, path string) {
	t.Helper()
	if _, err := os.Lstat(path); err != nil {
		t.Errorf("%s should still exist: %v", path, err)
	}
}

func mustNotExist(t *testing.T, path string) {
	t.Helper()
	if _, err := os.Lstat(path); !os.IsNotExist(err) {
		t.Errorf("%s should have been removed (lstat err = %v)", path, err)
	}
}

func TestSweepRemovesStaleMarkedDirs(t *testing.T) {
	root := t.TempDir()
	state := makeStale(t, root, "taillaunch-state-1234567")
	browser := makeStale(t, root, "taillaunch-browser-7654321")

	s, _ := newTestSweeper(root)
	if got := s.run(); got != 2 {
		t.Errorf("run() = %d, want 2", got)
	}
	mustNotExist(t, state)
	mustNotExist(t, browser)
}

func TestSweepKeepsLockedDirs(t *testing.T) {
	root := t.TempDir()
	dir := mkdirWithState(t, root, "taillaunch-state-42")
	lock, err := lockSessionDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	released := false
	defer func() {
		if !released {
			lock.Release()
		}
	}()

	s, logs := newTestSweeper(root)
	if got := s.run(); got != 0 {
		t.Errorf("run() = %d while the session is live, want 0", got)
	}
	mustExist(t, filepath.Join(dir, "tailscaled.state"))
	if len(logs.lines) != 0 {
		t.Errorf("a live session is normal and should not be logged: %q", logs.lines)
	}

	// Once the holder goes away the same folder is stale and goes.
	if err := lock.Release(); err != nil {
		t.Fatal(err)
	}
	released = true
	if got := s.run(); got != 1 {
		t.Errorf("run() after release = %d, want 1", got)
	}
	mustNotExist(t, dir)
}

func TestSweepKeepsUnrelatedAndNearMissNames(t *testing.T) {
	root := t.TempDir()
	names := []string{
		"unrelated",
		"taillaunch",
		"taillaunch-state",
		"taillaunch-state-",
		"taillaunch-state-abc",
		"taillaunch-state-123.bak",
		"taillaunch-state-12 3",
		"taillaunch-states-123",
		"taillaunch-state_123",
		"xtaillaunch-state-123",
		"TAILLAUNCH-STATE-123",
		"taillaunch-browser",
		"taillaunch-browser-12a",
		"taillaunch-cache-123",
	}
	var dirs []string
	for _, name := range names {
		dir := makeStale(t, root, name)
		ageDir(t, dir, 72*time.Hour)
		dirs = append(dirs, dir)
	}
	// A plain file with a matching name is not a folder.
	file := filepath.Join(root, "taillaunch-state-999")
	if err := os.WriteFile(file, []byte("x"), 0o600); err != nil {
		t.Fatal(err)
	}

	s, _ := newTestSweeper(root)
	if got := s.run(); got != 0 {
		t.Errorf("run() = %d, want 0", got)
	}
	for _, dir := range append(dirs, file) {
		mustExist(t, dir)
	}
}

func TestSweepKeepsSymlinks(t *testing.T) {
	root := t.TempDir()
	outside := t.TempDir()
	target := makeStale(t, outside, "target")
	link := filepath.Join(root, "taillaunch-state-555")
	if err := os.Symlink(target, link); err != nil {
		t.Skipf("cannot create symlinks here: %v", err)
	}

	s, _ := newTestSweeper(root)
	if got := s.run(); got != 0 {
		t.Errorf("run() = %d, want 0", got)
	}
	mustExist(t, link)
	mustExist(t, filepath.Join(target, "tailscaled.state"))
}

func TestSweepNeverTouchesUnmarkedDirs(t *testing.T) {
	root := t.TempDir()
	var dirs []string
	for i, age := range []time.Duration{0, time.Hour, 25 * time.Hour, 30 * 24 * time.Hour, 5 * 365 * 24 * time.Hour} {
		dir := mkdirWithState(t, root, fmt.Sprintf("taillaunch-state-%d", i))
		ageDir(t, dir, age)
		dirs = append(dirs, dir)
		dir = mkdirWithState(t, root, fmt.Sprintf("taillaunch-browser-%d", i))
		ageDir(t, dir, age)
		dirs = append(dirs, dir)
	}
	// A lock file alone, or a marker with the wrong content, is not a marker.
	lockOnly := mkdirWithState(t, root, "taillaunch-state-100")
	if err := os.WriteFile(filepath.Join(lockOnly, lockFileName), nil, 0o600); err != nil {
		t.Fatal(err)
	}
	bogus := mkdirWithState(t, root, "taillaunch-state-101")
	if err := os.WriteFile(filepath.Join(bogus, markerFileName), []byte("not ours"), 0o600); err != nil {
		t.Fatal(err)
	}
	for _, dir := range []string{lockOnly, bogus} {
		ageDir(t, dir, 365*24*time.Hour)
		dirs = append(dirs, dir)
	}

	s, logs := newTestSweeper(root)
	if got := s.run(); got != 0 {
		t.Errorf("run() = %d, want 0", got)
	}
	for _, dir := range dirs {
		mustExist(t, filepath.Join(dir, "tailscaled.state"))
		mustExist(t, filepath.Join(dir, "Default", "Cache", "data_0"))
	}
	if len(logs.lines) != 0 {
		t.Errorf("unmarked folders should be skipped silently: %q", logs.lines)
	}
}

func TestSweepDeletionErrorsDoNotAbort(t *testing.T) {
	root := t.TempDir()
	stuck := makeStale(t, root, "taillaunch-browser-100")
	stuckToo := makeStale(t, root, "taillaunch-state-101")
	later := makeStale(t, root, "taillaunch-state-102")

	s, logs := newTestSweeper(root)
	real := s.removeAll
	s.removeAll = func(path string) error {
		for _, bad := range []string{stuck, stuckToo} {
			if path == bad || strings.HasPrefix(path, bad+string(filepath.Separator)) {
				return fmt.Errorf("simulated: file in use")
			}
		}
		return real(path)
	}

	if got := s.run(); got != 1 {
		t.Errorf("run() = %d, want 1", got)
	}
	mustExist(t, stuck)
	mustExist(t, stuckToo)
	mustNotExist(t, later)

	joined := strings.Join(logs.lines, "\n")
	for _, bad := range []string{stuck, stuckToo} {
		if !strings.Contains(joined, fmt.Sprintf("%q", bad)) {
			t.Errorf("failure for %s was not logged; log:\n%s", bad, joined)
		}
	}

	// The failed folders are retried on the next startup.
	s.removeAll = real
	if got := s.run(); got != 2 {
		t.Errorf("retry run() = %d, want 2", got)
	}
	mustNotExist(t, stuck)
	mustNotExist(t, stuckToo)
}

func TestSweepMissingRootIsHarmless(t *testing.T) {
	s, _ := newTestSweeper(filepath.Join(t.TempDir(), "nope"))
	if got := s.run(); got != 0 {
		t.Errorf("run() = %d, want 0", got)
	}
}

func TestPrepareDirsHoldsLockUntilCleanup(t *testing.T) {
	root := t.TempDir()
	for _, key := range []string{"TMPDIR", "TMP", "TEMP"} {
		t.Setenv(key, root)
	}

	stateDir, profileDir, cleanup, err := PrepareDirs(Options{})
	if err != nil {
		t.Fatal(err)
	}
	if filepath.Dir(stateDir) != filepath.Clean(root) {
		t.Fatalf("state dir %q is not directly under %q", stateDir, root)
	}

	s, _ := newTestSweeper(root)
	if got := s.run(); got != 0 {
		t.Errorf("sweep removed %d folders of a live session", got)
	}
	mustExist(t, stateDir)
	mustExist(t, profileDir)

	if err := cleanup(); err != nil {
		t.Fatal(err)
	}
	mustNotExist(t, stateDir)
	mustNotExist(t, profileDir)
	if err := cleanup(); err != nil {
		t.Errorf("second cleanup() = %v, want nil", err)
	}
}

// TestHelperHoldSessionLock is not a test: it is the body of the child process
// started by TestSweepFollowsLockOfAnotherProcess.
func TestHelperHoldSessionLock(t *testing.T) {
	dir := os.Getenv("TAILLAUNCH_TEST_HOLD_DIR")
	if dir == "" {
		t.Skip("helper process only")
	}
	if _, err := lockSessionDir(dir); err != nil {
		fmt.Println("error:", err)
		os.Exit(1)
	}
	fmt.Println("locked")
	time.Sleep(time.Hour) // hold the lock until killed
}

func TestSweepFollowsLockOfAnotherProcess(t *testing.T) {
	root := t.TempDir()
	dir := mkdirWithState(t, root, "taillaunch-state-31337")

	cmd := exec.Command(os.Args[0], "-test.run=^TestHelperHoldSessionLock$")
	cmd.Env = append(os.Environ(), "TAILLAUNCH_TEST_HOLD_DIR="+dir)
	out, err := cmd.StdoutPipe()
	if err != nil {
		t.Fatal(err)
	}
	if err := cmd.Start(); err != nil {
		t.Fatal(err)
	}
	killed := false
	defer func() {
		if !killed {
			cmd.Process.Kill()
			cmd.Wait()
		}
	}()
	sc := bufio.NewScanner(out)
	for sc.Scan() {
		if sc.Text() == "locked" {
			break
		}
		if len(sc.Text()) > 6 && sc.Text()[:6] == "error:" {
			t.Fatalf("helper: %s", sc.Text())
		}
	}
	if sc.Text() != "locked" {
		t.Fatal("helper process never reported holding the lock")
	}

	s, _ := newTestSweeper(root)
	if got := s.run(); got != 0 {
		t.Errorf("run() = %d while another process holds the lock, want 0", got)
	}
	mustExist(t, filepath.Join(dir, "tailscaled.state"))

	// A killed process cannot clean up after itself; the OS drops its lock.
	cmd.Process.Kill()
	cmd.Wait()
	killed = true
	if got := s.run(); got != 1 {
		t.Errorf("run() after the holder was killed = %d, want 1", got)
	}
	mustNotExist(t, dir)
}
