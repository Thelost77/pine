// Package player plays audio through libmpv and provides the player model.
package player

import (
	"errors"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"sync"

	"github.com/Thelost77/pine/internal/logger"
)

var (
	// appName names the runtime files of this process: the runtime directory
	// and the media cache.
	appName        = "player"
	mpvSocketDir   string
	mpvSocketDirMu sync.Mutex
)

// SetAppName sets the app name used for runtime file names.
// Call it once at startup, before playback starts.
func SetAppName(name string) {
	mpvSocketDirMu.Lock()
	defer mpvSocketDirMu.Unlock()
	appName = name
	mpvSocketDir = ""
}

// runtimeDir returns the private runtime directory of the app:
// /tmp/<app name>-runtime-<uid> with 0700 permissions. The media cache is in
// this directory.
func runtimeDir() (string, error) {
	mpvSocketDirMu.Lock()
	defer mpvSocketDirMu.Unlock()
	if mpvSocketDir != "" {
		return mpvSocketDir, nil
	}
	dir, err := resolveMpvSocketDir()
	if err != nil {
		return "", err
	}
	mpvSocketDir = dir
	return dir, nil
}

func resolveMpvSocketDir() (string, error) {
	primary := filepath.Join(os.TempDir(), fmt.Sprintf("%s-runtime-%d", appName, os.Getuid()))
	fallback := filepath.Join(os.TempDir(), fmt.Sprintf("%s-%d", appName, os.Getuid()))
	return selectMpvSocketDir(primary, fallback)
}

func selectMpvSocketDir(primary, fallback string) (string, error) {
	if err := prepareMpvSocketDir(primary); err == nil {
		return primary, nil
	} else if fallbackErr := prepareMpvSocketDir(fallback); fallbackErr == nil {
		return fallback, nil
	} else {
		return "", fmt.Errorf("prepare mpv socket directories: primary: %w; fallback: %v", err, fallbackErr)
	}
}

// ErrClosed reports that the player is gone: no file plays, or playback ended.
var ErrClosed = errors.New("player closed")

// ErrPropertyUnavailable reports that mpv has no value for a property yet, for
// example while it opens a file or seeks.
var ErrPropertyUnavailable = errors.New("property unavailable")

// Player defines the interface for media playback control.
type Player interface {
	Launch(url, startTime string, paused bool, httpHeaders []string, volume int) error
	GetPosition() (float64, error)
	GetDuration() (float64, error)
	GetPaused() (bool, error)
	GetIdle() (bool, error)
	SetPause(paused bool) error
	Seek(seconds float64) error
	SetSpeed(speed float64) error
	SetVolume(vol int) error
	Quit() error
}

// Mpv plays one file at a time through a libmpv handle in this process.
type Mpv struct {
	// opMu lets only one Launch or Quit run at a time.
	opMu sync.Mutex
	// mu guards handle and play. Each libmpv call holds it for reading, so
	// that Quit cannot destroy a handle that is in use.
	mu     sync.RWMutex
	handle mpvHandle
	play   *playback

	// ForceSeekable sets force-seekable=yes at launch, so that mpv can seek
	// in an HTTP stream that it does not find seekable.
	ForceSeekable bool

	// audioOutput sets the mpv option ao. The tests use "null".
	audioOutput string
}

// playback is the event state of one handle.
type playback struct {
	// ended closes on the first end-file event.
	ended chan struct{}
	// reason is the end-file reason. Read it only after ended closes.
	reason string
	// stopped closes when the event goroutine exits.
	stopped chan struct{}
}

// NewMpv creates a player. libmpv loads on the first Launch.
func NewMpv() *Mpv {
	return &Mpv{}
}

// Done returns a channel that closes when the current playback ends.
func (m *Mpv) Done() <-chan struct{} {
	if m == nil {
		return nil
	}
	m.mu.RLock()
	defer m.mu.RUnlock()
	if m.play == nil {
		return nil
	}
	return m.play.ended
}

// Launch starts audio-only playback of url.
// If paused is true, mpv starts paused and the user must press play to resume.
// A playback that is still active is stopped first.
func (m *Mpv) Launch(url, startTime string, paused bool, httpHeaders []string, volume int) error {
	return m.launch(url, startTime, paused, httpHeaders, volume, false)
}

