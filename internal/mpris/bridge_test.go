package mpris

import (
	"fmt"
	"math"
	"testing"
	"time"

	"github.com/godbus/dbus/v5"
	"github.com/godbus/dbus/v5/prop"
)

func TestBridgeMapsPlaybackActionsToExplicitMessages(t *testing.T) {
	bridge := NewBridge(nil, "spruce")
	bridge.Bind(accessorFn(&mockAccessor{}), 0)

	for _, tt := range []struct {
		name   string
		action func() error
		want   string
	}{
		{name: "pause", action: bridge.server.actions.Pause, want: "mpris.PauseMsg"},
		{name: "play", action: bridge.server.actions.Play, want: "mpris.PlayMsg"},
		{name: "stop", action: bridge.server.actions.Stop, want: "mpris.PauseMsg"},
		{name: "play pause", action: bridge.server.actions.PlayPause, want: "mpris.PlayPauseMsg"},
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

	if err := bridge.server.actions.Next(); err != nil {
		t.Fatalf("Next() error: %v", err)
	}
	if _, ok := (<-bridge.msgCh).(NextMsg); !ok {
		t.Fatal("Next() should send NextMsg")
	}
	if err := bridge.server.actions.Previous(); err != nil {
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
	if err := bridge.server.actions.Next(); err == nil {
		t.Fatal("Next() should return an error in seek mode")
	}
	if got := <-bridge.msgCh; got != (SeekRelativeMsg{OffsetSeconds: 10}) {
		t.Fatalf("Next() sent %#v; want SeekRelativeMsg{OffsetSeconds: 10}", got)
	}
	if err := bridge.server.actions.Previous(); err == nil {
		t.Fatal("Previous() should return an error in seek mode")
	}
	if got := <-bridge.msgCh; got != (SeekRelativeMsg{OffsetSeconds: -10}) {
		t.Fatalf("Previous() sent %#v; want SeekRelativeMsg{OffsetSeconds: -10}", got)
	}
}

func TestBridgeSetPositionAcceptsThePublishedTrackID(t *testing.T) {
	bridge := NewBridge(nil, "spruce")
	bridge.Bind(accessorFn(&mockAccessor{title: "Song", itemID: "t-1"}), 0)

	metadata, dbusErr := bridge.server.get(playerInterface, "Metadata")
	if dbusErr != nil {
		t.Fatalf("Get(Metadata) error: %v", dbusErr)
	}
	trackID := metadata.Value().(map[string]dbus.Variant)["mpris:trackid"].Value().(dbus.ObjectPath)
	if err := bridge.server.actions.SetPosition(string(trackID), 30*time.Second); err != nil {
		t.Fatalf("SetPosition() error: %v", err)
	}

	select {
	case got := <-bridge.msgCh:
		if got != (SeekAbsoluteMsg{PositionSeconds: 30}) {
			t.Fatalf("message = %#v; want SeekAbsoluteMsg{PositionSeconds: 30}", got)
		}
	default:
		t.Fatal("SetPosition for the current track sent no message")
	}
}

func TestBridgeSetPositionIgnoresStaleAndOutOfRangeRequests(t *testing.T) {
	for _, tt := range []struct {
		name     string
		accessor *mockAccessor
		trackID  string
		// position is 30 seconds if it is 0.
		position time.Duration
	}{
		{name: "other track", accessor: &mockAccessor{title: "Song", itemID: "t-1"}, trackID: "/org/spruce/track/t_2"},
		{name: "raw item id", accessor: &mockAccessor{title: "Song", itemID: "t-1"}, trackID: "t-1"},
		{name: "empty id", accessor: &mockAccessor{title: "Song", itemID: "t-1"}, trackID: ""},
		{name: "no track", accessor: &mockAccessor{}, trackID: "/org/mpris/MediaPlayer2/TrackList/NoTrack"},
		{name: "seek not possible", accessor: &mockAccessor{title: "Mix", itemID: "mix", noSeek: true}, trackID: "/org/spruce/track/mix"},
		{name: "position before the start", accessor: &mockAccessor{title: "Song", itemID: "t-1"}, trackID: "/org/spruce/track/t_1", position: -time.Second},
		{name: "position past the end", accessor: &mockAccessor{title: "Song", itemID: "t-1", duration: 20}, trackID: "/org/spruce/track/t_1"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			bridge := NewBridge(nil, "spruce")
			bridge.Bind(accessorFn(tt.accessor), 0)

			position := 30 * time.Second
			if tt.position != 0 {
				position = tt.position
			}
			if err := bridge.server.actions.SetPosition(tt.trackID, position); err != nil {
				t.Fatalf("SetPosition() error: %v", err)
			}

			select {
			case got := <-bridge.msgCh:
				t.Fatalf("SetPosition sent %#v; want no message", got)
			default:
			}
		})
	}
}

func TestBridgeValidatesRateAndVolume(t *testing.T) {
	bridge := NewBridge(nil, "spruce")
	bridge.Bind(accessorFn(&mockAccessor{}), 0)

	for _, tt := range []struct {
		name     string
		property string
		value    float64
		want     any
	}{
		// MPRIS: a rate of 0 acts as Pause.
		{name: "rate zero", property: "Rate", value: 0, want: PauseMsg{}},
		{name: "rate below the minimum", property: "Rate", value: -1, want: SetRateMsg{Rate: 0.5}},
		{name: "rate above the maximum", property: "Rate", value: 100, want: SetRateMsg{Rate: 4}},
		{name: "rate in range", property: "Rate", value: 1.25, want: SetRateMsg{Rate: 1.25}},
		// MPRIS: a negative volume is 0.
		{name: "negative volume", property: "Volume", value: -0.5, want: SetVolumeMsg{Volume: 0}},
		{name: "volume rounds", property: "Volume", value: 0.58, want: SetVolumeMsg{Volume: 58}},
		{name: "volume above the scale", property: "Volume", value: 2, want: SetVolumeMsg{Volume: 150}},
	} {
		t.Run(tt.name, func(t *testing.T) {
			if err := bridge.server.set(playerInterface, tt.property, dbus.MakeVariant(tt.value)); err != nil {
				t.Fatalf("Set(%s, %v) error: %v", tt.property, tt.value, err)
			}
			select {
			case got := <-bridge.msgCh:
				if got != tt.want {
					t.Errorf("Set(%s, %v) sent %#v; want %#v", tt.property, tt.value, got, tt.want)
				}
			default:
				t.Errorf("Set(%s, %v) sent no message; want %#v", tt.property, tt.value, tt.want)
			}
		})
	}

	for _, property := range []string{"Rate", "Volume"} {
		for _, value := range []float64{math.NaN(), math.Inf(1), math.Inf(-1)} {
			if err := bridge.server.set(playerInterface, property, dbus.MakeVariant(value)); err != prop.ErrInvalidArg {
				t.Errorf("Set(%s, %v) error = %v; want %v", property, value, err, prop.ErrInvalidArg)
			}
			select {
			case got := <-bridge.msgCh:
				t.Errorf("Set(%s, %v) sent %#v; want no message", property, value, got)
			default:
			}
		}
	}
}

// TestBridgeSkipSeekReportsNextAndPreviousAsSeeks verifies that with a skip
// seek, CanGoNext and CanGoPrevious tell whether Next and Previous can seek.
func TestBridgeSkipSeekReportsNextAndPreviousAsSeeks(t *testing.T) {
	for _, tt := range []struct {
		name     string
		accessor *mockAccessor
		want     bool
	}{
		{name: "empty queue", accessor: &mockAccessor{hasItem: true}, want: true},
		{name: "seek not possible", accessor: &mockAccessor{hasItem: true, queueLen: 2, noSeek: true}, want: false},
		{name: "no active item", accessor: &mockAccessor{}, want: false},
	} {
		t.Run(tt.name, func(t *testing.T) {
			bridge := NewBridge(nil, "pine")
			bridge.Bind(accessorFn(tt.accessor), 10)

			for _, property := range []string{"CanGoNext", "CanGoPrevious"} {
				value, err := bridge.server.get(playerInterface, property)
				if err != nil {
					t.Fatalf("Get(%s) error: %v", property, err)
				}
				if value.Value() != tt.want {
					t.Errorf("%s = %v; want %v", property, value.Value(), tt.want)
				}
			}
		})
	}
}
