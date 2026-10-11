package mpris

// PlayPauseMsg is sent when MPRIS requests a play/pause toggle.
type PlayPauseMsg struct{}

// PauseMsg is sent when MPRIS requests playback to pause.
type PauseMsg struct{}

// PlayMsg is sent when MPRIS requests playback to resume.
type PlayMsg struct{}

// SeekMsg is sent when MPRIS requests a seek by offset (in seconds).
type SeekMsg struct {
	Offset float64
}

// SeekRelativeMsg is sent when MPRIS requests a relative seek by offset (in seconds).
type SeekRelativeMsg struct {
	OffsetSeconds float64
}

// SeekAbsoluteMsg is sent when MPRIS requests setting position to an absolute timestamp (in seconds).
type SeekAbsoluteMsg struct {
	TrackID         string
	PositionSeconds float64
}

// SetVolumeMsg is sent when MPRIS sets the volume (0-150 scale).
type SetVolumeMsg struct {
	Volume int
}

// SetRateMsg is sent when MPRIS sets the playback rate.
type SetRateMsg struct {
	Rate float64
}

// NextMsg is sent when MPRIS requests skipping to the next track.
type NextMsg struct{}

// PreviousMsg is sent when MPRIS requests skipping to the previous track.
type PreviousMsg struct{}
