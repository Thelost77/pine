package mpris

import (
	"fmt"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/quarckster/go-mpris-server/pkg/events"
	"github.com/quarckster/go-mpris-server/pkg/types"

	"github.com/Thelost77/pine/internal/logger"
)

// Bridge connects pine's bubbletea program to the MPRIS D-Bus server.
type Bridge struct {
	program *tea.Program
	server  *Server
	player  PlayerAdapter
}

// NewBridge creates a new MPRIS bridge.
func NewBridge(program *tea.Program) *Bridge {
	return &Bridge{program: program}
}

// send dispatches a message to the bubbletea program without blocking the caller.
// The D-Bus handler goroutine must return quickly; the program's msg channel is unbuffered.
func (b *Bridge) send(msg tea.Msg) {
	go func() {
		b.program.Send(msg)
	}()
}

// Bind wires the adapter closures to read from accessor and send messages via program.Send().
// accessor is a function that returns the current ModelAccessor, called on each read.
// seekSeconds is the seek amount for Next/Previous (non-standard MPRIS).
func (b *Bridge) Bind(accessor func() ModelAccessor, seekSeconds float64) {
	actions := PlayerActions{
		Next: func() error {
			b.send(SeekMsg{Offset: seekSeconds})
			// DE media key daemons (GNOME, KDE, BlueZ) debounce "Next" keypresses by
			// blocking until the track metadata (mpris:trackid) changes. Since we repurpose
			// Next/Previous for seeking, the track doesn't change, causing a 5s timeout delay.
			// Returning an error immediately aborts the DE's debounce wait state.
			return fmt.Errorf("seek completed: returning error to bypass DE track-change debounce")
		},
		Previous: func() error {
			b.send(SeekMsg{Offset: -seekSeconds})
			return fmt.Errorf("seek completed: returning error to bypass DE track-change debounce")
		},
		Pause: func() error {
			b.send(PlayPauseMsg{})
			return nil
		},
		PlayPause: func() error {
			b.send(PlayPauseMsg{})
			return nil
		},
		Stop: func() error {
			b.send(PlayPauseMsg{})
			return nil
		},
		Play: func() error {
			b.send(PlayPauseMsg{})
			return nil
		},
		Seek: func(offset types.Microseconds) error {
			b.send(SeekMsg{Offset: float64(offset) / 1_000_000})
			return nil
		},
		SetPosition: func(trackId string, pos types.Microseconds) error {
			b.send(SeekMsg{Offset: float64(pos) / 1_000_000})
			return nil
		},
		SetRate: func(rate float64) error {
			b.send(SetRateMsg{Rate: rate})
			return nil
		},
		SetVolume: func(vol int) error {
			b.send(SetVolumeMsg{Volume: vol})
			return nil
		},
	}
	root := RootAdapter{}
	b.player = NewPlayerAdapter(accessor, actions)
	b.server = NewServer(root, b.player)
}

// Start runs the D-Bus server in a goroutine.
func (b *Bridge) Start() {
	go func() {
		if err := b.server.Listen(); err != nil {
			logger.Warn("MPRIS server failed to start", "err", err)
		}
	}()
}

// Stop shuts down the D-Bus server.
func (b *Bridge) Stop() error {
	if b.server == nil {
		return nil
	}
	return b.server.Stop()
}

// EventHandler returns the MPRIS event handler for emitting property changes.
func (b *Bridge) EventHandler() *events.EventHandler {
	if b.server == nil {
		return nil
	}
	return b.server.EventHandler()
}
