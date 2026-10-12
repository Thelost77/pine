package mpris

import (
	"fmt"
	"strings"
	"time"

	"github.com/godbus/dbus/v5"
)

// PlaybackStatus is a value of the MPRIS PlaybackStatus property.
type PlaybackStatus string

const (
	StatusPlaying PlaybackStatus = "Playing"
	StatusPaused  PlaybackStatus = "Paused"
	StatusStopped PlaybackStatus = "Stopped"
)

// Metadata is the metadata of the current track.
type Metadata struct {
	TrackID dbus.ObjectPath
	Length  time.Duration
	Title   string
	Artist  []string
}

// RootAdapter supplies the org.mpris.MediaPlayer2 interface with static values.
// Name is the app name that MPRIS clients see.
type RootAdapter struct {
	Name string
}

func (r RootAdapter) Raise() error                           { return nil }
func (r RootAdapter) Quit() error                            { return nil }
func (r RootAdapter) CanQuit() (bool, error)                 { return true, nil }
func (r RootAdapter) CanRaise() (bool, error)                { return false, nil }
func (r RootAdapter) HasTrackList() (bool, error)            { return false, nil }
func (r RootAdapter) Identity() (string, error)              { return r.Name, nil }
func (r RootAdapter) SupportedUriSchemes() ([]string, error) { return nil, nil }
func (r RootAdapter) SupportedMimeTypes() ([]string, error)  { return nil, nil }
func (r RootAdapter) DesktopEntry() (string, error)          { return r.Name, nil }

// PlayerAdapter supplies the org.mpris.MediaPlayer2.Player interface using closures.
// Each method field is a closure that reads from a ModelAccessor.
type PlayerAdapter struct {
	OnNext           func() error
	OnPrevious       func() error
	OnPause          func() error
	OnPlayPause      func() error
	OnStop           func() error
	OnPlay           func() error
	OnSeek           func(offset time.Duration) error
	OnSetPosition    func(trackId string, position time.Duration) error
	OnOpenUri        func(uri string) error
	OnPlaybackStatus func() (PlaybackStatus, error)
	OnRate           func() (float64, error)
	OnSetRate        func(rate float64) error
	OnMetadata       func() (Metadata, error)
	OnVolume         func() (float64, error)
	OnSetVolume      func(vol float64) error
	OnPosition       func() (int64, error)
	OnMinimumRate    func() (float64, error)
	OnMaximumRate    func() (float64, error)
	OnCanGoNext      func() (bool, error)
	OnCanGoPrevious  func() (bool, error)
	OnCanPlay        func() (bool, error)
	OnCanPause       func() (bool, error)
	OnCanSeek        func() (bool, error)
	OnCanControl     func() (bool, error)
}

func (p PlayerAdapter) Next() error                     { return p.OnNext() }
func (p PlayerAdapter) Previous() error                 { return p.OnPrevious() }
func (p PlayerAdapter) Pause() error                    { return p.OnPause() }
func (p PlayerAdapter) PlayPause() error                { return p.OnPlayPause() }
func (p PlayerAdapter) Stop() error                     { return p.OnStop() }
func (p PlayerAdapter) Play() error                     { return p.OnPlay() }
func (p PlayerAdapter) Seek(offset time.Duration) error { return p.OnSeek(offset) }
func (p PlayerAdapter) SetPosition(trackId string, position time.Duration) error {
	return p.OnSetPosition(trackId, position)
}
func (p PlayerAdapter) OpenUri(uri string) error                { return p.OnOpenUri(uri) }
func (p PlayerAdapter) PlaybackStatus() (PlaybackStatus, error) { return p.OnPlaybackStatus() }
func (p PlayerAdapter) Rate() (float64, error)                  { return p.OnRate() }
func (p PlayerAdapter) SetRate(rate float64) error              { return p.OnSetRate(rate) }
func (p PlayerAdapter) Metadata() (Metadata, error)             { return p.OnMetadata() }
func (p PlayerAdapter) Volume() (float64, error)                { return p.OnVolume() }
func (p PlayerAdapter) SetVolume(vol float64) error             { return p.OnSetVolume(vol) }
func (p PlayerAdapter) Position() (int64, error)                { return p.OnPosition() }
func (p PlayerAdapter) MinimumRate() (float64, error)           { return p.OnMinimumRate() }
func (p PlayerAdapter) MaximumRate() (float64, error)           { return p.OnMaximumRate() }
func (p PlayerAdapter) CanGoNext() (bool, error)                { return p.OnCanGoNext() }
func (p PlayerAdapter) CanGoPrevious() (bool, error)            { return p.OnCanGoPrevious() }
func (p PlayerAdapter) CanPlay() (bool, error)                  { return p.OnCanPlay() }
func (p PlayerAdapter) CanPause() (bool, error)                 { return p.OnCanPause() }
func (p PlayerAdapter) CanSeek() (bool, error)                  { return p.OnCanSeek() }
func (p PlayerAdapter) CanControl() (bool, error)               { return p.OnCanControl() }

