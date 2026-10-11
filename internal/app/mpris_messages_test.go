package app

import (
	"testing"

	"github.com/Thelost77/pine/internal/mpris"
)

func newMprisPlaybackTestModel(playing bool) Model {
	m := newPlaybackTestModel()
	m.sessionID = "sess-123"
	m.itemID = "item-1"
	m.player.Playing = playing
	m.player.Position = 100
	m.player.Duration = 3600
	return m
}

func TestMprisPauseMsgKeepsPausedPlaybackPaused(t *testing.T) {
	m := newMprisPlaybackTestModel(false)

	result, cmd := m.Update(mpris.PauseMsg{})

	if result.(Model).player.Playing {
		t.Error("PauseMsg must not resume paused playback")
	}
	if cmd != nil {
		t.Error("expected no command when playback is already paused")
	}
}

func TestMprisPlayMsgKeepsActivePlaybackPlaying(t *testing.T) {
	m := newMprisPlaybackTestModel(true)

	result, cmd := m.Update(mpris.PlayMsg{})

	if !result.(Model).player.Playing {
		t.Error("PlayMsg must not pause active playback")
	}
	if cmd != nil {
		t.Error("expected no command when playback is already active")
	}
}

func TestMprisPauseAndPlayMsgSetPlayback(t *testing.T) {
	m := newMprisPlaybackTestModel(true)

	result, cmd := m.Update(mpris.PauseMsg{})
	m = result.(Model)
	if m.player.Playing {
		t.Error("PauseMsg should pause active playback")
	}
	if cmd == nil {
		t.Error("expected a pause command")
	}

	result, cmd = m.Update(mpris.PlayMsg{})
	m = result.(Model)
	if !m.player.Playing {
		t.Error("PlayMsg should resume paused playback")
	}
	if cmd == nil {
		t.Error("expected a resume command")
	}
}

func TestMprisPlayPauseMsgTogglesPlayback(t *testing.T) {
	m := newMprisPlaybackTestModel(true)

	result, _ := m.Update(mpris.PlayPauseMsg{})

	if result.(Model).player.Playing {
		t.Error("PlayPauseMsg should pause active playback")
	}
}

func TestMprisSeekRelativeMsgSeeksByOffset(t *testing.T) {
	m := newMprisPlaybackTestModel(true)

	result, _ := m.Update(mpris.SeekRelativeMsg{OffsetSeconds: 10})

	if got := result.(Model).player.Position; got != 110 {
		t.Errorf("position = %v; want 110", got)
	}
}

func TestMprisSeekAbsoluteMsgSeeksToPosition(t *testing.T) {
	m := newMprisPlaybackTestModel(true)

	result, _ := m.Update(mpris.SeekAbsoluteMsg{PositionSeconds: 30})

	if got := result.(Model).player.Position; got != 30 {
		t.Errorf("position = %v; want 30", got)
	}
}

func TestMprisMessagesDoNothingWithoutPlayback(t *testing.T) {
	m := newPlaybackTestModel()

	for _, msg := range []any{mpris.PlayMsg{}, mpris.PauseMsg{}, mpris.PlayPauseMsg{}, mpris.SeekRelativeMsg{OffsetSeconds: 10}} {
		result, cmd := m.Update(msg)
		if cmd != nil {
			t.Errorf("%T: expected no command without playback", msg)
		}
		if result.(Model).player.Playing {
			t.Errorf("%T: playback must stay stopped", msg)
		}
	}
}

func TestMprisPauseAndPlayMsgSendTheStateToThePlayer(t *testing.T) {
	m := newMprisPlaybackTestModel(true)
	mp := m.mpv.(*mockPlayer)

	result, cmd := m.Update(mpris.PauseMsg{})
	m, _ = feedCmd(result.(Model), cmd)
	if paused, _ := mp.GetPaused(); !paused {
		t.Error("PauseMsg should pause the player")
	}

	result, cmd = m.Update(mpris.PlayMsg{})
	feedCmd(result.(Model), cmd)
	if paused, _ := mp.GetPaused(); paused {
		t.Error("PlayMsg should resume the player")
	}
}
