package mpris

import (
	"testing"

	"github.com/godbus/dbus/v5"
	"github.com/quarckster/go-mpris-server/pkg/types"
)

type mockAccessor struct {
	playing    bool
	paused     bool
	hasItem    bool
	title      string
	authors    []string
	itemID     string
	position   float64
	duration   float64
	volume     int
	speed      float64
	queueLen   int
}

func (m *mockAccessor) IsPlaying() bool        { return m.playing }
func (m *mockAccessor) IsPaused() bool         { return m.paused }
func (m *mockAccessor) HasActiveItem() bool    { return m.hasItem }
func (m *mockAccessor) CurrentTitle() string   { return m.title }
func (m *mockAccessor) CurrentAuthors() []string { return m.authors }
func (m *mockAccessor) CurrentItemID() string  { return m.itemID }
func (m *mockAccessor) PlayerPosition() float64 { return m.position }
func (m *mockAccessor) PlayerDuration() float64 { return m.duration }
func (m *mockAccessor) PlayerVolume() int       { return m.volume }
func (m *mockAccessor) PlayerSpeed() float64    { return m.speed }
func (m *mockAccessor) QueueLength() int        { return m.queueLen }

func TestRootAdapterIdentity(t *testing.T) {
	r := RootAdapter{}

	if v, err := r.Identity(); err != nil || v != "pine" {
		t.Errorf("Identity() = %q, %v; want %q, nil", v, err, "pine")
	}
	if v, err := r.CanQuit(); err != nil || !v {
		t.Errorf("CanQuit() = %v, %v; want true, nil", v, err)
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

func TestPlayerAdapterPlaybackStatus(t *testing.T) {
	tests := []struct {
		name   string
		accessor *mockAccessor
		want   types.PlaybackStatus
	}{
		{"playing", &mockAccessor{playing: true}, types.PlaybackStatusPlaying},
		{"paused", &mockAccessor{paused: true}, types.PlaybackStatusPaused},
		{"stopped", &mockAccessor{}, types.PlaybackStatusStopped},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			p := NewPlayerAdapter(tt.accessor, PlayerActions{})
			got, err := p.PlaybackStatus()
			if err != nil {
				t.Fatalf("PlaybackStatus() error: %v", err)
			}
			if got != tt.want {
				t.Errorf("PlaybackStatus() = %q; want %q", got, tt.want)
			}
		})
	}
}

func TestPlayerAdapterVolumeConversion(t *testing.T) {
	tests := []struct {
		name      string
		pineVol   int
		wantMPRIS float64
	}{
		{"zero", 0, 0.0},
		{"half", 75, 0.75},
		{"full", 100, 1.0},
		{"boost", 150, 1.5},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			a := &mockAccessor{volume: tt.pineVol}
			p := NewPlayerAdapter(a, PlayerActions{})
			got, err := p.Volume()
			if err != nil {
				t.Fatalf("Volume() error: %v", err)
			}
			if got != tt.wantMPRIS {
				t.Errorf("Volume() = %v; want %v", got, tt.wantMPRIS)
			}
		})
	}
}

func TestPlayerAdapterSetVolumeConversion(t *testing.T) {
	var capturedVol int
	actions := PlayerActions{
		SetVolume: func(vol int) error {
			capturedVol = vol
			return nil
		},
	}
	a := &mockAccessor{}
	p := NewPlayerAdapter(a, actions)

	if err := p.SetVolume(0.75); err != nil {
		t.Fatalf("SetVolume() error: %v", err)
	}
	if capturedVol != 75 {
		t.Errorf("SetVolume(0.75) set pine volume = %d; want 75", capturedVol)
	}
}