// NewPlayerAdapter creates a PlayerAdapter whose closures read from the accessor.
// name is the app name used in track object paths.
// accessor is a function that returns the current ModelAccessor, called on each read.
func NewPlayerAdapter(name string, accessor func() ModelAccessor, actions PlayerActions) PlayerAdapter {
	return PlayerAdapter{
		OnNext:        actions.Next,
		OnPrevious:    actions.Previous,
		OnPause:       actions.Pause,
		OnPlayPause:   actions.PlayPause,
		OnStop:        actions.Stop,
		OnPlay:        actions.Play,
		OnSeek:        actions.Seek,
		OnSetPosition: actions.SetPosition,
		OnOpenUri:     func(string) error { return nil },
		OnPlaybackStatus: func() (PlaybackStatus, error) {
			a := accessor()
			if a.IsPlaying() {
				return StatusPlaying, nil
			}
			if a.IsPaused() {
				return StatusPaused, nil
			}
			return StatusStopped, nil
		},
		OnRate:    func() (float64, error) { return accessor().PlayerSpeed(), nil },
		OnSetRate: actions.SetRate,
		OnMetadata: func() (Metadata, error) {
			a := accessor()
			title := a.CurrentTitle()
			if title == "" {
				return Metadata{
					TrackID: dbus.ObjectPath("/org/mpris/MediaPlayer2/NoTrack"),
				}, nil
			}
			return Metadata{
				TrackID: trackPath(name, a.CurrentItemID()),
				Length:  time.Duration(a.PlayerDuration() * float64(time.Second)),
				Title:   title,
				Artist:  a.CurrentAuthors(),
			}, nil
		},
		OnVolume: func() (float64, error) {
			return float64(accessor().PlayerVolume()) / 100.0, nil
		},
		OnSetVolume: func(vol float64) error {
			return actions.SetVolume(int(vol * 100))
		},
		OnPosition: func() (int64, error) {
			return int64(accessor().PlayerPosition() * 1_000_000), nil
		},
		OnMinimumRate:   func() (float64, error) { return accessor().PlayerMinimumRate(), nil },
		OnMaximumRate:   func() (float64, error) { return accessor().PlayerMaximumRate(), nil },
		OnCanGoNext:     func() (bool, error) { return accessor().CanGoNext(), nil },
		OnCanGoPrevious: func() (bool, error) { return accessor().CanGoPrevious(), nil },
		OnCanPlay:       func() (bool, error) { return true, nil },
		OnCanPause:      func() (bool, error) { return true, nil },
		OnCanSeek:       func() (bool, error) { return accessor().CanSeek(), nil },
		OnCanControl:    func() (bool, error) { return true, nil },
	}
}

// trackPath returns the D-Bus object path that identifies an item as an MPRIS track.
func trackPath(name, itemID string) dbus.ObjectPath {
	return dbus.ObjectPath(fmt.Sprintf("/org/%s/track/%s", name, strings.ReplaceAll(itemID, "-", "_")))
}

// PlayerActions defines the action callbacks for the PlayerAdapter.
type PlayerActions struct {
	Next        func() error
	Previous    func() error
	Pause       func() error
	PlayPause   func() error
	Stop        func() error
	Play        func() error
	Seek        func(offset time.Duration) error
	SetPosition func(trackId string, position time.Duration) error
	SetRate     func(rate float64) error
	SetVolume   func(vol int) error
}
