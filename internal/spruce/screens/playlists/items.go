package playlists

import (
	"fmt"

	"github.com/Thelost77/pine/internal/spruce/jellyfin"
	"github.com/Thelost77/pine/internal/spruce/ui"
)

type playlistItem struct {
	Playlist jellyfin.Playlist
}

func (i playlistItem) Title() string { return i.Playlist.Name }
func (i playlistItem) Description() string {
	if i.Playlist.Count > 0 {
		return fmt.Sprintf("%d tracks", i.Playlist.Count)
	}
	return "Playlist"
}
func (i playlistItem) FilterValue() string { return i.Playlist.Name }
func (i playlistItem) FlashID() string     { return i.Playlist.ID }

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
	return fmt.Sprintf("%s • %s", i.Track.DisplayArtist(), ui.FormatDuration(i.Track.Duration()))
}
func (i trackItem) FilterValue() string { return i.Track.Name }
func (i trackItem) FlashID() string     { return i.Track.ID }
