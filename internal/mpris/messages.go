package mpris

// PlayPauseMsg is sent when MPRIS requests a play/pause toggle.
type PlayPauseMsg struct{}

// SeekMsg is sent when MPRIS requests a seek by offset (in seconds).
type SeekMsg struct {
	Offset float64
}

// SetVolumeMsg is sent when MPRIS sets the volume (0-150 scale).
type SetVolumeMsg struct {
	Volume int
}

// SetRateMsg is sent when MPRIS sets the playback rate.
type SetRateMsg struct {
	Rate float64
}
