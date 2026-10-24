package app

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"time"

	"github.com/Thelost77/pine/internal/logger"
	"github.com/Thelost77/pine/internal/player"
	tea "github.com/charmbracelet/bubbletea"
)

func (m *Model) startPlaybackAt(index int) (*Model, tea.Cmd) {
	if index < 0 || index >= len(m.tracks) {
		return m.stopPlayback()
	}

	m.currentIndex = index
	m.pendingRepeatIndex = -1
	m.pendingRepeatItemID = ""
	m.playGeneration++
	track := m.tracks[index]
	m.playerState.Title = track.Name
	m.playerState.Position = 0
	m.playerState.Duration = track.Duration()
	m.playerState.Playing = true
	m.playerState.SubliminalCount = 0
	m.lastHeartbeat = time.Now()

	logger.Info("starting track playback", "index", index, "id", track.ID, "title", track.Name)

	m.syncQueueScreen()

	var cmds []tea.Cmd
	if m.client != nil {
		playSessionID := strconv.FormatInt(time.Now().UnixNano(), 16)
		m.playSessionID = playSessionID
		source := m.client.StreamURL(track.ID, playSessionID)
		streamHeaders := m.client.StreamHeaders()
		if m.mediaCache != nil {
			if path, ok := m.mediaCache.Path(track.ID); ok {
				source = path
				streamHeaders = nil
			}
		}
		client := m.client
		itemID := track.ID

		startReqCmd := func() tea.Msg {
			_ = client.ReportPlaybackStart(context.Background(), itemID, playSessionID)
			return nil
		}
		cmds = append(cmds, startReqCmd)

		if m.mpv != nil {
			cmds = append(cmds, player.LaunchCmd(m.mpv, source, 0, false, streamHeaders, m.playerState.Volume, m.playGeneration))
		}
	}
	if m.repeatQueue {
		cmds = append(cmds, m.prefetchRepeatQueue(index))
	} else if m.repeatTrackID == track.ID {
		cmds = append(cmds, m.prefetchRepeatTrack(index))
	}

	return m, tea.Batch(cmds...)
}

func (m *Model) stopPlayback() (*Model, tea.Cmd) {
	logger.Info("stopping playback")
	position := m.playerState.Position
	m.playGeneration++
	m.playerState.Title = ""
	m.playerState.Playing = false
	m.playerState.Position = 0
	m.pendingRepeatIndex = -1
	m.pendingRepeatItemID = ""
	m.sleepDeadline = time.Time{}
	m.sleepDuration = 0
	m.playerState.SleepRemaining = ""

	var cmds []tea.Cmd
	if m.client != nil && m.currentIndex >= 0 && m.currentIndex < len(m.tracks) {
		client := m.client
		itemID := m.tracks[m.currentIndex].ID
		playSessionID := m.playSessionID
		stopReqCmd := func() tea.Msg {
			_ = client.ReportPlaybackStopped(context.Background(), itemID, position, playSessionID)
			return nil
		}
		cmds = append(cmds, stopReqCmd)
	}

	if m.mpv != nil {
		cmds = append(cmds, player.QuitCmd(m.mpv))
	}

	m.currentIndex = -1
	m.syncQueueScreen()
	return m, tea.Batch(cmds...)
}

func (m *Model) setPlaybackPaused(paused bool) (*Model, tea.Cmd) {
	if !m.IsPlaying() {
		return m, nil
	}

	m.playerState.Playing = !paused
	m.syncQueueScreen()
	if m.mpv != nil {
		return m, player.SetPauseCmd(m.mpv, paused)
	}
	return m, nil
}

