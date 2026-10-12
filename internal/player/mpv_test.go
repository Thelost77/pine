package player

import (
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"runtime"
	"sync"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"
)

func TestSelectMpvSocketDirUsesSecureFallback(t *testing.T) {
	root := t.TempDir()
	primary := filepath.Join(root, "primary")
	if err := os.WriteFile(primary, nil, 0600); err != nil {
		t.Fatalf("create primary file: %v", err)
	}
	fallback := filepath.Join(root, "fallback")

	got, err := selectMpvSocketDir(primary, fallback)
	if err != nil {
		t.Fatalf("selectMpvSocketDir() error: %v", err)
	}
	if got != fallback {
		t.Fatalf("socket directory = %q, want fallback %q", got, fallback)
	}
	info, err := os.Stat(fallback)
	if err != nil {
		t.Fatalf("stat fallback: %v", err)
	}
	// Windows has no Unix file modes.
	if got := info.Mode().Perm(); runtime.GOOS != "windows" && got != 0700 {
		t.Fatalf("fallback mode = %o, want 700", got)
	}
}

func TestSelectMpvSocketDirReturnsErrorWhenBothPathsInvalid(t *testing.T) {
	root := t.TempDir()
	primary := filepath.Join(root, "primary")
	fallback := filepath.Join(root, "fallback")
	for _, path := range []string{primary, fallback} {
		if err := os.WriteFile(path, nil, 0600); err != nil {
			t.Fatalf("create invalid path: %v", err)
		}
	}

	if _, err := selectMpvSocketDir(primary, fallback); err == nil {
		t.Fatal("expected socket directory preparation error")
	}
}

// setTempDir makes os.TempDir return dir. Unix reads TMPDIR and Windows reads
// TMP.
func setTempDir(t *testing.T, dir string) {
	t.Helper()
	t.Setenv("TMPDIR", dir)
	t.Setenv("TMP", dir)
}

func resetMpvSocketDirForTest(t *testing.T) {
	t.Helper()
	mpvSocketDirMu.Lock()
	mpvSocketDir = ""
	mpvSocketDirMu.Unlock()
	t.Cleanup(func() {
		mpvSocketDirMu.Lock()
		mpvSocketDir = ""
		mpvSocketDirMu.Unlock()
	})
}

func TestMpvSocketDirRetriesAfterPreparationFailure(t *testing.T) {
	resetMpvSocketDirForTest(t)
	root := t.TempDir()
	setTempDir(t, root)
	paths := []string{
		filepath.Join(root, fmt.Sprintf("%s-runtime-%d", appName, os.Getuid())),
		filepath.Join(root, fmt.Sprintf("%s-%d", appName, os.Getuid())),
	}
	for _, path := range paths {
		if err := os.WriteFile(path, nil, 0600); err != nil {
			t.Fatalf("create invalid socket path: %v", err)
		}
	}

	if _, err := MpvSocketDir(); err == nil {
		t.Fatal("expected socket directory preparation error")
	}
	for _, path := range paths {
		if err := os.Remove(path); err != nil {
			t.Fatalf("remove invalid socket path: %v", err)
		}
	}

	dir, err := MpvSocketDir()
	if err != nil {
		t.Fatalf("MpvSocketDir() retry error: %v", err)
	}
	if dir != paths[0] {
		t.Fatalf("socket directory = %q, want primary %q", dir, paths[0])
	}
}

func TestPlayerInterfaceCompliance(t *testing.T) {
	var _ Player = &Mpv{}
}

func TestDescribeStreamURL(t *testing.T) {
	info := describeStreamURL("https://abs.example.com/s/item/book/audio.mp3?api_key=secret-token")

	if !info.Parsed {
		t.Fatal("expected URL to parse")
	}
	if info.Scheme != "https" {
		t.Fatalf("scheme = %q, want https", info.Scheme)
	}
	if info.Host != "abs.example.com" {
		t.Fatalf("host = %q, want abs.example.com", info.Host)
	}
	if info.Path != "/s/item/book/audio.mp3" {
		t.Fatalf("path = %q, want /s/item/book/audio.mp3", info.Path)
	}
	if !info.HasToken {
		t.Fatal("expected api_key query to be detected")
	}
}

func TestDescribeStreamURLInvalid(t *testing.T) {
	info := describeStreamURL("://bad-url")

	if info.Parsed {
		t.Fatal("expected invalid URL to remain unparsed")
	}
	if info.Host != "" || info.Path != "" || info.HasToken {
		t.Fatalf("unexpected info for invalid URL: %+v", info)
	}
}

