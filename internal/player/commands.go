package player

import (
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/Thelost77/spruce/internal/logger"
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

// PlayerEndMsg is emitted when mpv broadcasts an end-file event, indicating the
// current file finished (eof) or failed to load/decode (error/redirect). The
// app uses the Reason to decide whether to auto-advance the queue.
type PlayerEndMsg struct {
	Generation uint64
	Reason     string // "eof" | "error" | "redirect"
	Err        error
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

// SetPauseCmd sets mpv's paused state.
func SetPauseCmd(p Player, paused bool) tea.Cmd {
	return func() tea.Msg {
		err := p.SetPause(paused)
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

// LaunchCmd spawns mpv and connects via IPC. If paused is true, mpv starts paused.
// httpHeaders are passed to mpv via --http-header-fields.
// Returns PlayerReadyMsg on success.
func LaunchCmd(p Player, url string, startTime float64, paused bool, httpHeaders []string, volume int, generation uint64) tea.Cmd {
	return func() tea.Msg {
		socketDir, err := MpvSocketDir()
		if err != nil {
			logger.Error("failed to prepare mpv socket directory", "err", err)
			return PlayerLaunchErrMsg{Generation: generation, Err: err}
		}
		logger.Info("launching mpv", "startTime", startTime, "socketDir", socketDir)
		socketPath := filepath.Join(socketDir, fmt.Sprintf("spruce-mpv-%d.sock", os.Getpid()))
		// Remove stale socket
		_ = os.Remove(socketPath)

		startStr := fmt.Sprintf("%f", startTime)
		if err := p.Launch(url, startStr, socketPath, paused, httpHeaders, volume); err != nil {
			logger.Error("mpv launch failed", "err", err)
			return PlayerLaunchErrMsg{Generation: generation, Err: err}
		}

		// Retry connection for up to 3 seconds (mpv takes a moment to create the socket)
		var connectErr error
		for i := 0; i < 30; i++ {
			time.Sleep(100 * time.Millisecond)
			if connectErr = p.Connect(); connectErr == nil {
				logger.Info("mpv connected via socket", "socket", socketPath)
				return PlayerReadyMsg{Generation: generation}
			}
		}
		logger.Error("mpv socket connect timeout", "socket", socketPath, "err", connectErr)
		return PlayerLaunchErrMsg{Generation: generation, Err: fmt.Errorf("connect to mpv: %w", connectErr)}
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

// WatchEvents subscribes to mpv's IPC event stream and returns a PlayerEndMsg
// when mpv broadcasts end-file (normal EOF, load/decode error, or redirect).
// mpv auto-broadcasts end-file; no observe command is required.
//
// A side goroutine polls conn.IsClosed so that if mpv is killed without
// emitting end-file (e.g. SIGKILL during stopPlayback), the event listener is
// unregistered and this cmd returns a fatal PlayerEndMsg instead of leaking.
func (m *Mpv) WatchEvents(generation uint64) tea.Cmd {
	return func() tea.Msg {
		conn := m.getConn()
		if conn == nil {
			return PlayerEndMsg{Generation: generation, Reason: "error", Err: fmt.Errorf("no mpv connection")}
		}
		events, stop := conn.NewEventListener()
		if events == nil {
			return PlayerEndMsg{Generation: generation, Reason: "error", Err: fmt.Errorf("event listener unavailable")}
		}
		var closeOnce sync.Once
		closeStop := func() {
			closeOnce.Do(func() {
				close(stop)
			})
		}
		go func() {
			for !conn.IsClosed() {
				time.Sleep(200 * time.Millisecond)
			}
			closeStop()
		}()
		for ev := range events {
			if ev == nil {
				continue
			}
			if ev.Name == "end-file" {
				closeStop()
				return PlayerEndMsg{Generation: generation, Reason: ev.Reason}
			}
		}
		closeStop()
		return PlayerEndMsg{Generation: generation, Reason: "error", Err: fmt.Errorf("mpv connection closed")}
	}
}
