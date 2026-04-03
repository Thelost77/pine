// Package player provides an mpv IPC wrapper for audio playback control.
package player

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sync"

	"github.com/Thelost77/pine/internal/logger"
	"github.com/dexterlb/mpvipc"
)

var (
	mpvSocketDir     string
	mpvSocketDirOnce sync.Once
)

// MpvSocketDir returns the directory where mpv can create IPC sockets.
// For snap-packaged mpv, this is ~/snap/mpv/common/ since snap's /tmp is isolated.
// For native mpv, this is os.TempDir().
func MpvSocketDir() string {
	mpvSocketDirOnce.Do(func() {
		mpvPath, err := exec.LookPath("mpv")
		if err == nil {
			resolved, err := filepath.EvalSymlinks(mpvPath)
			if err == nil && filepath.Base(resolved) == "snap" {
				home, err := os.UserHomeDir()
				if err == nil {
					snapDir := filepath.Join(home, "snap", "mpv", "common")
					if info, err := os.Stat(snapDir); err == nil && info.IsDir() {
						mpvSocketDir = snapDir
						return
					}
				}
			}
		}
		mpvSocketDir = os.TempDir()
	})
	return mpvSocketDir
}

// Player defines the interface for media playback control.
type Player interface {
	Launch(url, startTime, socketPath string) error
	Connect() error
	GetPosition() (float64, error)
	GetDuration() (float64, error)
	GetPaused() (bool, error)
	SetPause(paused bool) error
	Seek(seconds float64) error
	SetSpeed(speed float64) error
	SetVolume(vol int) error
	GetVolume() (int, error)
	Quit() error
}

// IPCConnection abstracts the mpvipc.Connection for testability.
type IPCConnection interface {
	Open() error
	Get(property string) (interface{}, error)
	Set(property string, value interface{}) error
	Call(arguments ...interface{}) (interface{}, error)
	Close() error
	IsClosed() bool
}

// ProcessStarter abstracts process spawning for testability.
type ProcessStarter func(name string, args ...string) *exec.Cmd

// Mpv wraps mpvipc to control an mpv subprocess via IPC.
type Mpv struct {
	conn    IPCConnection
	cmd     *exec.Cmd
	startFn ProcessStarter
	newConn func(socketPath string) IPCConnection
}

// NewMpv creates an Mpv player with default process and connection factories.
func NewMpv() *Mpv {
	return &Mpv{
		startFn: exec.Command,
		newConn: func(socketPath string) IPCConnection {
			return mpvipc.NewConnection(socketPath)
		},
	}
}

// Launch spawns mpv in audio-only mode with the given IPC socket.
// If a previous mpv process is still running, it is killed first.
func (m *Mpv) Launch(url, startTime, socketPath string) error {
	// Clean up any existing mpv process to avoid orphans
	if m.cmd != nil && m.cmd.Process != nil {
		logger.Warn("killing previous mpv process", "pid", m.cmd.Process.Pid)
		_ = m.cmd.Process.Kill()
		_ = m.cmd.Wait()
	}
	if m.conn != nil && !m.conn.IsClosed() {
		_ = m.conn.Close()
	}
	m.cmd = m.startFn("mpv",
		"--no-video",
		fmt.Sprintf("--input-ipc-server=%s", socketPath),
		fmt.Sprintf("--start=%s", startTime),
		url,
	)
	if err := m.cmd.Start(); err != nil {
		logger.Error("failed to start mpv subprocess", "socketPath", socketPath, "startTime", startTime, "err", err)
		return fmt.Errorf("failed to launch mpv: %w", err)
	}
	logger.Info("mpv subprocess started", "pid", m.cmd.Process.Pid, "socketPath", socketPath, "startTime", startTime)
	m.conn = m.newConn(socketPath)
	return nil
}

// Connect opens the IPC connection to mpv.
func (m *Mpv) Connect() error {
	if m.conn == nil {
		return fmt.Errorf("no connection: call Launch first")
	}
	if err := m.conn.Open(); err != nil {
		logger.Debug("failed to open mpv ipc connection", "err", err)
		return err
	}
	logger.Info("mpv ipc connected")
	return nil
}

// GetPosition returns the current playback position in seconds.
func (m *Mpv) GetPosition() (float64, error) {
	return m.getFloat("time-pos")
}

// GetDuration returns the total duration in seconds.
func (m *Mpv) GetDuration() (float64, error) {
	return m.getFloat("duration")
}

// GetPaused returns whether playback is paused.
func (m *Mpv) GetPaused() (bool, error) {
	val, err := m.conn.Get("pause")
	if err != nil {
		logger.Debug("failed to query mpv property", "property", "pause", "err", err)
		return false, fmt.Errorf("get pause: %w", err)
	}
	b, ok := val.(bool)
	if !ok {
		logger.Warn("unexpected mpv property type", "property", "pause", "type", fmt.Sprintf("%T", val))
		return false, fmt.Errorf("unexpected type for pause: %T", val)
	}
	return b, nil
}

// SetPause pauses or resumes playback.
func (m *Mpv) SetPause(paused bool) error {
	if err := m.conn.Set("pause", paused); err != nil {
		logger.Warn("failed to set mpv pause", "paused", paused, "err", err)
		return err
	}
	return nil
}

// Seek seeks to an absolute position in seconds.
func (m *Mpv) Seek(seconds float64) error {
	_, err := m.conn.Call("seek", seconds, "absolute")
	if err != nil {
		logger.Warn("failed to seek mpv", "seconds", seconds, "err", err)
	}
	return err
}

// SetSpeed sets the playback speed multiplier.
func (m *Mpv) SetSpeed(speed float64) error {
	if err := m.conn.Set("speed", speed); err != nil {
		logger.Warn("failed to set mpv speed", "speed", speed, "err", err)
		return err
	}
	return nil
}

// SetVolume sets the playback volume (0-150).
func (m *Mpv) SetVolume(vol int) error {
	if err := m.conn.Set("volume", float64(vol)); err != nil {
		logger.Warn("failed to set mpv volume", "volume", vol, "err", err)
		return err
	}
	return nil
}

// GetVolume returns the current volume level.
func (m *Mpv) GetVolume() (int, error) {
	v, err := m.getFloat("volume")
	if err != nil {
		return 0, err
	}
	return int(v), nil
}

// Quit sends the quit command and cleans up.
func (m *Mpv) Quit() error {
	if m.conn != nil && !m.conn.IsClosed() {
		_, _ = m.conn.Call("quit")
		_ = m.conn.Close()
	}
	if m.cmd != nil && m.cmd.Process != nil {
		logger.Info("stopping mpv subprocess", "pid", m.cmd.Process.Pid)
		_ = m.cmd.Process.Kill()
		_ = m.cmd.Wait()
	}
	return nil
}

func (m *Mpv) getFloat(property string) (float64, error) {
	val, err := m.conn.Get(property)
	if err != nil {
		logger.Debug("failed to query mpv property", "property", property, "err", err)
		return 0, fmt.Errorf("get %s: %w", property, err)
	}
	switch v := val.(type) {
	case float64:
		return v, nil
	case int:
		return float64(v), nil
	case int64:
		return float64(v), nil
	default:
		logger.Warn("unexpected mpv property type", "property", property, "type", fmt.Sprintf("%T", val))
		return 0, fmt.Errorf("unexpected type for %s: %T", property, val)
	}
}
