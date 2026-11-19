//go:build linux

package systemsleep

import (
	"bufio"
	"errors"
	"io"
	"os"
	"os/exec"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/godbus/dbus/v5"
)

// startSessionBus starts a private D-Bus session bus and returns its address.
// Without dbus-daemon the test skips. With PINE_REQUIRE_DBUS=1 it fails, so
// that a release cannot skip this test.
func startSessionBus(t *testing.T) string {
	t.Helper()
	path, err := exec.LookPath("dbus-daemon")
	if err != nil {
		if os.Getenv("PINE_REQUIRE_DBUS") == "1" {
			t.Fatalf("dbus-daemon is required: %v", err)
		}
		t.Skipf("dbus-daemon is not available: %v", err)
	}
	cmd := exec.Command(path, "--session", "--nofork", "--print-address")
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		t.Fatalf("StdoutPipe() error: %v", err)
	}
	if err := cmd.Start(); err != nil {
		t.Fatalf("start dbus-daemon: %v", err)
	}
	t.Cleanup(func() {
		_ = cmd.Process.Kill()
		_ = cmd.Wait()
	})
	address, err := bufio.NewReader(stdout).ReadString('\n')
	if err != nil {
		t.Fatalf("read the address of dbus-daemon: %v", err)
	}
	return strings.TrimSpace(address)
}

// fakeLogind owns the name of logind on a private bus. Each Inhibit call
// returns the write end of a new pipe. The read end gets EOF when the
// monitor closes its copy of the write end.
type fakeLogind struct {
	conn *dbus.Conn

	mu sync.Mutex
	// block, if it is not nil, makes Inhibit wait until it closes.
	block  chan struct{}
	reads  []*os.File
	writes []*os.File
}

// Inhibit is the logind method. The write end stays open until
// closeWriteEnds, so that it is valid while D-Bus sends it.
func (f *fakeLogind) Inhibit(what, who, why, mode string) (dbus.UnixFD, *dbus.Error) {
	f.mu.Lock()
	block := f.block
	f.mu.Unlock()
	if block != nil {
		<-block
	}
	if what != "sleep" || mode != "delay" {
		return 0, dbus.MakeFailedError(errors.New("unexpected inhibitor " + what + " " + mode))
	}
	r, w, err := os.Pipe()
	if err != nil {
		return 0, dbus.MakeFailedError(err)
	}
	// The fake reads the descriptor under mu, because closeWriteEnds closes
	// the file under mu.
	f.mu.Lock()
	defer f.mu.Unlock()
	f.reads = append(f.reads, r)
	f.writes = append(f.writes, w)
	return dbus.UnixFD(w.Fd()), nil
}

func (f *fakeLogind) calls() int {
	f.mu.Lock()
	defer f.mu.Unlock()
	return len(f.reads)
}

// closeWriteEnds closes the copies of the fake, so that the monitor holds
// the only write end of each pipe.
func (f *fakeLogind) closeWriteEnds() {
	f.mu.Lock()
	defer f.mu.Unlock()
	for _, w := range f.writes {
		_ = w.Close()
	}
	f.writes = nil
}

func (f *fakeLogind) read(i int) *os.File {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.reads[i]
}

func (f *fakeLogind) prepareForSleep(t *testing.T, sleeping bool) {
	t.Helper()
	if err := f.conn.Emit(logindPath, prepareSignal, sleeping); err != nil {
		t.Fatalf("emit PrepareForSleep(%v): %v", sleeping, err)
	}
}

func startFakeLogind(t *testing.T, address string) *fakeLogind {
	t.Helper()
	conn, err := dbus.Connect(address)
	if err != nil {
		t.Fatalf("connect the fake logind: %v", err)
	}
	t.Cleanup(func() { _ = conn.Close() })
	fake := &fakeLogind{conn: conn}
	t.Cleanup(func() {
		fake.closeWriteEnds()
		fake.mu.Lock()
		defer fake.mu.Unlock()
		for _, r := range fake.reads {
			_ = r.Close()
		}
	})
	if err := conn.Export(fake, logindPath, managerIface); err != nil {
		t.Fatalf("export the fake logind: %v", err)
	}
	reply, err := conn.RequestName(logindName, dbus.NameFlagDoNotQueue)
	if err != nil || reply != dbus.RequestNameReplyPrimaryOwner {
		t.Fatalf("own %s: reply %v, error %v", logindName, reply, err)
	}
	return fake
}

