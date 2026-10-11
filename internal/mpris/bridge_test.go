package mpris

import (
	"fmt"
	"testing"
)

func TestBridgeMapsPlaybackActionsToExplicitMessages(t *testing.T) {
	bridge := NewBridge(nil, "spruce")
	bridge.Bind(accessorFn(&mockAccessor{}), 0)

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

func TestBridgeSkipsTrackOnNextAndPreviousByDefault(t *testing.T) {
	bridge := NewBridge(nil, "spruce")
	bridge.Bind(accessorFn(&mockAccessor{}), 0)

	if err := bridge.player.Next(); err != nil {
		t.Fatalf("Next() error: %v", err)
	}
	if _, ok := (<-bridge.msgCh).(NextMsg); !ok {
		t.Fatal("Next() should send NextMsg")
	}
	if err := bridge.player.Previous(); err != nil {
		t.Fatalf("Previous() error: %v", err)
	}
	if _, ok := (<-bridge.msgCh).(PreviousMsg); !ok {
		t.Fatal("Previous() should send PreviousMsg")
	}
}

func TestBridgeSeeksOnNextAndPreviousWithSkipSeekSeconds(t *testing.T) {
	bridge := NewBridge(nil, "pine")
	bridge.Bind(accessorFn(&mockAccessor{}), 10)

	// The error return stops the desktop wait for a track change.
	if err := bridge.player.Next(); err == nil {
		t.Fatal("Next() should return an error in seek mode")
	}
	if got := <-bridge.msgCh; got != (SeekRelativeMsg{OffsetSeconds: 10}) {
		t.Fatalf("Next() sent %#v; want SeekRelativeMsg{OffsetSeconds: 10}", got)
	}
	if err := bridge.player.Previous(); err == nil {
		t.Fatal("Previous() should return an error in seek mode")
	}
	if got := <-bridge.msgCh; got != (SeekRelativeMsg{OffsetSeconds: -10}) {
		t.Fatalf("Previous() sent %#v; want SeekRelativeMsg{OffsetSeconds: -10}", got)
	}
}
