package app

import "github.com/Thelost77/spruce/internal/jellyfin"

type Screen int

const (
	ScreenLogin Screen = iota
	ScreenLibrary
	ScreenPlaylists
	ScreenQueue
	ScreenMetadataEdit
)

// SwitchScreenMsg requests switching to a specific screen.
type SwitchScreenMsg struct {
	Screen Screen
}

// NavigateMsg requests navigation to a different screen, pushing the current
// one onto the back stack.
type NavigateMsg struct {
	Target Screen
}

// BackMsg requests navigation back to the previous screen on the back stack.
type BackMsg struct{}

// EditMetadataMsg requests opening the metadata editor for a track or album.
type EditMetadataMsg struct {
	ItemID   string
	ItemType string // "Track" or "Album"
	Track    *jellyfin.Track
	Album    *jellyfin.Album
}

// SleepTimerExpiredMsg fires when the sleep timer reaches zero.
type SleepTimerExpiredMsg struct {
	Generation uint64
}

type repeatCacheMsg struct {
	Generation uint64
	ItemID     string
	Index      int
	Start      bool
	Err        error
}

type subliminalCacheMsg struct {
	Generation uint64
	ItemID     string
	Err        error
}
