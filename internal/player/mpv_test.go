package player

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/dexterlb/mpvipc"
)

func TestPrepareMpvSocketDirCreatesPrivateDirectory(t *testing.T) {
	path := filepath.Join(t.TempDir(), "socket")

	if err := prepareMpvSocketDir(path); err != nil {
		t.Fatalf("prepareMpvSocketDir() error: %v", err)
	}
	info, err := os.Stat(path)
	if err != nil {
		t.Fatalf("stat socket directory: %v", err)
	}
	if !info.IsDir() {
		t.Fatal("expected socket path to be a directory")
	}
	if info.Mode().Perm()&0077 != 0 {
		t.Fatalf("socket directory mode = %o, want no group or other permissions", info.Mode().Perm())
	}
}

func TestPrepareMpvSocketDirRepairsSameUserPermissions(t *testing.T) {
	path := filepath.Join(t.TempDir(), "socket")
	if err := os.Mkdir(path, 0755); err != nil {
		t.Fatalf("create socket directory: %v", err)
	}

	if err := prepareMpvSocketDir(path); err != nil {
		t.Fatalf("prepareMpvSocketDir() error: %v", err)
	}
	info, err := os.Stat(path)
	if err != nil {
		t.Fatalf("stat socket directory: %v", err)
	}
	if got := info.Mode().Perm(); got != 0700 {
		t.Fatalf("socket directory mode = %o, want 700", got)
	}
}

