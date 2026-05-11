package app

// MprisState holds playback state shared between the Model and MPRIS accessor.
// The Model writes to it; the MPRIS adapter reads from it.
type MprisState struct {
	IsPlaying       bool
	IsPaused        bool
	HasActiveItem   bool
	Title           string
	Authors         []string
	ItemID          string
	Position        float64
	Duration        float64
	Volume          int
	Speed           float64
	QueueLength     int
}