// LaunchLoop starts playback of a local media file that loops indefinitely.
func (m *Mpv) LaunchLoop(path string, paused bool, volume int) error {
	return m.launch(path, "0", paused, nil, volume, true)
}

func (m *Mpv) launch(url, startTime string, paused bool, httpHeaders []string, volume int, loop bool) error {
	m.opMu.Lock()
	defer m.opMu.Unlock()
	m.stop()

	if err := loadLibmpv(); err != nil {
		return err
	}
	h := mpvHandle(mpvCreate())
	if h == 0 {
		logger.Error("failed to create mpv handle", "startTime", startTime)
		return errors.New("failed to launch mpv: no handle")
	}
	if err := m.start(h, url, startTime, paused, httpHeaders, volume, loop); err != nil {
		mpvTerminateDestroy(uintptr(h))
		logger.Error("failed to start mpv", "startTime", startTime, "err", err)
		return fmt.Errorf("failed to launch mpv: %w", err)
	}

	play := &playback{ended: make(chan struct{}), stopped: make(chan struct{})}
	go play.watch(h)
	m.mu.Lock()
	m.handle, m.play = h, play
	m.mu.Unlock()

	stream := describeStreamURL(url)
	logger.Info("mpv started",
		"startTime", startTime,
		"streamParsed", stream.Parsed,
		"streamScheme", stream.Scheme,
		"streamHost", stream.Host,
		"streamPath", stream.Path,
		"streamHasToken", stream.HasToken,
	)
	return nil
}

// start sets the options, initializes the handle, and loads the file.
func (m *Mpv) start(h mpvHandle, url, startTime string, paused bool, httpHeaders []string, volume int, loop bool) error {
	options := [][2]string{
		{"vid", "no"},
		// By default mpv starts yt-dlp for a URL that fails to load.
		{"ytdl", "no"},
		{"start", startTime},
		{"volume", strconv.Itoa(volume)},
	}
	if paused {
		options = append(options, [2]string{"pause", "yes"})
	}
	if m.ForceSeekable {
		options = append(options, [2]string{"force-seekable", "yes"})
	}
	if loop {
		options = append(options, [2]string{"loop-file", "inf"})
	}
	if m.audioOutput != "" {
		options = append(options, [2]string{"ao", m.audioOutput})
	}
	if runtime.GOOS == "darwin" {
		// On macOS 27 with mpv 0.41, CoreAudio plays mono audio at double
		// speed. With a stereo layout it plays at the correct speed.
		options = append(options, [2]string{"audio-channels", "stereo"})
	}
	// The options go in before mpv_initialize and not into loadfile: the
	// position of the loadfile options argument changed in mpv 0.38.
	for _, option := range options {
		if err := h.setOption(option[0], option[1]); err != nil {
			return err
		}
	}
	if err := mpvError(mpvInitialize(uintptr(h)), "initialize"); err != nil {
		return err
	}
	// mpv sends its warnings and errors as events, and watch writes them to
	// the app log. They are the only detail on a stream that fails to load.
	if err := mpvError(mpvRequestLogMessages(uintptr(h), mpvLogLevel), "request log messages"); err != nil {
		return err
	}
	// A header value can contain a comma, so each header is one list item.
	for _, header := range httpHeaders {
		if err := h.command("change-list", "http-header-fields", "append", header); err != nil {
			return err
		}
	}
	return h.command("loadfile", url)
}

// watch reads the events of the handle until mpv shuts down. No other
// goroutine may call mpv_wait_event on the handle.
func (p *playback) watch(h mpvHandle) {
	defer close(p.stopped)
	ended := false
	end := func(reason string) {
		if ended {
			return
		}
		ended = true
		p.reason = reason
		close(p.ended)
		logger.Info("mpv playback ended", "reason", reason)
	}
	for {
		// The event is valid only until the next mpv_wait_event call.
		switch event := mpvWaitEvent(uintptr(h), -1); event.id {
		case mpvEventLogMessage:
			message := (*mpvLogMessage)(event.data)
			logger.Warn("mpv message",
				"level", cString(message.level),
				"prefix", cString(message.prefix),
				"text", message.logText(),
			)
		case mpvEventEndFile:
			file := (*mpvEndFile)(event.data)
			reason := endFileReason(file.reason)
			if reason == "error" {
				logger.Warn("mpv failed to play the file", "err", mpvErrorString(file.err))
			}
			end(reason)
		case mpvEventShutdown:
			end("quit")
			return
		}
	}
}

