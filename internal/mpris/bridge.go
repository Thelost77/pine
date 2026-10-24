package mpris

import (
	"errors"
	"fmt"
	"math"
	"sync"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/Thelost77/pine/internal/logger"
)

// Bridge connects the app's bubbletea program to the MPRIS D-Bus server.
type Bridge struct {
	program   *tea.Program
	name      string
	server    *Server
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

// maxVolume is the MPRIS volume of the app volume 150, the end of the scale
// of SetVolumeMsg.
const maxVolume = 1.5

// skipSeekAccessor reports Next and Previous as possible when an item is
// active and a seek is possible, because with a skip seek they seek.
type skipSeekAccessor struct{ ModelAccessor }

func (a skipSeekAccessor) CanGoNext() bool     { return a.HasActiveItem() && a.CanSeek() }
func (a skipSeekAccessor) CanGoPrevious() bool { return a.HasActiveItem() && a.CanSeek() }

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

// Bind creates the MPRIS server. Its properties read from accessor, and its methods send messages via program.Send().
// accessor is a function that returns the current ModelAccessor, called on each read.
// If skipSeekSeconds is more than 0, Next and Previous seek by that amount (non-standard MPRIS)
// and do not change the track.
func (b *Bridge) Bind(accessor func() ModelAccessor, skipSeekSeconds float64) {
	if skipSeekSeconds > 0 {
		appAccessor := accessor
		accessor = func() ModelAccessor { return skipSeekAccessor{appAccessor()} }
	}
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
		Seek: func(offset time.Duration) error {
			b.send(SeekRelativeMsg{OffsetSeconds: offset.Seconds()})
			return nil
		},
		SetPosition: func(trackId string, pos time.Duration) error {
			// MPRIS: SetPosition has no effect if CanSeek is false or if the
			// position is outside the track. A request for a track that is not
			// the current track is stale.
			a := accessor()
			if !a.CanSeek() || trackId != string(trackPath(b.name, a.CurrentItemID())) {
				return nil
			}
			seconds := pos.Seconds()
			if seconds < 0 || (a.PlayerDuration() > 0 && seconds > a.PlayerDuration()) {
				return nil
			}
			b.send(SeekAbsoluteMsg{PositionSeconds: seconds})
			return nil
		},
		SetRate: func(rate float64) error {
			// MPRIS: a rate of 0 acts as Pause. A rate is in the range of
			// MinimumRate and MaximumRate.
			if rate == 0 {
				b.send(PauseMsg{})
				return nil
			}
			a := accessor()
			b.send(SetRateMsg{Rate: min(max(rate, a.PlayerMinimumRate()), a.PlayerMaximumRate())})
			return nil
		},
		SetVolume: func(volume float64) error {
			// MPRIS: a negative volume is 0.
			b.send(SetVolumeMsg{Volume: int(math.Round(min(max(volume, 0), maxVolume) * 100))})
			return nil
		},
	}
	b.server = NewServer(b.name, RootAdapter{Name: b.name}, accessor, actions)
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
		case msg := <-b.msgCh:
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

// Server returns the MPRIS server. It is nil before Bind.
func (b *Bridge) Server() *Server {
	return b.server
}
