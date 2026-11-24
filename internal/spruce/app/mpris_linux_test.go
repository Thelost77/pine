//go:build linux

package app

import (
	"bufio"
	"context"
	"os"
	"os/exec"
	"strings"
	"testing"
	"time"

	"github.com/Thelost77/pine/internal/mpris"
	"github.com/Thelost77/pine/internal/spruce/jellyfin"
	"github.com/Thelost77/pine/internal/spruce/screens/library"
	"github.com/Thelost77/pine/internal/ui/components"
	tea "github.com/charmbracelet/bubbletea"
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

	bridge := mpris.NewBridge(nil, "spruce")
	state := m.mprisState
	bridge.Bind(func() mpris.ModelAccessor { return state }, 0)
	m.mprisBridge = bridge
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

// TestMprisSeekedOnlyForSameTrackStart verifies that a start of the track that
// plays sends Seeked(0), because MPRIS clients see the same track ID. A start
// of a different track sends no Seeked signal.
func TestMprisSeekedOnlyForSameTrackStart(t *testing.T) {
	playFromLibrary := func(id string) func(m *Model) (tea.Model, tea.Cmd) {
		return func(m *Model) (tea.Model, tea.Cmd) {
			return m.Update(library.PlayTracksMsg{Tracks: []jellyfin.Track{{ID: id}}})
		}
	}
	playFromPalette := func(id string) func(m *Model) (tea.Model, tea.Cmd) {
		return func(m *Model) (tea.Model, tea.Cmd) {
			return m.handlePaletteAction(components.ActionPlayDirect, id, jellyfin.Track{ID: id})
		}
	}
	for _, tc := range []struct {
		name       string
		queue      []string
		current    int
		preparing  bool
		start      func(m *Model) (tea.Model, tea.Cmd)
		wantSeeked bool
	}{
		{"library other track", []string{"X", "A"}, 0, false, playFromLibrary("Y"), false},
		{"library same track", []string{"A", "X"}, 1, false, playFromLibrary("X"), true},
		{"palette same track", []string{"A", "X"}, 1, false, playFromPalette("X"), true},
		{"library same track while subliminal prepares", []string{"A", "X"}, 1, true, playFromLibrary("X"), true},
		{"palette same track while subliminal prepares", []string{"A", "X"}, 1, true, playFromPalette("X"), true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			m := New(nil, nil)
			for _, id := range tc.queue {
				m.tracks = append(m.tracks, jellyfin.Track{ID: id})
			}
			m.currentIndex = tc.current
			m.playerState.Title = tc.queue[tc.current]
			m.playerState.Playing = true
			m.playerState.Position = 120
			if tc.preparing {
				m.subliminalState = subliminalPreparing
				m.subliminalContext, m.subliminalCancel = context.WithCancel(context.Background())
			}
			seeked := listenMpris(t, m)
			m.syncQueueScreen()

			_, _ = tc.start(m)
			// The signals keep their order. The first signal after the start
			// shows whether the start sent Seeked.
			const marker = 42
			m.mprisSeeked(marker)
			want := int64(marker * 1_000_000)
			if tc.wantSeeked {
				want = 0
			}
			select {
			case got := <-seeked:
				if got != want {
					t.Fatalf("first Seeked=%d, want %d", got, want)
				}
			case <-time.After(5 * time.Second):
				t.Fatal("no Seeked signal")
			}
		})
	}
}