func TestSetAppNameNamesTheSocketDirectory(t *testing.T) {
	resetMpvSocketDirForTest(t)
	previous := appName
	t.Cleanup(func() { SetAppName(previous) })
	root := t.TempDir()
	setTempDir(t, root)

	SetAppName("pine")

	dir, err := MpvSocketDir()
	if err != nil {
		t.Fatalf("MpvSocketDir() error: %v", err)
	}
	want := filepath.Join(root, fmt.Sprintf("pine-runtime-%d", os.Getuid()))
	if dir != want {
		t.Fatalf("MpvSocketDir() = %q, want %q", dir, want)
	}
}

func TestDescribeStreamURLDetectsTokenQuery(t *testing.T) {
	info := describeStreamURL("https://abs.example.com/s/item/book/audio.mp3?token=secret-token")

	if !info.HasToken {
		t.Fatal("expected token query to be detected")
	}
}

// newTestMpv returns a player with no audio device. The test stops it at its
// end.
func newTestMpv(t *testing.T) *Mpv {
	t.Helper()
	requireLibmpv(t)
	m := NewMpv()
	m.audioOutput = "null"
	t.Cleanup(func() { _ = m.Quit() })
	return m
}

// waitPosition returns the first position that mpv reports.
func waitPosition(t *testing.T, m *Mpv) float64 {
	t.Helper()
	var position float64
	eventually(t, "position after launch", func() (err error) {
		position, err = m.GetPosition()
		return err
	})
	return position
}

// waitPositionNear polls until the position is within half a second of want.
func waitPositionNear(t *testing.T, m *Mpv, want float64) {
	t.Helper()
	eventually(t, fmt.Sprintf("position near %.0f", want), func() error {
		position, err := m.GetPosition()
		if err != nil {
			return err
		}
		if position < want-0.5 || position > want+0.5 {
			return fmt.Errorf("position = %.1f", position)
		}
		return nil
	})
}

// watchEnd returns the PlayerEndMsg of the current playback.
func watchEnd(t *testing.T, m *Mpv, generation uint64) PlayerEndMsg {
	t.Helper()
	result := make(chan tea.Msg, 1)
	go func() { result <- m.WatchEvents(generation)() }()
	select {
	case msg := <-result:
		end, ok := msg.(PlayerEndMsg)
		if !ok {
			t.Fatalf("WatchEvents() = %T, want PlayerEndMsg", msg)
		}
		return end
	case <-time.After(5 * time.Second):
		t.Fatal("WatchEvents() did not return")
		return PlayerEndMsg{}
	}
}

func TestMpvWithoutPlaybackIsClosed(t *testing.T) {
	m := NewMpv()

	if _, err := m.GetPosition(); !errors.Is(err, ErrClosed) {
		t.Errorf("GetPosition() error = %v, want ErrClosed", err)
	}
	if _, err := m.GetPaused(); !errors.Is(err, ErrClosed) {
		t.Errorf("GetPaused() error = %v, want ErrClosed", err)
	}
	if err := m.SetPause(true); !errors.Is(err, ErrClosed) {
		t.Errorf("SetPause() error = %v, want ErrClosed", err)
	}
	if m.Done() != nil {
		t.Error("Done() is not nil without a playback")
	}
	end := watchEnd(t, m, 3)
	if end.Generation != 3 || end.Reason != "error" || !errors.Is(end.Err, ErrClosed) {
		t.Errorf("WatchEvents() = %+v, want generation 3, reason error, ErrClosed", end)
	}
	if err := m.Quit(); err != nil {
		t.Errorf("Quit() without a playback: %v", err)
	}
}

func TestMpvLaunchAppliesHeaderStartPauseAndVolume(t *testing.T) {
	srv := newAudioServer(t)
	m := newTestMpv(t)
	m.ForceSeekable = true

	start := formatStartPosition(60, 120)
	if err := m.Launch(srv.URL+"/long.wav", start, true, []string{testAuthHeader}, 35); err != nil {
		t.Fatalf("Launch() error: %v", err)
	}

	if position := waitPosition(t, m); position < 59 || position > 61 {
		t.Errorf("GetPosition() = %.1f, want 60", position)
	}
	if !srv.authorized.Load() {
		t.Error("the server did not get the Authorization header")
	}
	if duration, err := m.GetDuration(); err != nil || duration < 119 || duration > 121 {
		t.Errorf("GetDuration() = %.1f, %v, want 120", duration, err)
	}
	if paused, err := m.GetPaused(); err != nil || !paused {
		t.Errorf("GetPaused() = %v, %v, want true", paused, err)
	}
	if volume, err := m.GetVolume(); err != nil || volume != 35 {
		t.Errorf("GetVolume() = %d, %v, want 35", volume, err)
	}
	if forced, err := m.handle.getFlag("force-seekable"); err != nil || !forced {
		t.Errorf("force-seekable = %v, %v, want true with ForceSeekable", forced, err)
	}
}

