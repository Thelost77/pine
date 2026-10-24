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

// playbackStatus returns the MPRIS playback status of the app state.
func playbackStatus(a ModelAccessor) PlaybackStatus {
	if a.IsPlaying() {
		return StatusPlaying
	}
	if a.IsPaused() {
		return StatusPaused
	}
	return StatusStopped
}

// Metadata is the metadata of the current track.
type Metadata struct {
	TrackID dbus.ObjectPath
	Length  time.Duration
	Title   string
	Artist  []string
}

// trackMetadata returns the metadata of the current track of the app state.
// name is the app name used in track object paths.
func trackMetadata(name string, a ModelAccessor) Metadata {
	title := a.CurrentTitle()
	if title == "" {
		return Metadata{
			TrackID: dbus.ObjectPath("/org/mpris/MediaPlayer2/TrackList/NoTrack"),
		}
	}
	return Metadata{
		TrackID: trackPath(name, a.CurrentItemID()),
		Length:  time.Duration(a.PlayerDuration() * float64(time.Second)),
		Title:   title,
		Artist:  a.CurrentAuthors(),
	}
}

// RootAdapter supplies the org.mpris.MediaPlayer2 interface with static values.
// Name is the app name that MPRIS clients see. Quit and Raise do nothing, so
// CanQuit and CanRaise are false.
type RootAdapter struct {
	Name string
}

func (r RootAdapter) Raise() error                           { return nil }
func (r RootAdapter) Quit() error                            { return nil }
func (r RootAdapter) CanQuit() (bool, error)                 { return false, nil }
func (r RootAdapter) CanRaise() (bool, error)                { return false, nil }
func (r RootAdapter) HasTrackList() (bool, error)            { return false, nil }
func (r RootAdapter) Identity() (string, error)              { return r.Name, nil }
func (r RootAdapter) SupportedUriSchemes() ([]string, error) { return nil, nil }
func (r RootAdapter) SupportedMimeTypes() ([]string, error)  { return nil, nil }
func (r RootAdapter) DesktopEntry() (string, error)          { return r.Name, nil }

// trackPath returns the D-Bus object path that identifies an item as an MPRIS track.
func trackPath(name, itemID string) dbus.ObjectPath {
	return dbus.ObjectPath(fmt.Sprintf("/org/%s/track/%s", name, strings.ReplaceAll(itemID, "-", "_")))
}

// PlayerActions defines the callbacks of the methods and the writable
// properties of the org.mpris.MediaPlayer2.Player interface.
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
	// SetVolume gets the MPRIS volume, where 1.0 is the app volume 100.
	SetVolume func(volume float64) error
}
