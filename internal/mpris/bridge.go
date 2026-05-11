package mpris

import (
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

// Bind wires the adapter closures to read from accessor and send messages via program.Send().
// seekSeconds is the seek amount for Next/Previous (non-standard MPRIS).
func (b *Bridge) Bind(accessor ModelAccessor, seekSeconds float64) {
	actions := PlayerActions{
		Next: func() error {
			// Non-standard: Next seeks forward instead of skipping tracks.
			// Headphones/speakers have no dedicated seek button, so
			// Next/Previous are mapped to seek forward/backward for audiobook use.
			b.program.Send(SeekMsg{Offset: seekSeconds})
			return nil
		},
		Previous: func() error {
			// Non-standard: Previous seeks backward instead of skipping tracks.
			b.program.Send(SeekMsg{Offset: -seekSeconds})
			return nil
		},
		Pause: func() error {
			b.program.Send(PlayPauseMsg{})
			return nil
		},
		PlayPause: func() error {
			b.program.Send(PlayPauseMsg{})
			return nil
		},
		Stop: func() error {
			b.program.Send(PlayPauseMsg{})
			return nil
		},
		Play: func() error {
			b.program.Send(PlayPauseMsg{})
			return nil
		},
		Seek: func(offset types.Microseconds) error {
			b.program.Send(SeekMsg{Offset: float64(offset) / 1_000_000})
			return nil
		},
		SetPosition: func(trackId string, pos types.Microseconds) error {
			b.program.Send(SeekMsg{Offset: float64(pos) / 1_000_000})
			return nil
		},
		SetRate: func(rate float64) error {
			b.program.Send(SetRateMsg{Rate: rate})
			return nil
		},
		SetVolume: func(vol int) error {
			b.program.Send(SetVolumeMsg{Volume: vol})
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
