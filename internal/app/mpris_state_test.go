package app

import (
	"sync"
	"testing"
	"time"

	"github.com/Thelost77/pine/internal/mpris"
)

// TestMprisStateConcurrentAccess verifies that the ModelAccessor getter methods
// can be read concurrently while the state is being updated via Update, without
// racing or deadlocking.
func TestMprisStateConcurrentAccess(t *testing.T) {
	state := &MprisState{}

	// Compile-time: *MprisState implements mpris.ModelAccessor.
	var _ mpris.ModelAccessor = state

	const writers = 4
	const readers = 8
	const iterations = 500

	var wg sync.WaitGroup

	// Writers: mutate state through the locked Update path.
	for w := 0; w < writers; w++ {
		wg.Add(1)
		go func(seed int) {
			defer wg.Done()
			for i := 0; i < iterations; i++ {
				state.Update(func(s *MprisState) {
					s.playing = (i+seed)%2 == 0
					s.paused = (i+seed)%2 == 1
					s.hasActiveItem = true
					s.title = "title"
					s.authors = []string{"a", "b", "c"}
					s.itemID = "id"
					s.position = float64(i)
					s.duration = float64(i) * 2
					s.volume = i % 101
					s.speed = 1.0
					s.queueLength = i % 10
				})
			}
		}(w)
	}

	// Readers: exercise every getter concurrently.
	for r := 0; r < readers; r++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for i := 0; i < iterations; i++ {
				_ = state.IsPlaying()
				_ = state.IsPaused()
				_ = state.HasActiveItem()
				_ = state.CurrentTitle()
				_ = state.CurrentAuthors()
				_ = state.CurrentItemID()
				_ = state.PlayerPosition()
				_ = state.PlayerDuration()
				_ = state.PlayerVolume()
				_ = state.PlayerSpeed()
				_ = state.QueueLength()
			}
		}()
	}

	doneCh := make(chan struct{})
	go func() {
		wg.Wait()
		close(doneCh)
	}()
	select {
	case <-doneCh:
	case <-time.After(10 * time.Second):
		t.Fatal("deadlock or hang in MprisState concurrent access")
	}

	// Final sanity: state written via Update is observable via getters.
	state.Update(func(s *MprisState) {
		s.title = "done"
		s.authors = []string{"x", "y"}
	})
	if got := state.CurrentTitle(); got != "done" {
		t.Fatalf("CurrentTitle = %q, want done", got)
	}
	if got := state.CurrentAuthors(); len(got) != 2 || got[0] != "x" || got[1] != "y" {
		t.Fatalf("CurrentAuthors = %#v, want [x y]", got)
	}
}