func startMonitor(t *testing.T, address string, wait time.Duration, prepare func(release func())) *Monitor {
	t.Helper()
	conn, err := dbus.Connect(address)
	if err != nil {
		t.Fatalf("connect the monitor: %v", err)
	}
	m, err := start(conn, "pinetest", wait, prepare)
	if err != nil {
		_ = conn.Close()
		t.Fatalf("start() error: %v", err)
	}
	t.Cleanup(m.Close)
	return m
}

// assertOpen fails if the inhibitor of the pipe is released.
func assertOpen(t *testing.T, r *os.File) {
	t.Helper()
	if err := r.SetReadDeadline(time.Now().Add(100 * time.Millisecond)); err != nil {
		t.Fatalf("SetReadDeadline() error: %v", err)
	}
	if _, err := r.Read(make([]byte, 1)); !errors.Is(err, os.ErrDeadlineExceeded) {
		t.Fatalf("the inhibitor is released too early: read error %v", err)
	}
}

// assertReleased fails if the inhibitor of the pipe is not released in 2 s.
func assertReleased(t *testing.T, r *os.File) {
	t.Helper()
	if err := r.SetReadDeadline(time.Now().Add(2 * time.Second)); err != nil {
		t.Fatalf("SetReadDeadline() error: %v", err)
	}
	if _, err := r.Read(make([]byte, 1)); !errors.Is(err, io.EOF) {
		t.Fatalf("the inhibitor is not released: read error %v", err)
	}
}

func waitForCalls(t *testing.T, fake *fakeLogind, want int) {
	t.Helper()
	deadline := time.Now().Add(2 * time.Second)
	for fake.calls() < want {
		if time.Now().After(deadline) {
			t.Fatalf("Inhibit calls = %d, want %d", fake.calls(), want)
		}
		time.Sleep(10 * time.Millisecond)
	}
}

// TestReleaseAfterPrepare checks that the monitor holds logind back until
// prepare calls release, and that it takes a new inhibitor after the wake.
func TestReleaseAfterPrepare(t *testing.T) {
	address := startSessionBus(t)
	fake := startFakeLogind(t, address)
	releases := make(chan func(), 1)
	startMonitor(t, address, time.Minute, func(release func()) { releases <- release })
	waitForCalls(t, fake, 1)
	fake.closeWriteEnds()

	fake.prepareForSleep(t, true)
	var release func()
	select {
	case release = <-releases:
	case <-time.After(2 * time.Second):
		t.Fatal("prepare was not called before the sleep")
	}
	assertOpen(t, fake.read(0))
	release()
	assertReleased(t, fake.read(0))

	fake.prepareForSleep(t, false)
	waitForCalls(t, fake, 2)
}

// TestReleaseAfterTimeout checks that the monitor releases the inhibitor
// when prepare does not call release in time.
func TestReleaseAfterTimeout(t *testing.T) {
	address := startSessionBus(t)
	fake := startFakeLogind(t, address)
	called := make(chan struct{}, 1)
	startMonitor(t, address, 50*time.Millisecond, func(func()) { called <- struct{}{} })
	waitForCalls(t, fake, 1)
	fake.closeWriteEnds()

	fake.prepareForSleep(t, true)
	select {
	case <-called:
	case <-time.After(2 * time.Second):
		t.Fatal("prepare was not called before the sleep")
	}
	assertReleased(t, fake.read(0))
}

// TestStartDoesNotWaitForAnInhibitWithNoAnswer verifies that the start of the
// monitor does not wait longer than inhibitTimeout for logind.
func TestStartDoesNotWaitForAnInhibitWithNoAnswer(t *testing.T) {
	address := startSessionBus(t)
	fake := startFakeLogind(t, address)
	block := make(chan struct{})
	fake.mu.Lock()
	fake.block = block
	fake.mu.Unlock()
	t.Cleanup(func() { close(block) })

	conn, err := dbus.Connect(address)
	if err != nil {
		t.Fatalf("connect the monitor: %v", err)
	}
	type result struct {
		m   *Monitor
		err error
	}
	done := make(chan result, 1)
	go func() {
		m, err := start(conn, "pinetest", time.Minute, func(func()) {})
		done <- result{m, err}
	}()
	select {
	case r := <-done:
		if r.err != nil {
			t.Fatalf("start() error: %v", r.err)
		}
		r.m.Close()
	case <-time.After(inhibitTimeout + 2*time.Second):
		_ = conn.Close()
		t.Fatal("start waits for an Inhibit call that gets no answer")
	}
}
