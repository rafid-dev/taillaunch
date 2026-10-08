//go:build windows

package browser

import "testing"

func TestManagedSessionClosesAfterProcessExit(t *testing.T) {
	session, err := launch("cmd.exe", []string{"/C", "exit", "0"})
	if err != nil {
		t.Fatal(err)
	}
	if err := session.Wait(); err != nil {
		t.Fatalf("wait for child process: %v", err)
	}
	if err := session.Close(); err != nil {
		t.Fatalf("close managed session: %v", err)
	}
}
