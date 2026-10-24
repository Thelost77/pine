package mpris

import (
	"reflect"
	"testing"

	"github.com/godbus/dbus/v5"
)

type mockAccessor struct {
	playing  bool
	paused   bool
	hasItem  bool
	title    string
	authors  []string
	itemID   string
	position float64
	duration float64
	noSeek   bool
	volume   int
	speed    float64
	queueLen int
}

func (m *mockAccessor) IsPlaying() bool            { return m.playing }
func (m *mockAccessor) IsPaused() bool             { return m.paused }
func (m *mockAccessor) HasActiveItem() bool        { return m.hasItem }
func (m *mockAccessor) CurrentTitle() string       { return m.title }
func (m *mockAccessor) CurrentAuthors() []string   { return m.authors }
func (m *mockAccessor) CurrentItemID() string      { return m.itemID }
func (m *mockAccessor) PlayerPosition() float64    { return m.position }
func (m *mockAccessor) PlayerDuration() float64    { return m.duration }
func (m *mockAccessor) PlayerVolume() int          { return m.volume }
func (m *mockAccessor) PlayerSpeed() float64       { return m.speed }
func (m *mockAccessor) PlayerMinimumRate() float64 { return 0.5 }
func (m *mockAccessor) PlayerMaximumRate() float64 { return 4.0 }
func (m *mockAccessor) CanSeek() bool              { return !m.noSeek }
func (m *mockAccessor) CanGoNext() bool            { return m.queueLen > 0 }
func (m *mockAccessor) CanGoPrevious() bool        { return true }

func accessorFn(a *mockAccessor) func() ModelAccessor {
	return func() ModelAccessor { return a }
}

// playerProperty returns the value of a player property of a server that
// reads the app state from a.
func playerProperty(t *testing.T, a *mockAccessor, name string) any {
	t.Helper()
	s := NewServer("spruce", RootAdapter{Name: "spruce"}, accessorFn(a), PlayerActions{})
	value, err := s.get(playerInterface, name)
	if err != nil {
		t.Fatalf("Get(%s) error: %v", name, err)
	}
	return value.Value()
}

func TestRootAdapterIdentity(t *testing.T) {
	r := RootAdapter{Name: "pine"}

	if v, err := r.Identity(); err != nil || v != "pine" {
		t.Errorf("Identity() = %q, %v; want %q, nil", v, err, "pine")
	}
	if v, err := r.DesktopEntry(); err != nil || v != "pine" {
		t.Errorf("DesktopEntry() = %q, %v; want %q, nil", v, err, "pine")
	}
	// Quit does nothing, so MPRIS requires CanQuit to be false.
	if v, err := r.CanQuit(); err != nil || v {
		t.Errorf("CanQuit() = %v, %v; want false, nil", v, err)
	}
	if v, err := r.CanRaise(); err != nil || v {
		t.Errorf("CanRaise() = %v, %v; want false, nil", v, err)
	}
	if v, err := r.HasTrackList(); err != nil || v {
		t.Errorf("HasTrackList() = %v, %v; want false, nil", v, err)
	}
	if err := r.Quit(); err != nil {
		t.Errorf("Quit() returned error: %v", err)
	}
	if err := r.Raise(); err != nil {
		t.Errorf("Raise() returned error: %v", err)
	}
	schemes, err := r.SupportedUriSchemes()
	if err != nil || schemes != nil {
		t.Errorf("SupportedUriSchemes() = %v, %v; want nil, nil", schemes, err)
	}
	mimes, err := r.SupportedMimeTypes()
	if err != nil || mimes != nil {
		t.Errorf("SupportedMimeTypes() = %v, %v; want nil, nil", mimes, err)
	}
}

func TestServerPlaybackStatus(t *testing.T) {
	tests := []struct {
		name     string
		accessor *mockAccessor
		want     PlaybackStatus
	}{
		{"playing", &mockAccessor{playing: true}, StatusPlaying},
		{"paused", &mockAccessor{paused: true}, StatusPaused},
		{"stopped", &mockAccessor{}, StatusStopped},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := playerProperty(t, tt.accessor, "PlaybackStatus"); got != string(tt.want) {
				t.Errorf("PlaybackStatus = %q; want %q", got, tt.want)
			}
		})
	}
}

