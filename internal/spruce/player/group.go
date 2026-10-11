package player

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"sync/atomic"
	"time"

	tea "github.com/charmbracelet/bubbletea"
)

// MaxSubliminalTracks prevents accidental unbounded mpv process creation.
const MaxSubliminalTracks = 8

type groupMember interface {
	LaunchLoop(path, socketPath string, paused bool, volume int) error
	Connect() error
	SetPause(bool) error
	SetVolume(int) error
	Quit() error
	Done() <-chan struct{}
}

// Group manages independently looping mpv processes as one playback unit.
type Group struct {
	opMu      sync.Mutex
	mu        sync.RWMutex
	players   []groupMember
	newPlayer func() groupMember
}

var groupSocketSequence atomic.Uint64

func NewGroup() *Group {
	return &Group{newPlayer: func() groupMember { return NewMpv() }}
}

func newGroupWithFactory(factory func() groupMember) *Group {
	return &Group{newPlayer: factory}
}

// Launch starts every path paused, then resumes the complete group together.
func (g *Group) Launch(ctx context.Context, paths []string, volume int) error {
	if g == nil {
		return errors.New("player group is unavailable")
	}
	if ctx == nil {
		return errors.New("player group context is nil")
	}
	if len(paths) == 0 {
		return errors.New("player group has no media files")
	}
	if len(paths) > MaxSubliminalTracks {
		return fmt.Errorf("player group has %d tracks; maximum is %d", len(paths), MaxSubliminalTracks)
	}

	g.opMu.Lock()
	defer g.opMu.Unlock()
	g.quitLocked()

	socketDir, err := MpvSocketDir()
	if err != nil {
		return err
	}
	players := make([]groupMember, 0, len(paths))
	cleanup := func() {
		for _, p := range players {
			_ = p.Quit()
		}
	}

	sequence := groupSocketSequence.Add(1)
	for i, path := range paths {
		if err := ctx.Err(); err != nil {
			cleanup()
			return err
		}
		p := g.newPlayer()
		socketPath := filepath.Join(socketDir, fmt.Sprintf("spruce-mpv-%d-group-%d-%d.sock", os.Getpid(), sequence, i))
		_ = os.Remove(socketPath)
		if err := p.LaunchLoop(path, socketPath, true, volume); err != nil {
			cleanup()
			return fmt.Errorf("launch subliminal track %d: %w", i+1, err)
		}
		players = append(players, p)
		if err := connectPlayer(ctx, p); err != nil {
			cleanup()
			return fmt.Errorf("connect subliminal track %d: %w", i+1, err)
		}
	}
	for _, p := range players {
		if err := p.SetPause(false); err != nil {
			cleanup()
			return fmt.Errorf("start subliminal tracks: %w", err)
		}
	}

	g.mu.Lock()
	g.players = players
	g.mu.Unlock()
	return nil
}

func connectPlayer(ctx context.Context, p groupMember) error {
	var lastErr error
	for i := 0; i < 30; i++ {
		if err := ctx.Err(); err != nil {
			return err
		}
		if lastErr = p.Connect(); lastErr == nil {
			return nil
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(100 * time.Millisecond):
		}
	}
	return fmt.Errorf("connect to mpv: %w", lastErr)
}

func (g *Group) SetPause(paused bool) error {
	return g.forEach(func(p groupMember) error { return p.SetPause(paused) })
}

func (g *Group) SetVolume(volume int) error {
	return g.forEach(func(p groupMember) error { return p.SetVolume(volume) })
}

func (g *Group) forEach(action func(groupMember) error) error {
	if g == nil {
		return errors.New("player group is unavailable")
	}
	g.opMu.Lock()
	defer g.opMu.Unlock()
	g.mu.RLock()
	players := append([]groupMember(nil), g.players...)
	g.mu.RUnlock()
	for _, p := range players {
		if err := action(p); err != nil {
			return err
		}
	}
	return nil
}

// Wait returns when a group member exits or ctx is cancelled.
func (g *Group) Wait(ctx context.Context) error {
	if g == nil {
		return errors.New("player group is unavailable")
	}
	if ctx == nil {
		return errors.New("player group context is nil")
	}
	g.mu.RLock()
	players := append([]groupMember(nil), g.players...)
	g.mu.RUnlock()
	if len(players) == 0 {
		return errors.New("player group is empty")
	}

	watchContext, cancelWatch := context.WithCancel(ctx)
	defer cancelWatch()
	exited := make(chan int, len(players))
	watched := 0
	for index, member := range players {
		done := member.Done()
		if done == nil {
			continue
		}
		watched++
		go func() {
			select {
			case <-watchContext.Done():
			case <-done:
				exited <- index
			}
		}()
	}
	if watched == 0 {
		return errors.New("player group members cannot be monitored")
	}
	select {
	case <-ctx.Done():
		return ctx.Err()
	case index := <-exited:
		if err := ctx.Err(); err != nil {
			return err
		}
		return fmt.Errorf("subliminal track %d exited unexpectedly", index+1)
	}
}

func (g *Group) Quit() error {
	if g == nil {
		return nil
	}
	g.opMu.Lock()
	defer g.opMu.Unlock()
	return g.quitLocked()
}

func (g *Group) quitLocked() error {
	g.mu.Lock()
	players := g.players
	g.players = nil
	g.mu.Unlock()
	var result error
	for _, p := range players {
		if err := p.Quit(); err != nil {
			result = errors.Join(result, err)
		}
	}
	return result
}

type GroupReadyMsg struct {
	Generation uint64
	Err        error
}

type GroupControlMsg struct {
	Generation uint64
	Err        error
}

type GroupEndedMsg struct {
	Generation uint64
	Err        error
}

type GroupStoppedMsg struct {
	Group *Group
}

func LaunchGroupCmd(ctx context.Context, group *Group, paths []string, volume int, generation uint64) tea.Cmd {
	return func() tea.Msg {
		return GroupReadyMsg{Generation: generation, Err: group.Launch(ctx, paths, volume)}
	}
}

func WatchGroupCmd(ctx context.Context, group *Group, generation uint64) tea.Cmd {
	return func() tea.Msg {
		return GroupEndedMsg{Generation: generation, Err: group.Wait(ctx)}
	}
}

func SetGroupPauseCmd(group *Group, paused bool, generation uint64) tea.Cmd {
	return func() tea.Msg {
		return GroupControlMsg{Generation: generation, Err: group.SetPause(paused)}
	}
}

func SetGroupVolumeCmd(group *Group, volume int, generation uint64) tea.Cmd {
	return func() tea.Msg {
		return GroupControlMsg{Generation: generation, Err: group.SetVolume(volume)}
	}
}

func QuitGroupCmd(group *Group) tea.Cmd {
	return func() tea.Msg {
		_ = group.Quit()
		return GroupStoppedMsg{Group: group}
	}
}
