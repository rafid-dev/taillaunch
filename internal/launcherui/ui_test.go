package launcherui

import (
	"testing"

	"github.com/rafid-dev/taillaunch/internal/app"
)

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
