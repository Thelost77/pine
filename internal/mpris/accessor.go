package mpris

// ModelAccessor provides read-only access to pine's playback state.
type ModelAccessor interface {
	IsPlaying() bool
	IsPaused() bool
	HasActiveItem() bool
	CurrentTitle() string
	CurrentAuthors() []string
	CurrentItemID() string
	PlayerPosition() float64
	PlayerDuration() float64
	PlayerVolume() int
	PlayerSpeed() float64
	QueueLength() int
}
