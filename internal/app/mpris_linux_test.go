//go:build linux

package app

import (
	"bufio"
	"os"
	"os/exec"
	"strings"
	"testing"
	"time"

	"github.com/Thelost77/pine/internal/mpris"
	"github.com/Thelost77/pine/internal/player"
	"github.com/godbus/dbus/v5"
)

// listenMpris starts the MPRIS server of m on a private D-Bus session bus. It
// returns the positions of the Seeked signals of the server, in microseconds.
// Without dbus-daemon the test skips. With PINE_REQUIRE_DBUS=1 it fails.
func listenMpris(t *testing.T, m *Model) <-chan int64 {
	t.Helper()
	path, err := exec.LookPath("dbus-daemon")
	if err != nil {
		if os.Getenv("PINE_REQUIRE_DBUS") == "1" {
			t.Fatalf("dbus-daemon is required: %v", err)
		}
		t.Skipf("dbus-daemon is not available: %v", err)
	}
	daemon := exec.Command(path, "--session", "--nofork", "--print-address")
	stdout, err := daemon.StdoutPipe()
	if err != nil {
		t.Fatalf("StdoutPipe() error: %v", err)
	}
	if err := daemon.Start(); err != nil {
		t.Fatalf("start dbus-daemon: %v", err)
	}
	t.Cleanup(func() {
		_ = daemon.Process.Kill()
		_ = daemon.Wait()
	})
	address, err := bufio.NewReader(stdout).ReadString('\n')
	if err != nil {
		t.Fatalf("read the address of dbus-daemon: %v", err)
	}
	address = strings.TrimSpace(address)
	// The server must not connect to the session bus of the desktop.
	t.Setenv("DBUS_SESSION_BUS_ADDRESS", address)

	bridge := mpris.NewBridge(nil, "pine")
	state := m.mprisState
	bridge.Bind(func() mpris.ModelAccessor { return state }, 0)
	m.mprisLink.bridge = bridge
	server := bridge.Server()
	go func() { _ = server.Listen() }()
	t.Cleanup(func() { _ = server.Stop() })

	client, err := dbus.Connect(address)
	if err != nil {
		t.Fatalf("connect to the test bus: %v", err)
	}
	t.Cleanup(func() { _ = client.Close() })
	for _, member := range []string{"Seeked", "PropertiesChanged"} {
		if err := client.AddMatchSignal(dbus.WithMatchMember(member)); err != nil {
			t.Fatalf("AddMatchSignal(%s) error: %v", member, err)
		}
	}
	signals := make(chan *dbus.Signal, 64)
	client.Signal(signals)

	// The server sends a signal only after it has its connection. Send a
	// change of the volume until a change arrives.
	deadline := time.Now().Add(5 * time.Second)
	for ready := false; !ready; {
		if time.Now().After(deadline) {
			t.Fatal("the MPRIS server sends no signal")
		}
		server.VolumeChanged()
		select {
		case sig := <-signals:
			ready = sig.Name == "org.freedesktop.DBus.Properties.PropertiesChanged"
		case <-time.After(50 * time.Millisecond):
		}
	}

	seeked := make(chan int64, 64)
	go func() {
		for sig := range signals {
			if sig.Name == "org.mpris.MediaPlayer2.Player.Seeked" && len(sig.Body) == 1 {
				if pos, ok := sig.Body[0].(int64); ok {
					seeked <- pos
				}
			}
		}
	}()
	return seeked
}

// nextSeeked returns the position of the next Seeked signal, in microseconds.
func nextSeeked(t *testing.T, seeked <-chan int64) int64 {
	t.Helper()
	select {
	case pos := <-seeked:
		return pos
	case <-time.After(5 * time.Second):
		t.Fatal("no Seeked signal")
		return 0
	}
}

// TestMprisSeekedOnlyForSeek verifies that the position ticks of a playback
// send no Seeked signal, and that a seek sends Seeked with the new position.
// An MPRIS client computes the position of a playback from the rate.
func TestMprisSeekedOnlyForSeek(t *testing.T) {
	m := newMprisPlaybackTestModel(true)
	m.trackDuration = 3600
	seeked := listenMpris(t, &m)

	for pos := 101.0; pos <= 103; pos++ {
		result, cmd := m.Update(player.PositionMsg{Position: pos, Duration: 3600, Generation: m.playGeneration})
		m = result.(Model)
		executeBatchCmds(cmd)
	}
	result, cmd := m.Update(mpris.SeekRelativeMsg{OffsetSeconds: 30})
	m = result.(Model)
	executeBatchCmds(cmd)

	if got := nextSeeked(t, seeked); got != 133_000_000 {
		t.Errorf("first Seeked position = %d µs, want 133000000 µs (the seek)", got)
	}
}

// TestMprisSeekedForStartAtPosition verifies that a playback that starts at a
// position after the start of the book sends Seeked with that position.
func TestMprisSeekedForStartAtPosition(t *testing.T) {
	m := newPlaybackTestModel()
	seeked := listenMpris(t, &m)

	result, cmd := m.Update(PlaySessionMsg{Session: PlaySessionData{
		SessionID:     "sess-123",
		ItemID:        "item-1",
		CurrentTime:   300,
		Duration:      3600,
		Title:         "A Book",
		TrackDuration: 3600,
	}})
	m = result.(Model)
	executeBatchCmds(cmd)

	if got := nextSeeked(t, seeked); got != 300_000_000 {
		t.Errorf("Seeked position = %d µs, want 300000000 µs", got)
	}
}

// TestMprisSeekedForRestartAtStart verifies that a restart of the item at the
// start of the book sends Seeked, for example a seek from a later file of the
// book to the first chapter. The track ID does not change.
func TestMprisSeekedForRestartAtStart(t *testing.T) {
	m := newMprisPlaybackTestModel(true)
	seeked := listenMpris(t, &m)
	// The item plays the second file, at the book position 2000.
	m.trackStartOffset = 1800
	m.trackDuration = 1800
	m.player.Position = 2000
	m.syncMprisState()

	m, _ = m.restartPlaybackAt(0)
	result, cmd := m.Update(PlaySessionMsg{Session: PlaySessionData{
		SessionID:     "sess-restart",
		ItemID:        "item-1",
		Duration:      3600,
		TrackDuration: 1800,
	}})
	m = result.(Model)
	executeBatchCmds(cmd)

	if got := nextSeeked(t, seeked); got != 0 {
		t.Errorf("Seeked position = %d µs, want 0 µs", got)
	}
}
