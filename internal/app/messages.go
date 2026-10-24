package app

import (
	"github.com/Thelost77/pine/internal/abs"
	"github.com/Thelost77/pine/internal/captions"
)

// Screen represents a screen identifier in the application.
type Screen int

const (
	ScreenLogin Screen = iota
	ScreenHome
	ScreenLibrary
	ScreenDetail
	ScreenMetadataEdit
	ScreenSeriesList
	ScreenSeries
	ScreenSessions
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
	case ScreenMetadataEdit:
		return "Metadata"
	case ScreenSeriesList:
		return "Series"
	case ScreenSeries:
		return "Series"
	case ScreenSessions:
		return "Sessions"
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
	AuthToken string
}

// PlaySessionData carries the data returned from ABS when starting a session.
type PlaySessionData struct {
	SessionID        string
	ItemID           string
	EpisodeID        string
	CurrentTime      float64
	SyncedTime       float64 // book position that the server has
	Duration         float64
	Title            string
	Authors          []string
	Chapters         []abs.Chapter
	TrackStartOffset float64
	TrackDuration    float64
	TrackFilename    string
	TranscriptIno    string
	HasLibraryFiles  bool // the session gave the files of the item
}

// QueueEntry represents a queued book or podcast episode.
type QueueEntry struct {
	Item    abs.LibraryItem
	Episode *abs.PodcastEpisode
}

// SyncTickMsg fires every 30 seconds to sync playback progress with ABS.
// Generation is the playGeneration of the playback that started the ticks.
type SyncTickMsg struct {
	Generation uint64
}

// SessionSyncedMsg carries the result of a sync of the playback session.
// NewSessionID is the session that pine opened because the server did not
// have the session SessionID.
type SessionSyncedMsg struct {
	SessionID    string
	NewSessionID string
	CurrentTime  float64
	TimeListened float64
	Err          error
}

// PlaybackErrorMsg carries an error from the playback lifecycle.
type PlaybackErrorMsg struct {
	Err error
}

// PlaybackStoppedMsg signals that playback cleanup is complete.
type PlaybackStoppedMsg struct{}

// SessionItemLoadedMsg carries the item of a listening session, so that it
// plays from Position. Episode is nil for a book.
type SessionItemLoadedMsg struct {
	Item     abs.LibraryItem
	Episode  *abs.PodcastEpisode
	Position float64
}

// SleepTimerExpiredMsg fires when the sleep timer reaches zero.
type SleepTimerExpiredMsg struct {
	Generation uint64
}

// EpisodesLoadedMsg carries podcast episodes fetched from the API.
type EpisodesLoadedMsg struct {
	ItemID   string
	Episodes []abs.PodcastEpisode
	Err      error
}

// BookDetailLoadedMsg carries an enriched library item fetched from ABS.
type BookDetailLoadedMsg struct {
	ItemID string
	Item   *abs.LibraryItem
	Err    error
}

// RestoreSessionMsg carries the result of a session restore attempt.
// If Item is nil, no session was found or the restore failed — the app
// proceeds to the Home screen as usual.
type RestoreSessionMsg struct {
	Item           *abs.LibraryItem
	Episode        *abs.PodcastEpisode
	SavedEpisodeID string
}

// RestorePlaySessionMsg carries a restore-initiated play session that should
// launch paused.
type RestorePlaySessionMsg struct {
	PlaySessionMsg PlaySessionMsg
}

// SeriesContinueMsg carries the next book in a series after auto-continue lookup.
// Empty Item.ID with nil Err means the finished book was the last in the series.
type SeriesContinueMsg struct {
	Item abs.LibraryItem
	Err  error
}

// PrewarmDoneMsg is sent after cache pre-warming completes. No action needed,
// just used to keep the command chain alive.
type PrewarmDoneMsg struct{}

// CaptionLoadedMsg carries parsed VTT cues for the current play generation.
type CaptionLoadedMsg struct {
	Generation uint64
	Cues       []captions.Cue
}