func TestMpvControls(t *testing.T) {
	srv := newAudioServer(t)
	m := newTestMpv(t)
	if err := m.Launch(srv.URL+"/long.wav", "0", true, []string{testAuthHeader}, 100); err != nil {
		t.Fatalf("Launch() error: %v", err)
	}
	waitPosition(t, m)
	if forced, err := m.handle.getFlag("force-seekable"); err != nil || forced {
		t.Errorf("force-seekable = %v, %v, want false without ForceSeekable", forced, err)
	}

	if err := m.Seek(10); err != nil {
		t.Fatalf("Seek() error: %v", err)
	}
	waitPositionNear(t, m, 10)
	if err := m.SeekRelative(5); err != nil {
		t.Fatalf("SeekRelative() error: %v", err)
	}
	waitPositionNear(t, m, 15)

	if err := m.SetVolume(40); err != nil {
		t.Fatalf("SetVolume() error: %v", err)
	}
	if volume, err := m.GetVolume(); err != nil || volume != 40 {
		t.Errorf("GetVolume() = %d, %v, want 40", volume, err)
	}
	if err := m.SetSpeed(1.5); err != nil {
		t.Fatalf("SetSpeed() error: %v", err)
	}
	if speed, err := m.handle.getDouble("speed"); err != nil || speed != 1.5 {
		t.Errorf("speed = %v, %v, want 1.5", speed, err)
	}
	if err := m.SetPause(false); err != nil {
		t.Fatalf("SetPause() error: %v", err)
	}
	if paused, err := m.GetPaused(); err != nil || paused {
		t.Errorf("GetPaused() = %v, %v, want false", paused, err)
	}
}

func TestMpvEndOfFileGivesEOFAndClosesThePlayer(t *testing.T) {
	srv := newAudioServer(t)
	m := newTestMpv(t)
	if err := m.Launch(srv.URL+"/short.wav", "0", false, []string{testAuthHeader}, 100); err != nil {
		t.Fatalf("Launch() error: %v", err)
	}
	done := m.Done()

	end := watchEnd(t, m, 7)
	if end.Generation != 7 || end.Reason != "eof" || end.Err != nil {
		t.Fatalf("WatchEvents() = %+v, want generation 7, reason eof, no error", end)
	}
	select {
	case <-done:
	default:
		t.Error("Done() is open after the end of the file")
	}
	// The apps find the end of a track through the error of the position tick.
	tick, ok := TickCmd(m, 7)().(PositionMsg)
	if !ok || !errors.Is(tick.Err, ErrClosed) {
		t.Errorf("TickCmd() after the end of the file = %+v, want ErrClosed", tick)
	}
}

func TestMpvLoadFailureGivesErrorAndClosesThePlayer(t *testing.T) {
	srv := newAudioServer(t)
	m := newTestMpv(t)
	// No Authorization header: the server answers 401.
	if err := m.Launch(srv.URL+"/long.wav", "0", false, nil, 100); err != nil {
		t.Fatalf("Launch() error: %v", err)
	}

	if end := watchEnd(t, m, 1); end.Reason != "error" {
		t.Fatalf("WatchEvents() = %+v, want reason error", end)
	}
	if _, err := m.GetPosition(); !errors.Is(err, ErrClosed) {
		t.Errorf("GetPosition() error = %v, want ErrClosed", err)
	}
}

func TestMpvReportsPropertyUnavailableWhileTheFileOpens(t *testing.T) {
	// The server accepts the request and then sends nothing, as a slow stream.
	release := make(chan struct{})
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		<-release
	}))
	t.Cleanup(srv.Close)
	t.Cleanup(func() { close(release) })
	m := newTestMpv(t)
	if err := m.Launch(srv.URL+"/slow.wav", "0", false, nil, 100); err != nil {
		t.Fatalf("Launch() error: %v", err)
	}

	// The apps continue to poll on this error. They stop on other errors.
	tick, ok := TickCmd(m, 1)().(PositionMsg)
	if !ok || !errors.Is(tick.Err, ErrPropertyUnavailable) {
		t.Errorf("TickCmd() while the file opens = %+v, want ErrPropertyUnavailable", tick)
	}

	stopped := make(chan struct{})
	go func() {
		_ = m.Quit()
		close(stopped)
	}()
	select {
	case <-stopped:
	case <-time.After(2 * time.Second):
		t.Fatal("Quit() did not return while the file opens")
	}
}

