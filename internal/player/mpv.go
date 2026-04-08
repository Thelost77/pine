// Package player provides an mpv IPC wrapper for audio playback control.
package player

import (
	"bufio"
	"fmt"
	"io"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"sync"
	"syscall"

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
	Launch(url, startTime, socketPath string, paused bool) error
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
	conn     IPCConnection
	cmd      *exec.Cmd
	startFn  ProcessStarter
	newConn  func(socketPath string) IPCConnection
	procMu   sync.Mutex
	waitDone chan struct{}
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
// If paused is true, mpv starts paused and the user must press play to resume.
// If a previous mpv process is still running, it is killed first.
func (m *Mpv) Launch(url, startTime, socketPath string, paused bool) error {
	// Clean up any existing mpv process to avoid orphans
	if m.cmd != nil && m.cmd.Process != nil {
		m.stopProcess("killing previous mpv process")
	}
	if m.conn != nil && !m.conn.IsClosed() {
		_ = m.conn.Close()
	}

	args := []string{
		"--no-video",
		fmt.Sprintf("--input-ipc-server=%s", socketPath),
		fmt.Sprintf("--start=%s", startTime),
		url,
	}
	if paused {
		args = append(args, "--pause")
	}
	cmd := m.startFn("mpv", args...)
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		logger.Error("failed to attach mpv stdout", "socketPath", socketPath, "startTime", startTime, "err", err)
		return fmt.Errorf("attach mpv stdout: %w", err)
	}
	stderr, err := cmd.StderrPipe()
	if err != nil {
		logger.Error("failed to attach mpv stderr", "socketPath", socketPath, "startTime", startTime, "err", err)
		return fmt.Errorf("attach mpv stderr: %w", err)
	}
	if err := cmd.Start(); err != nil {
		logger.Error("failed to start mpv subprocess", "socketPath", socketPath, "startTime", startTime, "err", err)
		return fmt.Errorf("failed to launch mpv: %w", err)
	}
	stream := describeStreamURL(url)
	logger.Info("mpv subprocess started",
		"pid", cmd.Process.Pid,
		"socketPath", socketPath,
		"startTime", startTime,
		"streamParsed", stream.Parsed,
		"streamScheme", stream.Scheme,
		"streamHost", stream.Host,
		"streamPath", stream.Path,
		"streamHasToken", stream.HasToken,
	)
	m.startProcessWatchers(cmd, stdout, stderr)
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
		m.stopProcess("stopping mpv subprocess")
	}
	return nil
}

type streamLogInfo struct {
	Parsed   bool
	Scheme   string
	Host     string
	Path     string
	HasToken bool
}

func describeStreamURL(rawURL string) streamLogInfo {
	parsed, err := url.Parse(rawURL)
	if err != nil {
		return streamLogInfo{}
	}
	return streamLogInfo{
		Parsed:   true,
		Scheme:   parsed.Scheme,
		Host:     parsed.Host,
		Path:     parsed.EscapedPath(),
		HasToken: parsed.Query().Has("token"),
	}
}

func (m *Mpv) startProcessWatchers(cmd *exec.Cmd, stdout, stderr io.ReadCloser) {
	done := make(chan struct{})

	m.procMu.Lock()
	m.cmd = cmd
	m.waitDone = done
	m.procMu.Unlock()

	pid := cmd.Process.Pid

	go logMpvPipe("stdout", pid, stdout)
	go logMpvPipe("stderr", pid, stderr)

	go func() {
		err := cmd.Wait()
		logMpvExit(pid, cmd.ProcessState, err)

		m.procMu.Lock()
		if m.cmd == cmd {
			m.cmd = nil
			m.waitDone = nil
		}
		m.procMu.Unlock()

		close(done)
	}()
}

func (m *Mpv) stopProcess(reason string) {
	m.procMu.Lock()
	cmd := m.cmd
	done := m.waitDone
	m.procMu.Unlock()

	if cmd == nil || cmd.Process == nil {
		return
	}

	logger.Info(reason, "pid", cmd.Process.Pid)
	if err := cmd.Process.Kill(); err != nil {
		logger.Debug("failed to kill mpv subprocess", "pid", cmd.Process.Pid, "err", err)
	}
	if done != nil {
		<-done
	}

	m.procMu.Lock()
	if m.cmd == cmd {
		m.cmd = nil
		m.waitDone = nil
	}
	m.procMu.Unlock()
}

func logMpvPipe(stream string, pid int, r io.ReadCloser) {
	defer r.Close()

	scanner := bufio.NewScanner(r)
	for scanner.Scan() {
		logger.Debug("mpv "+stream, "pid", pid, "line", scanner.Text())
	}
	if err := scanner.Err(); err != nil {
		logger.Warn("failed reading mpv "+stream, "pid", pid, "err", err)
	}
}

func logMpvExit(pid int, state *os.ProcessState, err error) {
	args := []any{"pid", pid}
	if state != nil {
		args = append(args, "exitCode", state.ExitCode(), "state", state.String())
		if status, ok := state.Sys().(syscall.WaitStatus); ok && status.Signaled() {
			args = append(args, "signal", status.Signal())
		}
	}
	if err != nil {
		args = append(args, "err", err)
		logger.Warn("mpv subprocess exited", args...)
		return
	}
	logger.Info("mpv subprocess exited", args...)
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
