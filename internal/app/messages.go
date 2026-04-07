package app

import "github.com/Thelost77/pine/internal/abs"

// Screen represents a screen identifier in the application.
type Screen int

const (
	ScreenLogin Screen = iota
	ScreenHome
	ScreenLibrary
	ScreenDetail
	ScreenSearch
	ScreenSeriesList
	ScreenSeries
)

// String returns the display name for a screen.
func (s Screen) String() string {
	switch s {
	case ScreenLogin:
		return "Login"
	case ScreenHome:
		return "Home"
	case ScreenLibrary:
		return "Library"
	case ScreenDetail:
		return "Detail"
	case ScreenSearch:
		return "Search"
	case ScreenSeriesList:
		return "Series"
	case ScreenSeries:
		return "Series"
	default:
		return "Unknown"
	}
}

// NavigateMsg requests navigation to a different screen.
// Data carries optional context (e.g., a library item ID for detail view).
type NavigateMsg struct {
	Screen Screen
}

// BackMsg requests navigation back to the previous screen.
type BackMsg struct{}

// PlaySessionMsg is sent after a play session is successfully started on ABS.
type PlaySessionMsg struct {
	Session   PlaySessionData
	StreamURL string
}

// PlaySessionData carries the data returned from ABS when starting a session.
type PlaySessionData struct {
	SessionID        string
	ItemID           string
	EpisodeID        string
	CurrentTime      float64
	Duration         float64
	Title            string
	Chapters         []abs.Chapter
	TrackStartOffset float64
	TrackDuration    float64
}

// QueueEntry represents a queued book or podcast episode.
type QueueEntry struct {
	Item    abs.LibraryItem
	Episode *abs.PodcastEpisode
}

// PlayerReadyMsg signals that mpv has been launched and connected.
type PlayerReadyMsg struct{}

// SyncTickMsg fires every 30 seconds to sync playback progress with ABS.
type SyncTickMsg struct{}

// PlaybackErrorMsg carries an error from the playback lifecycle.
type PlaybackErrorMsg struct {
	Err error
}

// PlaybackStoppedMsg signals that playback cleanup is complete.
type PlaybackStoppedMsg struct{}

// SleepTimerExpiredMsg fires when the sleep timer reaches zero.
type SleepTimerExpiredMsg struct {
	Generation uint64
}

// EpisodesLoadedMsg carries podcast episodes fetched from the API.
type EpisodesLoadedMsg struct {
	Episodes []abs.PodcastEpisode
	Err      error
}

// BookDetailLoadedMsg carries an enriched library item fetched from ABS.
type BookDetailLoadedMsg struct {
	Item *abs.LibraryItem
	Err  error
}
