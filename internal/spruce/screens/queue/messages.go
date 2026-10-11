package queue

// JumpQueueMsg is emitted when the user selects a track in the queue to play immediately.
type JumpQueueMsg struct {
	Index int
}

// RemoveQueueMsg is emitted when the user removes a track from the queue.
type RemoveQueueMsg struct {
	Index int
}

// QueueActionMsg signals a media control command from the queue screen.
type QueueActionMsg struct {
	Action  string // e.g. "toggle_pause", "clear", "repeat_queue", "subliminal"
	Index   int
	TrackID string
}