func TestMpvLaunchFailureBeforeInitializeLeavesThePlayerUsable(t *testing.T) {
	srv := newAudioServer(t)
	m := newTestMpv(t)

	// mpv rejects the start value before mpv_initialize.
	if err := m.Launch(srv.URL+"/long.wav", "not-a-time", true, []string{testAuthHeader}, 100); err == nil {
		t.Fatal("Launch() with an invalid start position gave no error")
	}
	if m.Done() != nil {
		t.Error("Done() is not nil after a failed Launch()")
	}
	if _, err := m.GetPosition(); !errors.Is(err, ErrClosed) {
		t.Errorf("GetPosition() after a failed Launch() error = %v, want ErrClosed", err)
	}

	if err := m.Launch(srv.URL+"/long.wav", "0", true, []string{testAuthHeader}, 100); err != nil {
		t.Fatalf("Launch() after a failed Launch() error: %v", err)
	}
	waitPosition(t, m)
}

func TestMpvQuitEndsWatchEventsWithQuit(t *testing.T) {
	srv := newAudioServer(t)
	m := newTestMpv(t)
	if err := m.Launch(srv.URL+"/long.wav", "0", true, []string{testAuthHeader}, 100); err != nil {
		t.Fatalf("Launch() error: %v", err)
	}
	waitPosition(t, m)

	watch := m.WatchEvents(4)
	started := make(chan struct{})
	result := make(chan tea.Msg, 1)
	go func() {
		close(started)
		result <- watch()
	}()
	<-started
	// Let the goroutine read the current playback before Quit removes it.
	time.Sleep(50 * time.Millisecond)
	if err := m.Quit(); err != nil {
		t.Fatalf("Quit() error: %v", err)
	}

	select {
	case msg := <-result:
		end, ok := msg.(PlayerEndMsg)
		if !ok || end.Generation != 4 || end.Reason != "quit" || end.Err != nil {
			t.Fatalf("WatchEvents() after Quit() = %+v, want generation 4, reason quit, no error", msg)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("WatchEvents() did not return after Quit()")
	}
}

func TestMpvLaunchStopsThePreviousPlayback(t *testing.T) {
	srv := newAudioServer(t)
	m := newTestMpv(t)
	if err := m.Launch(srv.URL+"/long.wav", "0", true, []string{testAuthHeader}, 100); err != nil {
		t.Fatalf("first Launch() error: %v", err)
	}
	first := m.Done()

	if err := m.Launch(srv.URL+"/long.wav", "30", true, []string{testAuthHeader}, 100); err != nil {
		t.Fatalf("second Launch() error: %v", err)
	}
	select {
	case <-first:
	default:
		t.Error("the first playback did not end at the second Launch")
	}
	if position := waitPosition(t, m); position < 29 || position > 31 {
		t.Errorf("GetPosition() = %.1f, want 30 from the second Launch", position)
	}
}

func TestMpvQuitWhileAnotherGoroutineReadsThePlayer(t *testing.T) {
	srv := newAudioServer(t)
	m := newTestMpv(t)
	for i := 0; i < 20; i++ {
		if err := m.Launch(srv.URL+"/long.wav", "0", true, []string{testAuthHeader}, 100); err != nil {
			t.Fatalf("Launch() %d error: %v", i, err)
		}
		stop := make(chan struct{})
		var readers sync.WaitGroup
		readers.Add(1)
		go func() {
			defer readers.Done()
			for {
				select {
				case <-stop:
					return
				default:
					_, _ = m.GetPosition()
					_, _ = m.GetPaused()
				}
			}
		}()
		time.Sleep(5 * time.Millisecond)

		if err := m.Quit(); err != nil {
			t.Fatalf("Quit() %d error: %v", i, err)
		}
		close(stop)
		readers.Wait()
		if _, err := m.GetPosition(); !errors.Is(err, ErrClosed) {
			t.Fatalf("GetPosition() after Quit() %d error = %v, want ErrClosed", i, err)
		}
	}
}