// stop ends the current playback and destroys its handle. The caller holds
// opMu.
func (m *Mpv) stop() {
	m.mu.Lock()
	h, play := m.handle, m.play
	m.handle, m.play = 0, nil
	m.mu.Unlock()
	if h == 0 {
		return
	}

	// mpv_terminate_destroy must not run while the event goroutine is in
	// mpv_wait_event. The quit command makes mpv send the shutdown event, and
	// the goroutine exits on that event.
	if err := h.command("quit"); err != nil {
		logger.Error("failed to stop mpv, the handle stays open", "err", err)
		return
	}
	<-play.stopped
	mpvTerminateDestroy(uintptr(h))
}

// active returns the handle while a file plays. The caller holds mu.
func (m *Mpv) active() (mpvHandle, error) {
	if m.handle == 0 {
		return 0, ErrClosed
	}
	select {
	case <-m.play.ended:
		// libmpv stays idle after the end of the file. For the apps the
		// player is closed, as it was when the mpv process exited.
		return 0, ErrClosed
	default:
		return m.handle, nil
	}
}

// withHandle runs request on the handle of the current playback.
func (m *Mpv) withHandle(what string, request func(mpvHandle) error) error {
	m.mu.RLock()
	defer m.mu.RUnlock()
	h, err := m.active()
	if err != nil {
		return fmt.Errorf("%s: %w", what, err)
	}
	return request(h)
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
	return m.getFlag("pause")
}

// GetIdle returns whether no audio plays now. It is true for a paused player,
// and for a player that is not paused but waits for data, starts, or seeks.
func (m *Mpv) GetIdle() (bool, error) {
	return m.getFlag("core-idle")
}

// SetPause pauses or resumes playback.
func (m *Mpv) SetPause(paused bool) error {
	err := m.withHandle("set pause", func(h mpvHandle) error {
		return h.setFlag("pause", paused)
	})
	if err != nil {
		logger.Warn("failed to set mpv pause", "paused", paused, "err", err)
	}
	return err
}

// Seek seeks to an absolute position in seconds.
func (m *Mpv) Seek(seconds float64) error {
	err := m.withHandle("seek", func(h mpvHandle) error {
		return h.command("seek", strconv.FormatFloat(seconds, 'f', -1, 64), "absolute")
	})
	if err != nil {
		logger.Warn("failed to seek mpv", "seconds", seconds, "err", err)
	}
	return err
}

// SetSpeed sets the playback speed multiplier.
func (m *Mpv) SetSpeed(speed float64) error {
	err := m.withHandle("set speed", func(h mpvHandle) error {
		return h.setDouble("speed", speed)
	})
	if err != nil {
		logger.Warn("failed to set mpv speed", "speed", speed, "err", err)
	}
	return err
}

// SetVolume sets the playback volume (0-150).
func (m *Mpv) SetVolume(vol int) error {
	err := m.withHandle("set volume", func(h mpvHandle) error {
		return h.setDouble("volume", float64(vol))
	})
	if err != nil {
		logger.Warn("failed to set mpv volume", "volume", vol, "err", err)
	}
	return err
}

// Quit stops playback and destroys the mpv handle.
func (m *Mpv) Quit() error {
	m.opMu.Lock()
	defer m.opMu.Unlock()
	m.stop()
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
		HasToken: parsed.Query().Has("token") || parsed.Query().Has("api_key") || parsed.Query().Has("ApiKey"),
	}
}

func (m *Mpv) getFlag(property string) (bool, error) {
	var value bool
	err := m.withHandle("get "+property, func(h mpvHandle) (err error) {
		value, err = h.getFlag(property)
		return err
	})
	if err != nil {
		logger.Debug("failed to query mpv property", "property", property, "err", err)
	}
	return value, err
}

func (m *Mpv) getFloat(property string) (float64, error) {
	var value float64
	err := m.withHandle("get "+property, func(h mpvHandle) (err error) {
		value, err = h.getDouble(property)
		return err
	})
	if err != nil {
		logger.Debug("failed to query mpv property", "property", property, "err", err)
	}
	return value, err
}