func (m *Model) handlePositionMsg(msg player.PositionMsg) (*Model, tea.Cmd) {
	if msg.Generation != m.playGeneration {
		return m, nil
	}

	if msg.Err != nil {
		if isMpvPropertyUnavailable(msg.Err) {
			logger.Warn("player position poll temporarily unavailable", "err", msg.Err)
			if m.mpv != nil {
				return m, player.TickCmd(m.mpv, m.playGeneration)
			}
			return m, nil
		}
		if errors.Is(msg.Err, player.ErrClosed) {
			// The playback ended. The player reports this from the instant of
			// the end-file event, so this tick can arrive before PlayerEndMsg.
			// PlayerEndMsg has the reason and selects the next step.
			return m, nil
		}
		logger.Error("player position poll failed (fatal)", "err", msg.Err)
		// Do NOT auto-advance — this is a fatal player error, not EOF.
		// EOF is delivered via PlayerEndMsg{Reason:"eof"}.
		return m.stopPlayback()
	}

	m.playerState.Position = msg.Position
	if msg.Duration > 0 {
		m.playerState.Duration = msg.Duration
	}
	m.playerState.Playing = !msg.Paused

	if !m.sleepDeadline.IsZero() {
		remaining := time.Until(m.sleepDeadline)
		if remaining <= 0 {
			m.playerState.SleepRemaining = ""
		} else {
			m.playerState.SleepRemaining = formatSleepRemaining(remaining)
		}
	}

	m.syncQueueScreen()

	var cmds []tea.Cmd
	if m.mpv != nil {
		cmds = append(cmds, player.TickCmd(m.mpv, m.playGeneration))
	}

	if m.client != nil && m.currentIndex >= 0 && m.currentIndex < len(m.tracks) && time.Since(m.lastHeartbeat) >= 15*time.Second {
		m.lastHeartbeat = time.Now()
		client := m.client
		itemID := m.tracks[m.currentIndex].ID
		pos := m.playerState.Position
		paused := msg.Paused
		playSessionID := m.playSessionID
		heartbeatCmd := func() tea.Msg {
			_ = client.ReportPlaybackProgress(context.Background(), itemID, pos, paused, playSessionID)
			return nil
		}
		cmds = append(cmds, heartbeatCmd)
	}

	return m, tea.Batch(cmds...)
}

func isMpvPropertyUnavailable(err error) bool {
	return errors.Is(err, player.ErrPropertyUnavailable)
}

