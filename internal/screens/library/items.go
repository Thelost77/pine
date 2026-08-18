package library

import (
	"fmt"
	"strings"

	"github.com/Thelost77/spruce/internal/jellyfin"
	"github.com/Thelost77/spruce/internal/ui"
)

// artistItem wraps jellyfin.Artist for bubbles list.
type artistItem struct {
	Artist jellyfin.Artist
}

func (i artistItem) Title() string       { return i.Artist.Name }
func (i artistItem) Description() string { return "Artist" }
func (i artistItem) FilterValue() string { return i.Artist.Name }

// albumItem wraps jellyfin.Album for bubbles list.
type albumItem struct {
	Album jellyfin.Album
}

func (i albumItem) Title() string {
	if i.Album.UserData.IsFavorite {
		return "♥ " + i.Album.Name
	}
	return i.Album.Name
}
func (i albumItem) Description() string {
	year := ""
	if i.Album.ProductionYear > 0 {
		year = fmt.Sprintf(" (%d)", i.Album.ProductionYear)
	}
	artist := "Unknown Artist"
	if len(i.Album.Artists) > 0 {
		artist = strings.Join(i.Album.Artists, ", ")
	}
	return artist + year
}
func (i albumItem) FilterValue() string { return i.Album.Name }
func (i albumItem) FlashID() string     { return i.Album.ID }

// trackItem wraps jellyfin.Track for bubbles list.
type trackItem struct {
	Track jellyfin.Track
}

func (i trackItem) Title() string {
	prefix := ""
	if i.Track.UserData.IsFavorite {
		prefix = "♥ "
	}
	if i.Track.IndexNumber > 0 {
		return fmt.Sprintf("%s%d. %s", prefix, i.Track.IndexNumber, i.Track.Name)
	}
	return prefix + i.Track.Name
}
func (i trackItem) Description() string {
	dur := ui.FormatDuration(i.Track.Duration())
	return fmt.Sprintf("%s • %s", i.Track.DisplayArtist(), dur)
}
func (i trackItem) FilterValue() string { return i.Track.Name }
func (i trackItem) FlashID() string     { return i.Track.ID }
