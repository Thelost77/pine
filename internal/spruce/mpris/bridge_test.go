package mpris

import (
	"fmt"
	"testing"
)

func TestBridgeMapsPlaybackActionsToExplicitMessages(t *testing.T) {
	bridge := NewBridge(nil)
	bridge.Bind(accessorFn(&mockAccessor{}))

	for _, tt := range []struct {
		name   string
		action func() error
		want   string
	}{
		{name: "pause", action: bridge.player.Pause, want: "mpris.PauseMsg"},
		{name: "play", action: bridge.player.Play, want: "mpris.PlayMsg"},
		{name: "stop", action: bridge.player.Stop, want: "mpris.PauseMsg"},
		{name: "play pause", action: bridge.player.PlayPause, want: "mpris.PlayPauseMsg"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			if err := tt.action(); err != nil {
				t.Fatalf("action() error: %v", err)
			}
			if got := fmt.Sprintf("%T", <-bridge.msgCh); got != tt.want {
				t.Fatalf("message = %s; want %s", got, tt.want)
			}
		})
	}
}