func authorsEqual(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

type mprisStateSnapshot struct {
	playing  bool
	paused   bool
	title    string
	authors  []string
	itemID   string
	position float64
	speed    float64
	volume   int
}

func (m *Model) syncQueueScreen() {
	if !m.queueScreen.IsFiltering() && !m.queueScreen.HasActiveFilter() {
		m.queueScreen.SetQueue(m.tracks, m.currentIndex)
	}
	m.queueScreen.SetPlaybackState(
		m.IsPlaying(),
		m.IsPaused(),
		m.playerState.Position,
		m.playerState.Duration,
	)
	status, complete, total := m.subliminalStatus()
	modeVolume := subliminalVolume
	if m.subliminalState == subliminalStarting || m.subliminalState == subliminalPlaying {
		modeVolume = m.playerState.Volume
	}
	m.queueScreen.SetSubliminalState(status, complete, total, modeVolume)
	if m.mprisState != nil {
		var oldState, newState mprisStateSnapshot
		m.mprisState.Update(func(s *MprisState) {
			oldState = mprisStateSnapshot{
				playing:  s.playing,
				paused:   s.paused,
				title:    s.title,
				authors:  s.authors,
				itemID:   s.itemID,
				position: s.position,
				speed:    s.speed,
				volume:   s.volume,
			}
			normalActive := m.IsPlaying()
			subliminalActive := m.isSubliminalRunning()
			active := normalActive || subliminalActive
			s.playing = active && m.playerState.Playing
			s.paused = active && !m.playerState.Playing
			s.hasActiveItem = active
			s.canSeek = normalActive
			s.canGoNext = normalActive && len(m.tracks) > 0
			s.canGoPrevious = normalActive
			if subliminalActive {
				s.title = m.playerState.Title
				s.authors = nil
				s.itemID = "subliminal_mix"
				s.duration = 0
			} else if normalActive && m.currentIndex >= 0 && m.currentIndex < len(m.tracks) {
				s.title = m.tracks[m.currentIndex].Name
				s.authors = m.tracks[m.currentIndex].Artists
				s.itemID = m.tracks[m.currentIndex].ID
				if m.playerState.Duration <= 0 {
					s.duration = m.tracks[m.currentIndex].Duration()
				} else {
					s.duration = m.playerState.Duration
				}
			} else {
				s.title = ""
				s.authors = nil
				s.itemID = ""
				s.duration = 0
			}
			s.position = m.playerState.Position
			if subliminalActive {
				s.speed = 1.0
				s.minimumRate = 1.0
				s.maximumRate = 1.0
			} else {
				s.speed = m.playerState.Speed
				s.minimumRate = 0.5
				s.maximumRate = 4.0
			}
			s.volume = m.playerState.Volume
			s.queueLength = len(m.tracks)
			newState = mprisStateSnapshot{
				playing:  s.playing,
				paused:   s.paused,
				title:    s.title,
				authors:  s.authors,
				itemID:   s.itemID,
				position: s.position,
				speed:    s.speed,
				volume:   s.volume,
			}
		})

		if m.mprisBridge != nil {
			server := m.mprisBridge.Server()
			if server != nil {
				playbackChanged := oldState.playing != newState.playing ||
					oldState.paused != newState.paused ||
					oldState.speed != newState.speed
				metadataChanged := oldState.title != newState.title ||
					oldState.itemID != newState.itemID ||
					!authorsEqual(oldState.authors, newState.authors)
				volumeChanged := oldState.volume != newState.volume
				positionChanged := oldState.position != newState.position

				if playbackChanged {
					server.PlaybackChanged()
				}
				if metadataChanged {
					server.MetadataChanged()
				}
				if volumeChanged {
					server.VolumeChanged()
				}
				if positionChanged {
					now := time.Now()
					if now.Sub(m.lastMprisEmit) >= time.Second {
						m.lastMprisEmit = now
						server.Seeked(newState.position)
					}
				}
			}
		}
	}
	m.propagateSize()
}

func (m *Model) handleSeek(offset float64) (*Model, tea.Cmd) {
	if m.mpv == nil || !m.IsPlaying() {
		return m, nil
	}
	target := m.playerState.Position + offset
	if target < 0 {
		target = 0
	}
	if m.playerState.Duration > 0 && target > m.playerState.Duration {
		target = m.playerState.Duration
	}
	m.playerState.Position = target
	m.syncQueueScreen()
	return m, player.SeekCmd(m.mpv, target)
}

func (m *Model) handleSeekAbsolute(pos float64) (*Model, tea.Cmd) {
	if m.mpv == nil || !m.IsPlaying() {
		return m, nil
	}
	if pos < 0 {
		pos = 0
	}
	if m.playerState.Duration > 0 && pos > m.playerState.Duration {
		pos = m.playerState.Duration
	}
	m.playerState.Position = pos
	m.syncQueueScreen()
	return m, player.SeekCmd(m.mpv, pos)
}

func (m *Model) mprisPlaybackCmd() tea.Cmd {
	if m.mprisBridge == nil {
		return nil
	}
	server := m.mprisBridge.Server()
	if server == nil {
		return nil
	}
	return func() tea.Msg {
		server.PlaybackChanged()
		return nil
	}
}

func (m *Model) mprisPlayPauseCmd() tea.Cmd {
	if m.mprisBridge == nil {
		return nil
	}
	server := m.mprisBridge.Server()
	if server == nil {
		return nil
	}
	return func() tea.Msg {
		server.StatusChanged()
		return nil
	}
}

func (m *Model) mprisEndedCmd() tea.Cmd {
	if m.mprisBridge == nil {
		return nil
	}
	server := m.mprisBridge.Server()
	if server == nil {
		return nil
	}
	return func() tea.Msg {
		server.StatusChanged()
		return nil
	}
}

func (m *Model) mprisTitleCmd() tea.Cmd {
	if m.mprisBridge == nil {
		return nil
	}
	server := m.mprisBridge.Server()
	if server == nil {
		return nil
	}
	return func() tea.Msg {
		server.MetadataChanged()
		return nil
	}
}

func (m *Model) mprisPositionCmd() tea.Cmd {
	if m.mprisBridge == nil {
		return nil
	}
	now := time.Now()
	if now.Sub(m.lastMprisEmit) < time.Second {
		return nil
	}
	m.lastMprisEmit = now
	server := m.mprisBridge.Server()
	if server == nil {
		return nil
	}
	pos := m.playerState.Position
	return func() tea.Msg {
		server.Seeked(pos)
		return nil
	}
}

func (m *Model) mprisVolumeCmd() tea.Cmd {
	if m.mprisBridge == nil {
		return nil
	}
	server := m.mprisBridge.Server()
	if server == nil {
		return nil
	}
	return func() tea.Msg {
		server.VolumeChanged()
		return nil
	}
}

func (m *Model) handlePlayerEvent(msg tea.Msg) (*Model, tea.Cmd, bool) {
	switch msg := msg.(type) {
	case SleepTimerExpiredMsg:
		if (m.IsPlaying() || m.isSubliminalRunning()) && !m.sleepDeadline.IsZero() && msg.Generation == m.sleepGeneration {
			logger.Info("sleep timer expired, stopping playback")
			m.sleepDeadline = time.Time{}
			m.sleepDuration = 0
			m.playerState.SleepRemaining = ""
			if m.isSubliminalRunning() {
				newM, cmd := m.stopSubliminalMode()
				return newM, cmd, true
			}
			newM, cmd := m.stopPlayback()
			return newM, cmd, true
		}
		return m, nil, true

	case player.PositionMsg:
		newM, cmd := m.handlePositionMsg(msg)
		return newM, cmd, true

	case player.CommandErrMsg:
		// A player that is closed has no state. The next launch sets the
		// speed and the volume.
		if errors.Is(msg.Err, player.ErrClosed) {
			return m, nil, true
		}
		return m, m.err.SetError(msg.Err), true

	case player.PlayerReadyMsg:
		if msg.Generation != m.playGeneration {
			if m.mpv != nil && (m.isSubliminalActive() || !m.IsPlaying()) {
				return m, player.QuitCmd(m.mpv), true
			}
			return m, nil, true
		}
		var cmds []tea.Cmd
		if m.mpv != nil {
			cmds = append(cmds, player.TickCmd(m.mpv, m.playGeneration))
			cmds = append(cmds, m.mpv.WatchEvents(m.playGeneration))
			cmds = append(cmds, player.SetVolumeCmd(m.mpv, m.playerState.Volume))
			cmds = append(cmds, player.SetSpeedCmd(m.mpv, m.playerState.Speed))
		}
		return m, tea.Batch(cmds...), true

	case player.PlayerEndMsg:
		if msg.Generation != m.playGeneration {
			return m, nil, true
		}
		if msg.Reason == "eof" {
			if m.repeatTrackID != "" {
				for idx, t := range m.tracks {
					if t.ID == m.repeatTrackID {
						newM, cmd := m.startRepeatedPlaybackAt(idx)
						return newM, cmd, true
					}
				}
			}
			nextIdx := m.currentIndex + 1
			if m.repeatQueue && len(m.tracks) > 0 {
				if nextIdx >= len(m.tracks) {
					nextIdx = 0
				}
				newM, cmd := m.startRepeatedPlaybackAt(nextIdx)
				return newM, cmd, true
			}
			if nextIdx < len(m.tracks) {
				newM, cmd := m.startPlaybackAt(nextIdx)
				return newM, cmd, true
			}
			newM, cmd := m.stopPlayback()
			return newM, cmd, true
		}
		logger.Error("player ended with non-eof reason", "reason", msg.Reason, "err", msg.Err)
		newM, cmd := m.stopPlayback()
		if msg.Err != nil {
			errCmd := newM.err.SetError(msg.Err)
			return newM, tea.Batch(cmd, errCmd), true
		}
		return newM, cmd, true

	case player.PlayerLaunchErrMsg:
		if msg.Generation != m.playGeneration {
			if m.mpv != nil && (m.isSubliminalActive() || !m.IsPlaying()) {
				return m, player.QuitCmd(m.mpv), true
			}
			return m, nil, true
		}
		logger.Error("player failed to launch", "err", msg.Err)
		newM, cmd := m.stopPlayback()
		errCmd := newM.err.SetError(msg.Err)
		return newM, tea.Batch(cmd, errCmd), true
	}
	return m, nil, false
}

var sleepDurations = []time.Duration{
	0,
	15 * time.Minute,
	30 * time.Minute,
	45 * time.Minute,
	60 * time.Minute,
}

func (m *Model) cycleSleepTimer() (*Model, tea.Cmd) {
	nextIdx := 0
	for i, d := range sleepDurations {
		if d == m.sleepDuration {
			nextIdx = (i + 1) % len(sleepDurations)
			break
		}
	}
	return m.setSleepTimer(sleepDurations[nextIdx])
}

func (m *Model) setSleepTimer(d time.Duration) (*Model, tea.Cmd) {
	m.sleepDuration = d
	if d == 0 {
		m.sleepDeadline = time.Time{}
		m.playerState.SleepRemaining = ""
		logger.Info("sleep timer disabled")
		return m, nil
	}
	m.sleepGeneration++
	m.sleepDeadline = time.Now().Add(d)
	m.playerState.SleepRemaining = formatSleepRemaining(d)
	logger.Info("sleep timer set", "duration", d)
	return m, sleepTimerCmd(d, m.sleepGeneration)
}

func sleepTimerCmd(d time.Duration, generation uint64) tea.Cmd {
	return tea.Tick(d, func(_ time.Time) tea.Msg {
		return SleepTimerExpiredMsg{Generation: generation}
	})
}

func formatSleepRemaining(d time.Duration) string {
	mins := int(d.Minutes())
	secs := int(d.Seconds()) % 60
	return fmt.Sprintf("%d:%02d", mins, secs)
}