func TestPrepareMpvSocketDirRejectsFileAndSymlink(t *testing.T) {
	root := t.TempDir()
	file := filepath.Join(root, "file")
	if err := os.WriteFile(file, nil, 0600); err != nil {
		t.Fatalf("create file: %v", err)
	}
	if err := prepareMpvSocketDir(file); err == nil {
		t.Fatal("expected file to be rejected")
	}

	target := filepath.Join(root, "target")
	if err := os.Mkdir(target, 0700); err != nil {
		t.Fatalf("create target directory: %v", err)
	}
	link := filepath.Join(root, "link")
	if err := os.Symlink(target, link); err != nil {
		t.Fatalf("create symlink: %v", err)
	}
	if err := prepareMpvSocketDir(link); err == nil {
		t.Fatal("expected symlink to be rejected")
	}
}

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
	if got := info.Mode().Perm(); got != 0700 {
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

type launchSpy struct {
	launched bool
}

func (p *launchSpy) Launch(string, string, string, bool, []string, int) error {
	p.launched = true
	return nil
}
func (*launchSpy) Connect() error                { return nil }
func (*launchSpy) GetPosition() (float64, error) { return 0, nil }
func (*launchSpy) GetDuration() (float64, error) { return 0, nil }
func (*launchSpy) GetPaused() (bool, error)      { return false, nil }
func (*launchSpy) SetPause(bool) error           { return nil }
func (*launchSpy) Seek(float64) error            { return nil }
func (*launchSpy) SeekRelative(float64) error    { return nil }
func (*launchSpy) SetSpeed(float64) error        { return nil }
func (*launchSpy) SetVolume(int) error           { return nil }
func (*launchSpy) GetVolume() (int, error)       { return 0, nil }
func (*launchSpy) Quit() error                   { return nil }

func TestLaunchCmdDoesNotStartMpvWhenSocketDirectoryFails(t *testing.T) {
	resetMpvSocketDirForTest(t)
	root := t.TempDir()
	t.Setenv("TMPDIR", root)
	t.Setenv("PATH", "")
	for _, name := range []string{
		fmt.Sprintf("%s-runtime-%d", appName, os.Getuid()),
		fmt.Sprintf("%s-%d", appName, os.Getuid()),
	} {
		if err := os.WriteFile(filepath.Join(root, name), nil, 0600); err != nil {
			t.Fatalf("create invalid socket path: %v", err)
		}
	}

	p := &launchSpy{}
	msg := LaunchCmd(p, "https://example.com/audio.mp3", 0, 0, false, nil, 100, 7)()
	if _, ok := msg.(PlayerLaunchErrMsg); !ok {
		t.Fatalf("message = %T, want PlayerLaunchErrMsg", msg)
	}
	if p.launched {
		t.Fatal("mpv launched after socket directory preparation failed")
	}
}

func TestMpvSocketDirRetriesAfterPreparationFailure(t *testing.T) {
	resetMpvSocketDirForTest(t)
	root := t.TempDir()
	t.Setenv("TMPDIR", root)
	t.Setenv("PATH", "")
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

// mockConn implements IPCConnection for testing.
type mockConn struct {
	opened     bool
	closed     bool
	props      map[string]interface{}
	calls      [][]interface{}
	openErr    error
	getErr     error
	setErr     error
	callErr    error
	callResult interface{}
	eventChan  chan *mpvipc.Event
	stopChan   chan struct{}
}

func newMockConn() *mockConn {
	return &mockConn{
		props: map[string]interface{}{
			"time-pos": 42.5,
			"duration": 3600.0,
			"pause":    false,
		},
	}
}

func (m *mockConn) Open() error {
	if m.openErr != nil {
		return m.openErr
	}
	m.opened = true
	return nil
}

func (m *mockConn) Get(property string) (interface{}, error) {
	if m.getErr != nil {
		return nil, m.getErr
	}
	val, ok := m.props[property]
	if !ok {
		return nil, fmt.Errorf("property not found: %s", property)
	}
	return val, nil
}

func (m *mockConn) Set(property string, value interface{}) error {
	if m.setErr != nil {
		return m.setErr
	}
	m.props[property] = value
	return nil
}

func (m *mockConn) Call(arguments ...interface{}) (interface{}, error) {
	m.calls = append(m.calls, arguments)
	if m.callErr != nil {
		return nil, m.callErr
	}
	return m.callResult, nil
}

func (m *mockConn) Close() error {
	m.closed = true
	return nil
}

func (m *mockConn) IsClosed() bool {
	return m.closed
}

func (m *mockConn) NewEventListener() (chan *mpvipc.Event, chan struct{}) {
	if m.eventChan != nil {
		return m.eventChan, m.stopChan
	}
	return nil, nil
}

func newTestMpv(mc *mockConn) *Mpv {
	return &Mpv{
		conn: mc,
		startFn: func(name string, args ...string) *exec.Cmd {
			return exec.Command("echo", "mock")
		},
		newConn: func(socketPath string) IPCConnection {
			return mc
		},
	}
}

func TestPlayerInterfaceCompliance(t *testing.T) {
	var _ Player = &Mpv{}
}

func TestConnect(t *testing.T) {
	mc := newMockConn()
	m := newTestMpv(mc)
	m.cmd = &exec.Cmd{}

	if err := m.Connect(); err != nil {
		t.Fatalf("Connect() error: %v", err)
	}
	if !mc.opened {
		t.Fatal("expected connection to be opened")
	}
}

func TestConnectError(t *testing.T) {
	mc := newMockConn()
	mc.openErr = fmt.Errorf("socket not found")
	m := newTestMpv(mc)
	m.cmd = &exec.Cmd{}

	if err := m.Connect(); err == nil {
		t.Fatal("expected error from Connect()")
	}
}

func TestConnectNilConn(t *testing.T) {
	m := &Mpv{cmd: &exec.Cmd{}}
	if err := m.Connect(); err == nil {
		t.Fatal("expected error when conn is nil")
	}
}

func TestConnectNilCmd(t *testing.T) {
	mc := newMockConn()
	m := newTestMpv(mc)
	m.cmd = nil

	err := m.Connect()
	if err == nil {
		t.Fatal("expected error when cmd is nil")
	}
	if err.Error() != "mpv process is not running (exited early during launch)" {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestConnectProcessExited(t *testing.T) {
	mc := newMockConn()
	m := newTestMpv(mc)
	cmd := exec.Command("true")
	_ = cmd.Run()
	m.cmd = cmd

	if cmd.ProcessState == nil || !cmd.ProcessState.Exited() {
		t.Skip("skipping because command did not exit as expected")
	}

	err := m.Connect()
	if err == nil {
		t.Fatal("expected error when cmd has exited")
	}
	if err.Error() != "mpv process is not running (exited early during launch)" {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestGetPosition(t *testing.T) {
	mc := newMockConn()
	m := newTestMpv(mc)

	pos, err := m.GetPosition()
	if err != nil {
		t.Fatalf("GetPosition() error: %v", err)
	}
	if pos != 42.5 {
		t.Fatalf("expected 42.5, got %f", pos)
	}
}

func TestGetDuration(t *testing.T) {
	mc := newMockConn()
	m := newTestMpv(mc)

	dur, err := m.GetDuration()
	if err != nil {
		t.Fatalf("GetDuration() error: %v", err)
	}
	if dur != 3600.0 {
		t.Fatalf("expected 3600.0, got %f", dur)
	}
}

func TestGetPaused(t *testing.T) {
	mc := newMockConn()
	m := newTestMpv(mc)

	paused, err := m.GetPaused()
	if err != nil {
		t.Fatalf("GetPaused() error: %v", err)
	}
	if paused {
		t.Fatal("expected not paused")
	}
}

func TestGetPausedTrue(t *testing.T) {
	mc := newMockConn()
	mc.props["pause"] = true
	m := newTestMpv(mc)

	paused, err := m.GetPaused()
	if err != nil {
		t.Fatalf("GetPaused() error: %v", err)
	}
	if !paused {
		t.Fatal("expected paused")
	}
}

func TestSetPause(t *testing.T) {
	mc := newMockConn()
	m := newTestMpv(mc)

	if err := m.SetPause(true); err != nil {
		t.Fatalf("SetPause() error: %v", err)
	}
	if mc.props["pause"] != true {
		t.Fatal("expected pause to be true")
	}
}

func TestSeek(t *testing.T) {
	mc := newMockConn()
	m := newTestMpv(mc)

	if err := m.Seek(120.5); err != nil {
		t.Fatalf("Seek() error: %v", err)
	}
	if len(mc.calls) != 1 {
		t.Fatalf("expected 1 call, got %d", len(mc.calls))
	}
	args := mc.calls[0]
	if args[0] != "seek" || args[1] != 120.5 || args[2] != "absolute" {
		t.Fatalf("unexpected seek args: %v", args)
	}
}

func TestSetSpeed(t *testing.T) {
	mc := newMockConn()
	m := newTestMpv(mc)

	if err := m.SetSpeed(1.5); err != nil {
		t.Fatalf("SetSpeed() error: %v", err)
	}
	if mc.props["speed"] != 1.5 {
		t.Fatal("expected speed to be 1.5")
	}
}

func TestQuit(t *testing.T) {
	mc := newMockConn()
	m := newTestMpv(mc)

	if err := m.Quit(); err != nil {
		t.Fatalf("Quit() error: %v", err)
	}
	if !mc.closed {
		t.Fatal("expected connection to be closed")
	}
	if len(mc.calls) != 1 || mc.calls[0][0] != "quit" {
		t.Fatal("expected quit call")
	}
}

func TestQuitNilConn(t *testing.T) {
	m := &Mpv{}
	if err := m.Quit(); err != nil {
		t.Fatalf("Quit() on nil should not error: %v", err)
	}
}

func TestLaunch(t *testing.T) {
	mc := newMockConn()
	launched := false
	m := &Mpv{
		startFn: func(name string, args ...string) *exec.Cmd {
			launched = true
			if name != "mpv" {
				t.Fatalf("expected mpv command, got %s", name)
			}
			expected := []string{
				"--no-video",
				"--input-ipc-server=/tmp/test.sock",
				"--start=30",
				"--volume=20",
				"http://example.com/audio.mp3",
			}
			for i, a := range expected {
				if args[i] != a {
					t.Fatalf("arg %d: expected %s, got %s", i, a, args[i])
				}
			}
			// Use "true" as a no-op command that starts and exits successfully
			return exec.Command("true")
		},
		newConn: func(socketPath string) IPCConnection {
			if socketPath != "/tmp/test.sock" {
				t.Fatalf("expected socket /tmp/test.sock, got %s", socketPath)
			}
			return mc
		},
	}

	err := m.Launch("http://example.com/audio.mp3", "30", "/tmp/test.sock", false, nil, 20)
	if err != nil {
		t.Fatalf("Launch() error: %v", err)
	}
	if !launched {
		t.Fatal("expected startFn to be called")
	}
}

func TestLaunchLoopUsesInfiniteLoopAndPerTrackVolume(t *testing.T) {
	mc := newMockConn()
	var gotArgs []string
	m := &Mpv{
		startFn: func(name string, args ...string) *exec.Cmd {
			gotArgs = args
			return exec.Command("true")
		},
		newConn: func(socketPath string) IPCConnection { return mc },
	}

	if err := m.LaunchLoop("/tmp/track.media", "/tmp/test.sock", true, 20); err != nil {
		t.Fatalf("LaunchLoop() error: %v", err)
	}
	for _, want := range []string{"--loop-file=inf", "--volume=20", "/tmp/track.media", "--pause"} {
		found := false
		for _, arg := range gotArgs {
			if arg == want {
				found = true
				break
			}
		}
		if !found {
			t.Fatalf("LaunchLoop() args %v do not contain %q", gotArgs, want)
		}
	}
}

func TestLaunchPaused(t *testing.T) {
	mc := newMockConn()
	var gotArgs []string
	m := &Mpv{
		startFn: func(name string, args ...string) *exec.Cmd {
			gotArgs = args
			return exec.Command("true")
		},
		newConn: func(socketPath string) IPCConnection { return mc },
	}

	if err := m.Launch("http://example.com/audio.mp3", "0", "/tmp/test.sock", true, nil, 100); err != nil {
		t.Fatalf("Launch() error: %v", err)
	}

	found := false
	for _, a := range gotArgs {
		if a == "--pause" {
			found = true
		}
	}
	if !found {
		t.Fatal("expected --pause flag in mpv args when paused=true")
	}
}

func TestGetPositionError(t *testing.T) {
	mc := newMockConn()
	mc.getErr = fmt.Errorf("connection lost")
	m := newTestMpv(mc)

	_, err := m.GetPosition()
	if err == nil {
		t.Fatal("expected error from GetPosition()")
	}
}

func TestGettersReturnTypedErrors(t *testing.T) {
	tests := []struct {
		name string
		conn func() *mockConn
		want error
	}{
		{
			name: "closed connection",
			conn: func() *mockConn { mc := newMockConn(); mc.closed = true; return mc },
			want: ErrClosed,
		},
		{
			name: "connection closes during the call",
			conn: func() *mockConn {
				mc := newMockConn()
				mc.getErr = fmt.Errorf("trying to send command on closed mpv client")
				return mc
			},
			want: ErrClosed,
		},
		{
			name: "property unavailable",
			conn: func() *mockConn {
				mc := newMockConn()
				mc.getErr = fmt.Errorf("mpv error: property unavailable")
				return mc
			},
			want: ErrPropertyUnavailable,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := newTestMpv(tt.conn())
			if _, err := m.GetPosition(); !errors.Is(err, tt.want) {
				t.Errorf("GetPosition() error = %v, want %v", err, tt.want)
			}
			if _, err := m.GetPaused(); !errors.Is(err, tt.want) {
				t.Errorf("GetPaused() error = %v, want %v", err, tt.want)
			}
		})
	}
}

func TestGetPausedBadType(t *testing.T) {
	mc := newMockConn()
	mc.props["pause"] = "not-a-bool"
	m := newTestMpv(mc)

	_, err := m.GetPaused()
	if err == nil {
		t.Fatal("expected error for bad type")
	}
}

func TestLaunchKillsExistingProcess(t *testing.T) {
	firstConn := newMockConn()
	secondConn := newMockConn()
	launchCount := 0

	m := &Mpv{
		startFn: func(name string, args ...string) *exec.Cmd {
			launchCount++
			return exec.Command("true")
		},
		newConn: func(socketPath string) IPCConnection {
			if launchCount <= 1 {
				return firstConn
			}
			return secondConn
		},
	}

	// First launch
	if err := m.Launch("http://example.com/a.mp3", "0", "/tmp/test.sock", false, nil, 100); err != nil {
		t.Fatalf("first Launch() error: %v", err)
	}
	if launchCount != 1 {
		t.Fatalf("expected 1 launch, got %d", launchCount)
	}

	// Second launch should close the first connection
	if err := m.Launch("http://example.com/b.mp3", "0", "/tmp/test.sock", false, nil, 100); err != nil {
		t.Fatalf("second Launch() error: %v", err)
	}
	if launchCount != 2 {
		t.Fatalf("expected 2 launches, got %d", launchCount)
	}
	if !firstConn.closed {
		t.Error("expected first connection to be closed on re-launch")
	}
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

func TestGetFloatIntType(t *testing.T) {
	mc := newMockConn()
	mc.props["time-pos"] = int(100)
	m := newTestMpv(mc)

	pos, err := m.GetPosition()
	if err != nil {
		t.Fatalf("GetPosition() error: %v", err)
	}
	if pos != 100.0 {
		t.Fatalf("expected 100.0, got %f", pos)
	}
}

func TestWatchEventsEndFileEOF(t *testing.T) {
	mc := newMockConn()
	mc.closed = true // let the watcher's close-poller exit promptly
	events := make(chan *mpvipc.Event, 1)
	stop := make(chan struct{})
	mc.eventChan = events
	mc.stopChan = stop
	events <- &mpvipc.Event{Name: "end-file", Reason: "eof"}

	m := newTestMpv(mc)
	msg := m.WatchEvents(7)()
	end, ok := msg.(PlayerEndMsg)
	if !ok {
		t.Fatalf("expected PlayerEndMsg, got %T", msg)
	}
	if end.Generation != 7 {
		t.Errorf("expected generation 7, got %d", end.Generation)
	}
	if end.Reason != "eof" {
		t.Errorf("expected reason eof, got %q", end.Reason)
	}
}

func TestWatchEventsEndFileError(t *testing.T) {
	mc := newMockConn()
	mc.closed = true
	events := make(chan *mpvipc.Event, 1)
	stop := make(chan struct{})
	mc.eventChan = events
	mc.stopChan = stop
	events <- &mpvipc.Event{Name: "end-file", Reason: "error"}

	m := newTestMpv(mc)
	msg := m.WatchEvents(1)()
	end, ok := msg.(PlayerEndMsg)
	if !ok {
		t.Fatalf("expected PlayerEndMsg, got %T", msg)
	}
	if end.Reason != "error" {
		t.Errorf("expected reason error, got %q", end.Reason)
	}
}

func TestSetAppNameNamesTheSocketDirectory(t *testing.T) {
	resetMpvSocketDirForTest(t)
	previous := appName
	t.Cleanup(func() { SetAppName(previous) })
	root := t.TempDir()
	t.Setenv("TMPDIR", root)
	t.Setenv("PATH", "")

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

func TestLaunchAddsForceSeekableOnlyWhenSet(t *testing.T) {
	for _, forceSeekable := range []bool{false, true} {
		var gotArgs []string
		m := &Mpv{
			ForceSeekable: forceSeekable,
			startFn: func(name string, args ...string) *exec.Cmd {
				gotArgs = args
				return exec.Command("true")
			},
			newConn: func(string) IPCConnection { return newMockConn() },
		}

		if err := m.Launch("http://example.com/audio.mp3", "30", "/tmp/test.sock", false, nil, 100); err != nil {
			t.Fatalf("Launch() error: %v", err)
		}

		got := false
		for _, arg := range gotArgs {
			if arg == "--force-seekable=yes" {
				got = true
			}
		}
		if got != forceSeekable {
			t.Errorf("ForceSeekable=%v: --force-seekable=yes present = %v", forceSeekable, got)
		}
	}
}

func TestDescribeStreamURLDetectsTokenQuery(t *testing.T) {
	info := describeStreamURL("https://abs.example.com/s/item/book/audio.mp3?token=secret-token")

	if !info.HasToken {
		t.Fatal("expected token query to be detected")
	}
}