func TestPlayerAdapterMetadata(t *testing.T) {
	a := &mockAccessor{
		hasItem:  true,
		title:    "The Hobbit",
		authors:  []string{"J.R.R. Tolkien"},
		itemID:   "item-123",
		duration: 3600,
	}
	p := NewPlayerAdapter(a, PlayerActions{})

	meta, err := p.Metadata()
	if err != nil {
		t.Fatalf("Metadata() error: %v", err)
	}
	if meta.Title != "The Hobbit" {
		t.Errorf("Metadata().Title = %q; want %q", meta.Title, "The Hobbit")
	}
	if len(meta.Artist) != 1 || meta.Artist[0] != "J.R.R. Tolkien" {
		t.Errorf("Metadata().Artist = %v; want [J.R.R. Tolkien]", meta.Artist)
	}
	if meta.Length != types.Microseconds(3600*1_000_000) {
		t.Errorf("Metadata().Length = %d; want %d", meta.Length, 3600*1_000_000)
	}
	expectedPath := dbus.ObjectPath("/org/pine/track/item-123")
	if meta.TrackId != expectedPath {
		t.Errorf("Metadata().TrackId = %q; want %q", meta.TrackId, expectedPath)
	}
}

func TestPlayerAdapterMetadataNoItem(t *testing.T) {
	a := &mockAccessor{}
	p := NewPlayerAdapter(a, PlayerActions{})

	meta, err := p.Metadata()
	if err != nil {
		t.Fatalf("Metadata() error: %v", err)
	}
	if meta.Title != "" {
		t.Errorf("Metadata().Title = %q; want empty", meta.Title)
	}
}

func TestPlayerAdapterPosition(t *testing.T) {
	a := &mockAccessor{position: 42.5}
	p := NewPlayerAdapter(a, PlayerActions{})

	pos, err := p.Position()
	if err != nil {
		t.Fatalf("Position() error: %v", err)
	}
	if pos != 42_500_000 {
		t.Errorf("Position() = %d; want 42500000", pos)
	}
}

func TestPlayerAdapterSeek(t *testing.T) {
	var capturedOffset types.Microseconds
	actions := PlayerActions{
		Seek: func(offset types.Microseconds) error {
			capturedOffset = offset
			return nil
		},
	}
	a := &mockAccessor{}
	p := NewPlayerAdapter(a, actions)

	if err := p.Seek(types.Microseconds(10_000_000)); err != nil {
		t.Fatalf("Seek() error: %v", err)
	}
	if capturedOffset != 10_000_000 {
		t.Errorf("Seek(10_000_000) captured = %d; want 10000000", capturedOffset)
	}
}

func TestPlayerAdapterCanGoNext(t *testing.T) {
	a := &mockAccessor{queueLen: 2}
	p := NewPlayerAdapter(a, PlayerActions{})

	v, err := p.CanGoNext()
	if err != nil {
		t.Fatalf("CanGoNext() error: %v", err)
	}
	if !v {
		t.Error("CanGoNext() = false; want true (queue has items)")
	}

	a.queueLen = 0
	v, err = p.CanGoNext()
	if err != nil {
		t.Fatalf("CanGoNext() error: %v", err)
	}
	if v {
		t.Error("CanGoNext() = true; want false (empty queue)")
	}
}

func TestPlayerAdapterCanControl(t *testing.T) {
	a := &mockAccessor{}
	p := NewPlayerAdapter(a, PlayerActions{})

	v, err := p.CanControl()
	if err != nil {
		t.Fatalf("CanControl() error: %v", err)
	}
	if !v {
		t.Error("CanControl() = false; want true")
	}
}

func TestPlayerAdapterRates(t *testing.T) {
	a := &mockAccessor{speed: 1.5}
	p := NewPlayerAdapter(a, PlayerActions{})

	rate, err := p.Rate()
	if err != nil {
		t.Fatalf("Rate() error: %v", err)
	}
	if rate != 1.5 {
		t.Errorf("Rate() = %v; want 1.5", rate)
	}

	min, err := p.MinimumRate()
	if err != nil || min != 0.5 {
		t.Errorf("MinimumRate() = %v, %v; want 0.5, nil", min, err)
	}

	max, err := p.MaximumRate()
	if err != nil || max != 4.0 {
		t.Errorf("MaximumRate() = %v, %v; want 4.0, nil", max, err)
	}
}
