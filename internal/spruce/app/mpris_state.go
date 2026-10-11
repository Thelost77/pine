package app

import "sync"

// MprisState holds playback state shared between the Model and MPRIS accessor.
// The Model writes to it via Update; the MPRIS adapter reads from it via the
// getter methods. All access is guarded by mu.
type MprisState struct {
	mu            sync.RWMutex
	playing       bool
	paused        bool
	hasActiveItem bool
	title         string
	authors       []string
	itemID        string
	position      float64
	duration      float64
	speed         float64
	minimumRate   float64
	maximumRate   float64
	volume        int
	queueLength   int
	canSeek       bool
	canGoNext     bool
	canGoPrevious bool
}

// Update runs fn with the write lock held. fn must not call the getter methods
// on s (or any *MprisState that aliases the same receiver) to avoid deadlocks.
func (s *MprisState) Update(fn func(s *MprisState)) {
	s.mu.Lock()
	defer s.mu.Unlock()
	fn(s)
}

func (s *MprisState) IsPlaying() bool {
	if s == nil {
		return false
	}
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.playing
}

func (s *MprisState) IsPaused() bool {
	if s == nil {
		return false
	}
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.paused
}

func (s *MprisState) HasActiveItem() bool {
	if s == nil {
		return false
	}
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.hasActiveItem
}

func (s *MprisState) CurrentTitle() string {
	if s == nil {
		return ""
	}
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.title
}

func (s *MprisState) CurrentAuthors() []string {
	if s == nil {
		return nil
	}
	s.mu.RLock()
	defer s.mu.RUnlock()
	cp := make([]string, len(s.authors))
	copy(cp, s.authors)
	return cp
}

func (s *MprisState) CurrentItemID() string {
	if s == nil {
		return ""
	}
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.itemID
}

func (s *MprisState) PlayerPosition() float64 {
	if s == nil {
		return 0
	}
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.position
}

func (s *MprisState) PlayerDuration() float64 {
	if s == nil {
		return 0
	}
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.duration
}

func (s *MprisState) PlayerMinimumRate() float64 {
	if s == nil {
		return 1.0
	}
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.minimumRate
}

func (s *MprisState) PlayerMaximumRate() float64 {
	if s == nil {
		return 1.0
	}
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.maximumRate
}

func (s *MprisState) PlayerVolume() int {
	if s == nil {
		return 100
	}
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.volume
}

func (s *MprisState) PlayerSpeed() float64 {
	if s == nil {
		return 1.0
	}
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.speed
}

func (s *MprisState) CanSeek() bool {
	if s == nil {
		return false
	}
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.canSeek
}

func (s *MprisState) CanGoNext() bool {
	if s == nil {
		return false
	}
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.canGoNext
}

func (s *MprisState) CanGoPrevious() bool {
	if s == nil {
		return false
	}
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.canGoPrevious
}

func (s *MprisState) QueueLength() int {
	if s == nil {
		return 0
	}
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.queueLength
}
