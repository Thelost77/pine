package player

import (
	"fmt"
	"os"
	"time"

	"github.com/Thelost77/pine/internal/logger"
	tea "github.com/charmbracelet/bubbletea"
)

// PositionMsg carries the current playback position polled from mpv.
type PositionMsg struct {
	Position   float64
	Duration   float64
	Paused     bool
	Err        error
	Generation uint64 // ties this tick to a specific play session
}

// TickCmd returns a command that fires twice per second and polls mpv state.
// The generation parameter ties the tick to a specific play session so stale
// ticks from a previous session can be safely ignored.
func TickCmd(p Player, generation uint64) tea.Cmd {
	return tea.Tick(500*time.Millisecond, func(_ time.Time) tea.Msg {
		pos, posErr := p.GetPosition()
		dur, durErr := p.GetDuration()
		paused, pauseErr := p.GetPaused()

		// Return first error encountered
		for _, err := range []error{posErr, durErr, pauseErr} {
			if err != nil {
				return PositionMsg{Err: err, Generation: generation}
			}
		}

		return PositionMsg{
			Position:   pos,
			Duration:   dur,
			Paused:     paused,
			Generation: generation,
		}
	})
}

// TogglePauseCmd sends a pause toggle to mpv.
func TogglePauseCmd(p Player, shouldPlay bool) tea.Cmd {
	return func() tea.Msg {
		err := p.SetPause(!shouldPlay)
		if err != nil {
			return PositionMsg{Err: err}
		}
		return nil
	}
}

// SetSpeedCmd sends a speed change to mpv.
func SetSpeedCmd(p Player, speed float64) tea.Cmd {
	return func() tea.Msg {
		err := p.SetSpeed(speed)
		if err != nil {
			return PositionMsg{Err: err}
		}
		return nil
	}
}

// SetVolumeCmd sends a volume change to mpv.
func SetVolumeCmd(p Player, vol int) tea.Cmd {
	return func() tea.Msg {
		err := p.SetVolume(vol)
		if err != nil {
			return PositionMsg{Err: err}
		}
		return nil
	}
}

// PlayerReadyMsg signals that mpv has been launched and connected.
type PlayerReadyMsg struct{}

// PlayerLaunchErrMsg signals that mpv failed to launch.
type PlayerLaunchErrMsg struct {
	Err error
}

// PlayerQuitMsg signals that mpv has been quit.
type PlayerQuitMsg struct{}

// LaunchCmd spawns mpv and connects via IPC. If paused is true, mpv starts paused.
// httpHeaders are passed to mpv via --http-header-fields.
// Returns PlayerReadyMsg on success.
func LaunchCmd(p Player, url string, startTime float64, paused bool, httpHeaders []string) tea.Cmd {
	return func() tea.Msg {
		logger.Info("launching mpv", "startTime", startTime, "socketDir", MpvSocketDir())
		socketPath := fmt.Sprintf("%s/pine-mpv-%d.sock", MpvSocketDir(), os.Getpid())
		// Remove stale socket
		os.Remove(socketPath)

		startStr := fmt.Sprintf("%f", startTime)
		if err := p.Launch(url, startStr, socketPath, paused, httpHeaders); err != nil {
			logger.Error("mpv launch failed", "err", err)
			return PlayerLaunchErrMsg{Err: err}
		}

		// Retry connection for up to 3 seconds (mpv takes a moment to create the socket)
		var connectErr error
		for i := 0; i < 30; i++ {
			time.Sleep(100 * time.Millisecond)
			if connectErr = p.Connect(); connectErr == nil {
				logger.Info("mpv connected via socket", "socket", socketPath)
				return PlayerReadyMsg{}
			}
		}
		logger.Error("mpv socket connect timeout", "socket", socketPath, "err", connectErr)
		return PlayerLaunchErrMsg{Err: fmt.Errorf("connect to mpv: %w", connectErr)}
	}
}

// QuitCmd stops mpv playback.
func QuitCmd(p Player) tea.Cmd {
	return func() tea.Msg {
		_ = p.Quit()
		return PlayerQuitMsg{}
	}
}
