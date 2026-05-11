package mpris

import (
	"fmt"

	"github.com/godbus/dbus/v5"
	"github.com/quarckster/go-mpris-server/pkg/types"
)

// RootAdapter implements types.OrgMprisMediaPlayer2Adapter with static values.
type RootAdapter struct{}

func (r RootAdapter) Raise() error                       { return nil }
func (r RootAdapter) Quit() error                        { return nil }
func (r RootAdapter) CanQuit() (bool, error)             { return true, nil }
func (r RootAdapter) CanRaise() (bool, error)            { return false, nil }
func (r RootAdapter) HasTrackList() (bool, error)        { return false, nil }
func (r RootAdapter) Identity() (string, error)          { return "pine", nil }
func (r RootAdapter) SupportedUriSchemes() ([]string, error) { return nil, nil }
func (r RootAdapter) SupportedMimeTypes() ([]string, error)  { return nil, nil }

// PlayerAdapter implements types.OrgMprisMediaPlayer2PlayerAdapter using closures.
// Each method field is a closure that reads from a ModelAccessor.
type PlayerAdapter struct {
	OnNext        func() error
	OnPrevious    func() error
	OnPause        func() error
	OnPlayPause    func() error
	OnStop         func() error
	OnPlay         func() error
	OnSeek         func(offset types.Microseconds) error
	OnSetPosition  func(trackId string, position types.Microseconds) error
	OnOpenUri      func(uri string) error
	OnPlaybackStatus func() (types.PlaybackStatus, error)
	OnRate         func() (float64, error)
	OnSetRate      func(rate float64) error
	OnMetadata     func() (types.Metadata, error)
	OnVolume       func() (float64, error)
	OnSetVolume    func(vol float64) error
	OnPosition     func() (int64, error)
	OnMinimumRate  func() (float64, error)
	OnMaximumRate  func() (float64, error)
	OnCanGoNext    func() (bool, error)
	OnCanGoPrevious func() (bool, error)
	OnCanPlay      func() (bool, error)
	OnCanPause     func() (bool, error)
	OnCanSeek      func() (bool, error)
	OnCanControl   func() (bool, error)
}

func (p PlayerAdapter) Next() error                              { return p.OnNext() }
func (p PlayerAdapter) Previous() error                          { return p.OnPrevious() }
func (p PlayerAdapter) Pause() error                             { return p.OnPause() }
func (p PlayerAdapter) PlayPause() error                         { return p.OnPlayPause() }
func (p PlayerAdapter) Stop() error                              { return p.OnStop() }
func (p PlayerAdapter) Play() error                              { return p.OnPlay() }
func (p PlayerAdapter) Seek(offset types.Microseconds) error     { return p.OnSeek(offset) }
func (p PlayerAdapter) SetPosition(trackId string, position types.Microseconds) error {
	return p.OnSetPosition(trackId, position)
}
func (p PlayerAdapter) OpenUri(uri string) error                 { return p.OnOpenUri(uri) }
func (p PlayerAdapter) PlaybackStatus() (types.PlaybackStatus, error) { return p.OnPlaybackStatus() }
func (p PlayerAdapter) Rate() (float64, error)                   { return p.OnRate() }
func (p PlayerAdapter) SetRate(rate float64) error               { return p.OnSetRate(rate) }
func (p PlayerAdapter) Metadata() (types.Metadata, error)        { return p.OnMetadata() }
func (p PlayerAdapter) Volume() (float64, error)                 { return p.OnVolume() }
func (p PlayerAdapter) SetVolume(vol float64) error              { return p.OnSetVolume(vol) }
func (p PlayerAdapter) Position() (int64, error)                 { return p.OnPosition() }
func (p PlayerAdapter) MinimumRate() (float64, error)            { return p.OnMinimumRate() }
func (p PlayerAdapter) MaximumRate() (float64, error)            { return p.OnMaximumRate() }
func (p PlayerAdapter) CanGoNext() (bool, error)                 { return p.OnCanGoNext() }
func (p PlayerAdapter) CanGoPrevious() (bool, error)             { return p.OnCanGoPrevious() }
func (p PlayerAdapter) CanPlay() (bool, error)                   { return p.OnCanPlay() }
func (p PlayerAdapter) CanPause() (bool, error)                  { return p.OnCanPause() }
func (p PlayerAdapter) CanSeek() (bool, error)                   { return p.OnCanSeek() }
func (p PlayerAdapter) CanControl() (bool, error)                { return p.OnCanControl() }

// NewPlayerAdapter creates a PlayerAdapter whose closures read from the accessor.
func NewPlayerAdapter(accessor ModelAccessor, actions PlayerActions) PlayerAdapter {
	return PlayerAdapter{
		OnNext:     actions.Next,
		OnPrevious: actions.Previous,
		OnPause:    actions.Pause,
		OnPlayPause: actions.PlayPause,
		OnStop:     actions.Stop,
		OnPlay:     actions.Play,
		OnSeek:     actions.Seek,
		OnSetPosition: actions.SetPosition,
		OnOpenUri:  func(string) error { return nil },
		OnPlaybackStatus: func() (types.PlaybackStatus, error) {
			if accessor.IsPlaying() {
				return types.PlaybackStatusPlaying, nil
			}
			if accessor.IsPaused() {
				return types.PlaybackStatusPaused, nil
			}
			return types.PlaybackStatusStopped, nil
		},
		OnRate:    func() (float64, error) { return accessor.PlayerSpeed(), nil },
		OnSetRate: actions.SetRate,
		OnMetadata: func() (types.Metadata, error) {
			if !accessor.HasActiveItem() {
				return types.Metadata{}, nil
			}
			itemID := accessor.CurrentItemID()
			trackID := dbus.ObjectPath(fmt.Sprintf("/org/pine/track/%s", itemID))
			durationMicro := types.Microseconds(accessor.PlayerDuration() * 1_000_000)
			return types.Metadata{
				TrackId: trackID,
				Length:  durationMicro,
				Title:   accessor.CurrentTitle(),
				Artist:  accessor.CurrentAuthors(),
			}, nil
		},
		OnVolume: func() (float64, error) {
			return float64(accessor.PlayerVolume()) / 100.0, nil
		},
		OnSetVolume: func(vol float64) error {
			return actions.SetVolume(int(vol * 100))
		},
		OnPosition: func() (int64, error) {
			return int64(accessor.PlayerPosition() * 1_000_000), nil
		},
		OnMinimumRate:  func() (float64, error) { return 0.5, nil },
		OnMaximumRate:  func() (float64, error) { return 4.0, nil },
		OnCanGoNext:    func() (bool, error) { return accessor.QueueLength() > 0, nil },
		OnCanGoPrevious: func() (bool, error) { return accessor.HasActiveItem(), nil },
		OnCanPlay:      func() (bool, error) { return accessor.HasActiveItem(), nil },
		OnCanPause:     func() (bool, error) { return accessor.HasActiveItem(), nil },
		OnCanSeek:      func() (bool, error) { return accessor.HasActiveItem(), nil },
		OnCanControl:   func() (bool, error) { return true, nil },
	}
}

// PlayerActions defines the action callbacks for the PlayerAdapter.
type PlayerActions struct {
	Next        func() error
	Previous    func() error
	Pause       func() error
	PlayPause   func() error
	Stop        func() error
	Play        func() error
	Seek        func(offset types.Microseconds) error
	SetPosition func(trackId string, position types.Microseconds) error
	SetRate     func(rate float64) error
	SetVolume   func(vol int) error
}
