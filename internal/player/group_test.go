package player

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"
)

type groupPlayerSpy struct {
	mu          sync.Mutex
	path        string
	pausedStart bool
	volume      int
	pauses      []bool
	quit        bool
	launchErr   error
	done        chan struct{}
}

func (p *groupPlayerSpy) LaunchLoop(path string, paused bool, volume int) error {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.path = path
	p.pausedStart = paused
	p.volume = volume
	return p.launchErr
}
func (p *groupPlayerSpy) SetPause(paused bool) error { p.pauses = append(p.pauses, paused); return nil }
func (p *groupPlayerSpy) SetVolume(volume int) error { p.volume = volume; return nil }
func (p *groupPlayerSpy) Quit() error                { p.quit = true; return nil }
func (p *groupPlayerSpy) Done() <-chan struct{}      { return p.done }

func TestGroupLaunchesPausedAtPerTrackVolumeThenResumes(t *testing.T) {
	var players []*groupPlayerSpy
	group := newGroupWithFactory(func() groupMember {
		p := &groupPlayerSpy{done: make(chan struct{})}
		players = append(players, p)
		return p
	})

	if err := group.Launch(context.Background(), []string{"one.media", "two.media"}, 20); err != nil {
		t.Fatalf("Launch() error: %v", err)
	}
	if len(players) != 2 {
		t.Fatalf("created %d players, want 2", len(players))
	}
	for i, p := range players {
		if !p.pausedStart || p.volume != 20 {
			t.Fatalf("player %d paused=%v volume=%d", i, p.pausedStart, p.volume)
		}
		if len(p.pauses) != 1 || p.pauses[0] {
			t.Fatalf("player %d resume calls = %v", i, p.pauses)
		}
	}

	if err := group.SetVolume(35); err != nil {
		t.Fatalf("SetVolume() error: %v", err)
	}
	if players[0].volume != 35 || players[1].volume != 35 {
		t.Fatalf("volumes = %d, %d", players[0].volume, players[1].volume)
	}
	if err := group.Quit(); err != nil {
		t.Fatalf("Quit() error: %v", err)
	}
	if !players[0].quit || !players[1].quit {
		t.Fatal("Quit() did not stop every player")
	}
}

func TestGroupWaitReportsMemberExit(t *testing.T) {
	member := &groupPlayerSpy{done: make(chan struct{})}
	group := newGroupWithFactory(func() groupMember { return member })
	if err := group.Launch(context.Background(), []string{"one.media"}, 20); err != nil {
		t.Fatalf("Launch() error: %v", err)
	}
	close(member.done)
	if err := group.Wait(context.Background()); err == nil {
		t.Fatal("Wait() did not report member exit")
	}
}

func TestDelayedCommandsCannotAffectAnotherGroup(t *testing.T) {
	firstMember := &groupPlayerSpy{done: make(chan struct{})}
	secondMember := &groupPlayerSpy{done: make(chan struct{})}
	first := newGroupWithFactory(func() groupMember { return firstMember })
	second := newGroupWithFactory(func() groupMember { return secondMember })
	if err := first.Launch(context.Background(), []string{"one.media"}, 20); err != nil {
		t.Fatalf("first Launch() error: %v", err)
	}
	if err := second.Launch(context.Background(), []string{"two.media"}, 20); err != nil {
		t.Fatalf("second Launch() error: %v", err)
	}

	oldVolumeCmd := SetGroupVolumeCmd(first, 75, 1)
	oldQuitCmd := QuitGroupCmd(first)
	_ = oldVolumeCmd()
	_ = oldQuitCmd()
	if secondMember.volume != 20 || secondMember.quit {
		t.Fatalf("old commands affected new group: volume=%d quit=%v", secondMember.volume, secondMember.quit)
	}
}

func TestGroupCleansUpAfterPartialLaunchFailure(t *testing.T) {
	first := &groupPlayerSpy{done: make(chan struct{})}
	second := &groupPlayerSpy{launchErr: errors.New("boom"), done: make(chan struct{})}
	players := []*groupPlayerSpy{first, second}
	next := 0
	group := newGroupWithFactory(func() groupMember {
		p := players[next]
		next++
		return p
	})

	if err := group.Launch(context.Background(), []string{"one.media", "two.media"}, 20); err == nil {
		t.Fatal("Launch() succeeded after partial failure")
	}
	if !first.quit {
		t.Fatal("first player was not stopped after second launch failed")
	}
}

func TestGroupLoopsTracksThroughLibmpv(t *testing.T) {
	requireLibmpv(t)
	dir := t.TempDir()
	var paths []string
	for _, name := range []string{"one.wav", "two.wav"} {
		path := filepath.Join(dir, name)
		if err := os.WriteFile(path, silentWAV(1), 0600); err != nil {
			t.Fatalf("write %s: %v", name, err)
		}
		paths = append(paths, path)
	}
	var members []*Mpv
	group := newGroupWithFactory(func() groupMember {
		m := NewMpv()
		m.audioOutput = "null"
		members = append(members, m)
		return m
	})
	t.Cleanup(func() { _ = group.Quit() })

	if err := group.Launch(context.Background(), paths, 20); err != nil {
		t.Fatalf("Launch() error: %v", err)
	}
	for i, m := range members {
		waitPosition(t, m)
		if paused, err := m.GetPaused(); err != nil || paused {
			t.Errorf("member %d GetPaused() = %v, %v, want false", i, paused, err)
		}
		if volume, err := m.handle.getDouble("volume"); err != nil || volume != 20 {
			t.Errorf("member %d volume = %v, %v, want 20", i, volume, err)
		}
	}

	// Each file is 1 second. A member that does not loop ends before the
	// context does.
	ctx, cancel := context.WithTimeout(context.Background(), 2500*time.Millisecond)
	defer cancel()
	if err := group.Wait(ctx); !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("Wait() = %v, want the context deadline", err)
	}

	if err := group.Quit(); err != nil {
		t.Fatalf("Quit() error: %v", err)
	}
	for i, m := range members {
		if _, err := m.GetPosition(); !errors.Is(err, ErrClosed) {
			t.Errorf("member %d GetPosition() after Quit() error = %v, want ErrClosed", i, err)
		}
	}
}
