package player

import (
	"fmt"
	"os/exec"
	"testing"
)

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

	if err := m.Connect(); err == nil {
		t.Fatal("expected error from Connect()")
	}
}

func TestConnectNilConn(t *testing.T) {
	m := &Mpv{}
	if err := m.Connect(); err == nil {
		t.Fatal("expected error when conn is nil")
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

	err := m.Launch("http://example.com/audio.mp3", "30", "/tmp/test.sock", false)
	if err != nil {
		t.Fatalf("Launch() error: %v", err)
	}
	if !launched {
		t.Fatal("expected startFn to be called")
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

	if err := m.Launch("http://example.com/audio.mp3", "0", "/tmp/test.sock", true); err != nil {
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
	if err := m.Launch("http://example.com/a.mp3", "0", "/tmp/test.sock", false); err != nil {
		t.Fatalf("first Launch() error: %v", err)
	}
	if launchCount != 1 {
		t.Fatalf("expected 1 launch, got %d", launchCount)
	}

	// Second launch should close the first connection
	if err := m.Launch("http://example.com/b.mp3", "0", "/tmp/test.sock", false); err != nil {
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
	info := describeStreamURL("https://abs.example.com/s/item/book/audio.mp3?token=secret-token")

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
		t.Fatal("expected token query to be detected")
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
