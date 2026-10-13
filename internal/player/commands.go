package player

import (
	"fmt"
	"time"

	"github.com/Thelost77/pine/internal/logger"
	tea "github.com/charmbracelet/bubbletea"
)

// PositionMsg carries the current playback position polled from mpv.
type PositionMsg struct {
	Position float64
	Duration float64
	Paused   bool
	// Idle is true if no audio plays now. A player that waits for data is
	// idle and not paused.
	Idle       bool
	Err        error
	Generation uint64 // ties this tick to a specific play session
}

// PlayerEndMsg is emitted when the current playback ends: the file finished
// (eof), failed to load or decode (error), or the player stopped (quit). The
// app uses the Reason to decide whether to auto-advance the queue.
type PlayerEndMsg struct {
	Generation uint64
	Reason     string // "eof" | "error" | "quit" | "stop" | "redirect" | "unknown"
	Err        error
}

// CommandErrMsg reports that the player did not accept a new pause state, a
// new speed, or a new volume. The app then shows a state that the player does
// not have. A seek that fails needs no message: the next position tick has
// the position of the player.
type CommandErrMsg struct {
	Err error
}

// TickCmd returns a command that fires twice per second and polls mpv state.
// The generation parameter ties the tick to a specific play session so stale
// ticks from a previous session can be safely ignored.
func TickCmd(p Player, generation uint64) tea.Cmd {
	return TickAfterCmd(p, generation, 500*time.Millisecond)
}

// TickAfterCmd returns a command that polls mpv state after the interval. An
// app uses it to wait longer while mpv has no position yet.
func TickAfterCmd(p Player, generation uint64, interval time.Duration) tea.Cmd {
	return tea.Tick(interval, func(_ time.Time) tea.Msg {
		pos, posErr := p.GetPosition()
		dur, durErr := p.GetDuration()
		paused, pauseErr := p.GetPaused()
		idle, idleErr := p.GetIdle()

		// Return first error encountered
		for _, err := range []error{posErr, durErr, pauseErr, idleErr} {
			if err != nil {
				return PositionMsg{Err: err, Generation: generation}
			}
		}

		return PositionMsg{
			Position:   pos,
			Duration:   dur,
			Paused:     paused,
			Idle:       idle,
			Generation: generation,
		}
	})
}

// SetPauseCmd sets mpv's paused state.
func SetPauseCmd(p Player, paused bool) tea.Cmd {
	return func() tea.Msg {
		err := p.SetPause(paused)
		if err != nil {
			return CommandErrMsg{Err: err}
		}
		return nil
	}
}

// SetSpeedCmd sends a speed change to mpv.
func SetSpeedCmd(p Player, speed float64) tea.Cmd {
	return func() tea.Msg {
		err := p.SetSpeed(speed)
		if err != nil {
			return CommandErrMsg{Err: err}
		}
		return nil
	}
}

// SetVolumeCmd sends a volume change to mpv.
func SetVolumeCmd(p Player, vol int) tea.Cmd {
	return func() tea.Msg {
		err := p.SetVolume(vol)
		if err != nil {
			return CommandErrMsg{Err: err}
		}
		return nil
	}
}

// PlayerReadyMsg signals that mpv has accepted the file.
type PlayerReadyMsg struct {
	Generation uint64
}

// PlayerLaunchErrMsg signals that mpv failed to launch.
type PlayerLaunchErrMsg struct {
	Generation uint64
	Err        error
}

// PlayerQuitMsg signals that mpv has been quit.
type PlayerQuitMsg struct{}

// formatStartPosition builds mpv's --start value.
// If duration is known and start is past 0, it uses a percent so mpv can jump
// by byte offset instead of scanning to a timestamp.
func formatStartPosition(startTime, duration float64) string {
	if duration > 0 && startTime > 0 {
		pct := startTime / duration * 100
		if pct > 100 {
			pct = 100
		}
		return fmt.Sprintf("%f%%", pct)
	}
	return fmt.Sprintf("%f", startTime)
}

// LaunchCmd starts playback. If paused is true, mpv starts paused.
// httpHeaders are the HTTP request headers of the stream.
// duration is the current track length in seconds; it is used to start at a
// percent offset when resuming into a long HTTP stream. Pass 0 to start at
// startTime in seconds.
// Returns PlayerReadyMsg when mpv has accepted the file. A file that fails to
// load ends the playback later.
func LaunchCmd(p Player, url string, startTime, duration float64, paused bool, httpHeaders []string, volume int, generation uint64) tea.Cmd {
	return func() tea.Msg {
		startStr := formatStartPosition(startTime, duration)
		logger.Info("launching mpv", "startTime", startTime, "duration", duration, "start", startStr, "generation", generation)
		if err := p.Launch(url, startStr, paused, httpHeaders, volume); err != nil {
			logger.Error("mpv launch failed", "err", err)
			return PlayerLaunchErrMsg{Generation: generation, Err: err}
		}
		return PlayerReadyMsg{Generation: generation}
	}
}

// QuitCmd stops mpv playback.
func QuitCmd(p Player) tea.Cmd {
	return func() tea.Msg {
		_ = p.Quit()
		return PlayerQuitMsg{}
	}
}

// SeekCmd sends an absolute seek request to mpv by target seconds.
func SeekCmd(p Player, target float64) tea.Cmd {
	return func() tea.Msg {
		err := p.Seek(target)
		if err != nil {
			return PositionMsg{Err: err}
		}
		return nil
	}
}

// SeekRelativeCmd sends a relative seek request to mpv by offset seconds.
func SeekRelativeCmd(p Player, offset float64) tea.Cmd {
	return func() tea.Msg {
		err := p.SeekRelative(offset)
		if err != nil {
			return PositionMsg{Err: err}
		}
		return nil
	}
}

// WatchEvents returns a PlayerEndMsg when the current playback ends: at the
// end of the file (eof), on a load or decode error (error), or when the
// player stops (quit).
func (m *Mpv) WatchEvents(generation uint64) tea.Cmd {
	return func() tea.Msg {
		m.mu.RLock()
		play := m.play
		m.mu.RUnlock()
		if play == nil {
			return PlayerEndMsg{Generation: generation, Reason: "error", Err: fmt.Errorf("watch events: %w", ErrClosed)}
		}
		<-play.ended
		return PlayerEndMsg{Generation: generation, Reason: play.reason}
	}
}