func TestServerVolumeConversion(t *testing.T) {
	tests := []struct {
		name      string
		appVol    int
		wantMPRIS float64
	}{
		{"zero", 0, 0.0},
		{"half", 75, 0.75},
		{"full", 100, 1.0},
		{"boost", 150, 1.5},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			a := &mockAccessor{volume: tt.appVol}
			if got := playerProperty(t, a, "Volume"); got != tt.wantMPRIS {
				t.Errorf("Volume = %v; want %v", got, tt.wantMPRIS)
			}
		})
	}
}

func TestServerMetadata(t *testing.T) {
	a := &mockAccessor{
		hasItem:  true,
		title:    "The Hobbit",
		authors:  []string{"J.R.R. Tolkien"},
		itemID:   "item-123",
		duration: 3600,
	}

	metadata, ok := playerProperty(t, a, "Metadata").(map[string]dbus.Variant)
	if !ok {
		t.Fatalf("Metadata is not a{sv}")
	}
	want := map[string]any{
		"mpris:trackid": dbus.ObjectPath("/org/spruce/track/item_123"),
		"mpris:length":  int64(3_600_000_000),
		"xesam:title":   "The Hobbit",
		"xesam:artist":  []string{"J.R.R. Tolkien"},
	}
	if len(metadata) != len(want) {
		t.Errorf("Metadata has %d entries; want %d", len(metadata), len(want))
	}
	for key, wantValue := range want {
		if got := metadata[key].Value(); !reflect.DeepEqual(got, wantValue) {
			t.Errorf("Metadata[%s] = %#v; want %#v", key, got, wantValue)
		}
	}
}

func TestServerMetadataNoItem(t *testing.T) {
	metadata, ok := playerProperty(t, &mockAccessor{}, "Metadata").(map[string]dbus.Variant)
	if !ok {
		t.Fatalf("Metadata is not a{sv}")
	}
	if _, ok := metadata["xesam:title"]; ok || len(metadata) != 1 {
		t.Errorf("Metadata = %v; want only mpris:trackid", metadata)
	}
	// MPRIS reserves this path for no track.
	if got, want := metadata["mpris:trackid"].Value(), dbus.ObjectPath("/org/mpris/MediaPlayer2/TrackList/NoTrack"); got != want {
		t.Errorf("Metadata[mpris:trackid] = %#v; want %#v", got, want)
	}
}

// TestServerCanPlayAndPauseNeedAnItem verifies that Play and Pause are
// possible only with a current item. Without one, Play does nothing.
func TestServerCanPlayAndPauseNeedAnItem(t *testing.T) {
	for _, hasItem := range []bool{false, true} {
		a := &mockAccessor{hasItem: hasItem}
		for _, property := range []string{"CanPlay", "CanPause"} {
			if got := playerProperty(t, a, property); got != hasItem {
				t.Errorf("%s with an item %v = %v; want %v", property, hasItem, got, hasItem)
			}
		}
	}
}

func TestServerPosition(t *testing.T) {
	if got := playerProperty(t, &mockAccessor{position: 42.5}, "Position"); got != int64(42_500_000) {
		t.Errorf("Position = %#v; want int64(42500000)", got)
	}
}

func TestServerCanGoNext(t *testing.T) {
	a := &mockAccessor{queueLen: 2}
	if got := playerProperty(t, a, "CanGoNext"); got != true {
		t.Error("CanGoNext = false; want true (queue has items)")
	}

	a.queueLen = 0
	if got := playerProperty(t, a, "CanGoNext"); got != false {
		t.Error("CanGoNext = true; want false (empty queue)")
	}
}

func TestServerCanControl(t *testing.T) {
	if got := playerProperty(t, &mockAccessor{}, "CanControl"); got != true {
		t.Error("CanControl = false; want true")
	}
}

func TestServerRates(t *testing.T) {
	a := &mockAccessor{speed: 1.5}
	for name, want := range map[string]float64{"Rate": 1.5, "MinimumRate": 0.5, "MaximumRate": 4.0} {
		if got := playerProperty(t, a, name); got != want {
			t.Errorf("%s = %v; want %v", name, got, want)
		}
	}
}
