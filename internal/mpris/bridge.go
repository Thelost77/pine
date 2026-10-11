package mpris

import (
	"errors"
	"fmt"
	"sync"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/quarckster/go-mpris-server/pkg/events"
	"github.com/quarckster/go-mpris-server/pkg/types"

	"github.com/Thelost77/pine/internal/logger"
)

// Bridge connects the app's bubbletea program to the MPRIS D-Bus server.
type Bridge struct {
	program   *tea.Program
	name      string
	server    *Server
	player    PlayerAdapter
	msgCh     chan tea.Msg
	quitCh    chan struct{}
	doneCh    chan struct{}
	mu        sync.Mutex
	started   bool
	startOnce sync.Once
	stopOnce  sync.Once
}

// NewBridge creates a new MPRIS bridge. name is the app name that MPRIS clients see.
func NewBridge(program *tea.Program, name string) *Bridge {
	return &Bridge{
		program: program,
		name:    name,
		msgCh:   make(chan tea.Msg, 64),
		quitCh:  make(chan struct{}),
		doneCh:  make(chan struct{}),
	}
}

var errSkipSeek = errors.New("seek completed: returning error to bypass DE track-change debounce")

// send dispatches a message to the bubbletea program via the buffered dispatcher without blocking the caller.
func (b *Bridge) send(msg tea.Msg) {
	select {
	case <-b.quitCh:
		return
	case b.msgCh <- msg:
	default:
		logger.Warn("MPRIS signal dropped: message buffer full", "msgType", fmt.Sprintf("%T", msg))
	}
}

// Bind wires the adapter closures to read from accessor and send messages via program.Send().
// accessor is a function that returns the current ModelAccessor, called on each read.
// If skipSeekSeconds is more than 0, Next and Previous seek by that amount (non-standard MPRIS)
// and do not change the track.
func (b *Bridge) Bind(accessor func() ModelAccessor, skipSeekSeconds float64) {
	actions := PlayerActions{
		Next: func() error {
			if skipSeekSeconds > 0 {
				b.send(SeekRelativeMsg{OffsetSeconds: skipSeekSeconds})
				// DE media key daemons (GNOME, KDE, BlueZ) debounce "Next" keypresses by
				// blocking until the track metadata (mpris:trackid) changes. Since we repurpose
				// Next/Previous for seeking, the track doesn't change, causing a 5s timeout delay.
				// Returning an error immediately aborts the DE's debounce wait state.
				return errSkipSeek
			}
			b.send(NextMsg{})
			return nil
		},
		Previous: func() error {
			if skipSeekSeconds > 0 {
				b.send(SeekRelativeMsg{OffsetSeconds: -skipSeekSeconds})
				return errSkipSeek
			}
			b.send(PreviousMsg{})
			return nil
		},
		Pause: func() error {
			b.send(PauseMsg{})
			return nil
		},
		PlayPause: func() error {
			b.send(PlayPauseMsg{})
			return nil
		},
		Stop: func() error {
			b.send(PauseMsg{})
			return nil
		},
		Play: func() error {
			b.send(PlayMsg{})
			return nil
		},
		Seek: func(offset types.Microseconds) error {
			b.send(SeekRelativeMsg{OffsetSeconds: float64(offset) / 1_000_000})
			return nil
		},
		SetPosition: func(trackId string, pos types.Microseconds) error {
			// MPRIS: SetPosition has no effect if CanSeek is false, and a request
			// for a track that is not the current track is stale.
			a := accessor()
			if !a.CanSeek() || trackId != string(trackPath(b.name, a.CurrentItemID())) {
				return nil
			}
			b.send(SeekAbsoluteMsg{PositionSeconds: float64(pos) / 1_000_000})
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
	root := RootAdapter{Name: b.name}
	b.player = NewPlayerAdapter(b.name, accessor, actions)
	b.server = NewServer(b.name, root, b.player)
}

// Start runs the D-Bus server in a goroutine and starts the message dispatcher loop.
func (b *Bridge) Start() {
	b.startOnce.Do(func() {
		b.mu.Lock()
		b.started = true
		b.mu.Unlock()
		go func() {
			if err := b.server.Listen(); err != nil {
				logger.Warn("MPRIS server failed to start", "err", err)
			}
		}()
		go b.dispatchLoop()
	})
}

func (b *Bridge) dispatchLoop() {
	defer close(b.doneCh)
	for {
		select {
		case <-b.quitCh:
			return
		case msg, ok := <-b.msgCh:
			if !ok {
				return
			}
			if b.program != nil {
				b.program.Send(msg)
			}
		}
	}
}

// Stop shuts down the D-Bus server and terminates the dispatcher loop.
func (b *Bridge) Stop() error {
	var err error
	b.stopOnce.Do(func() {
		close(b.quitCh)
		b.mu.Lock()
		started := b.started
		b.mu.Unlock()
		if started {
			<-b.doneCh
		}
		if b.server != nil {
			err = b.server.Stop()
		}
	})
	return err
}

// EventHandler returns the MPRIS event handler for emitting property changes.
func (b *Bridge) EventHandler() *events.EventHandler {
	if b.server == nil {
		return nil
	}
	return b.server.EventHandler()
}
