package app

import (
	"context"
	"errors"
	"fmt"
	"io"
	"time"

	"github.com/Thelost77/pine/internal/player"
	"github.com/Thelost77/pine/internal/spruce/jellyfin"
	"github.com/Thelost77/pine/internal/spruce/screens/queue"
	tea "github.com/charmbracelet/bubbletea"
)

const subliminalVolume = 20

type subliminalState uint8

const (
	subliminalInactive subliminalState = iota
	subliminalPreparing
	subliminalStarting
	subliminalPlaying
)

func (m *Model) ensureMediaCache() error {
	if m.mediaCache != nil {
		return nil
	}
	cache, err := player.NewSessionCache(player.DefaultMediaCacheLimit)
	if err != nil {
		return err
	}
	m.mediaCache = cache
	m.mediaContext, m.mediaCancel = context.WithCancel(context.Background())
	return nil
}

func (m *Model) cancelRepeatWork() {
	if m.repeatCancel != nil {
		m.repeatCancel()
	}
	m.repeatCancel = nil
	m.repeatContext = nil
}

func (m *Model) ensureRepeatContext() error {
	if err := m.ensureMediaCache(); err != nil {
		return err
	}
	if m.repeatContext == nil {
		m.repeatContext, m.repeatCancel = context.WithCancel(m.mediaContext)
	}
	return nil
}

func (m *Model) cacheTrackCmd(ctx context.Context, track jellyfin.Track, index int, start bool, generation uint64) tea.Cmd {
	cache := m.mediaCache
	client := m.client
	return func() tea.Msg {
		if cache == nil || client == nil {
			return repeatCacheMsg{
				Generation: generation,
				ItemID:     track.ID,
				Index:      index,
				Start:      start,
				Err:        errors.New("media cache is unavailable"),
			}
		}
		_, err := cache.Store(ctx, track.ID, func(downloadCtx context.Context, dst io.Writer) (int64, error) {
			return client.DownloadAudio(downloadCtx, track.ID, dst)
		})
		return repeatCacheMsg{
			Generation: generation,
			ItemID:     track.ID,
			Index:      index,
			Start:      start,
			Err:        err,
		}
	}
}

func (m *Model) prefetchRepeatTrack(index int) tea.Cmd {
	if index < 0 || index >= len(m.tracks) || m.client == nil {
		return nil
	}
	if err := m.ensureRepeatContext(); err != nil {
		return m.err.SetError(fmt.Errorf("prepare repeat cache: %w", err))
	}
	track := m.tracks[index]
	if _, ok := m.mediaCache.Path(track.ID); ok {
		return nil
	}
	return m.cacheTrackCmd(m.repeatContext, track, index, false, m.repeatGeneration)
}

func (m *Model) prefetchRepeatQueue(index int) tea.Cmd {
	if len(m.tracks) == 0 {
		return nil
	}
	cmds := []tea.Cmd{m.prefetchRepeatTrack(index)}
	next := index + 1
	if next >= len(m.tracks) {
		next = 0
	}
	if next != index {
		cmds = append(cmds, m.prefetchRepeatTrack(next))
	}
	return tea.Batch(cmds...)
}

func (m *Model) startRepeatedPlaybackAt(index int) (*Model, tea.Cmd) {
	if index < 0 || index >= len(m.tracks) {
		return m.stopPlayback()
	}
	if err := m.ensureRepeatContext(); err != nil {
		newM, playCmd := m.startPlaybackAt(index)
		return newM, tea.Batch(playCmd, m.err.SetError(fmt.Errorf("prepare repeat cache: %w", err)))
	}
	track := m.tracks[index]
	if _, ok := m.mediaCache.Path(track.ID); ok {
		return m.startPlaybackAt(index)
	}

	m.playGeneration++
	m.pendingRepeatIndex = index
	m.pendingRepeatItemID = track.ID
	m.playerState.Playing = false
	m.updateRepeatStatus(true)
	m.syncQueueScreen()
	return m, m.cacheTrackCmd(m.repeatContext, track, index, true, m.repeatGeneration)
}

func (m *Model) updateRepeatStatus(waiting bool) {
	suffix := ""
	if waiting {
		suffix = " (caching)"
	}
	switch {
	case m.repeatTrackID != "":
		m.playerState.RepeatStatus = "🔂 Track" + suffix
	case m.repeatQueue:
		m.playerState.RepeatStatus = "🔁 Queue" + suffix
	default:
		m.playerState.RepeatStatus = ""
	}
}

func (m *Model) handleRepeatCacheMsg(msg repeatCacheMsg) (*Model, tea.Cmd) {
	if msg.Generation != m.repeatGeneration {
		return m, nil
	}
	if !msg.Start {
		if msg.Err != nil && jellyfin.IsHTTPStatus(msg.Err, 401) {
			newModel, cmd := m.handleAuthReset(msg.Err)
			return newModel.(*Model), cmd
		}
		if msg.Err != nil && (m.repeatQueue || m.repeatTrackID == msg.ItemID) {
			return m, m.err.SetError(fmt.Errorf("cache repeated track: %w", msg.Err))
		}
		return m, nil
	}
	if m.pendingRepeatItemID != msg.ItemID {
		return m, nil
	}
	if !m.repeatQueue && m.repeatTrackID != msg.ItemID {
		return m, nil
	}
	index := m.resolvePendingRepeatIndex()
	m.pendingRepeatIndex = -1
	m.pendingRepeatItemID = ""
	m.updateRepeatStatus(false)
	if msg.Err != nil && jellyfin.IsHTTPStatus(msg.Err, 401) {
		newModel, cmd := m.handleAuthReset(msg.Err)
		return newModel.(*Model), cmd
	}
	if index < 0 {
		if m.repeatQueue && len(m.tracks) > 0 {
			next := m.currentIndex + 1
			if next < 0 || next >= len(m.tracks) {
				next = 0
			}
			return m.startRepeatedPlaybackAt(next)
		}
		return m.stopPlayback()
	}
	newM, playCmd := m.startPlaybackAt(index)
	if msg.Err != nil {
		return newM, tea.Batch(playCmd, m.err.SetError(fmt.Errorf("cache repeated track: %w", msg.Err)))
	}
	return newM, playCmd
}

func (m *Model) resolvePendingRepeatIndex() int {
	if m.pendingRepeatItemID == "" {
		return -1
	}
	if m.pendingRepeatIndex >= 0 && m.pendingRepeatIndex < len(m.tracks) && m.tracks[m.pendingRepeatIndex].ID == m.pendingRepeatItemID {
		return m.pendingRepeatIndex
	}
	for index, track := range m.tracks {
		if track.ID == m.pendingRepeatItemID {
			return index
		}
	}
	return -1
}

func (m *Model) isSubliminalActive() bool {
	return m.subliminalState != subliminalInactive
}

func (m *Model) isSubliminalRunning() bool {
	return m.subliminalState == subliminalPlaying
}

func (m *Model) toggleSubliminalMode() (*Model, tea.Cmd) {
	if m.isSubliminalActive() {
		return m.stopSubliminalMode()
	}
	if len(m.tracks) == 0 {
		return m, m.err.SetError(errors.New("cannot start subliminal mode with an empty queue"))
	}
	if len(m.tracks) > player.MaxSubliminalTracks {
		return m, m.err.SetError(fmt.Errorf("subliminal mode supports at most %d queued tracks", player.MaxSubliminalTracks))
	}
	if m.client == nil || (m.subliminalGroupFactory == nil && m.subliminalGroup == nil) {
		return m, m.err.SetError(errors.New("subliminal playback is unavailable"))
	}
	if m.subliminalGroupFactory != nil {
		m.subliminalGroup = m.subliminalGroupFactory()
	}
	if m.subliminalGroup == nil {
		return m, m.err.SetError(errors.New("subliminal player group is unavailable"))
	}
	if err := m.ensureMediaCache(); err != nil {
		return m, m.err.SetError(fmt.Errorf("prepare subliminal cache: %w", err))
	}

	m.cancelRepeatWork()
	m.repeatGeneration++
	m.repeatTrackID = ""
	m.repeatQueue = false
	m.pendingRepeatIndex = -1
	m.pendingRepeatItemID = ""
	m.updateRepeatStatus(false)
	m.subliminalGeneration++
	m.subliminalState = subliminalPreparing
	m.subliminalSnapshot = append([]jellyfin.Track(nil), m.tracks...)
	m.subliminalCacheTracks = uniqueTracks(m.subliminalSnapshot)
	m.subliminalCachedCount = 0
	m.subliminalContext, m.subliminalCancel = context.WithCancel(m.mediaContext)
	m.syncQueueScreen()
	return m, m.cacheNextSubliminalTrackCmd()
}

func uniqueTracks(tracks []jellyfin.Track) []jellyfin.Track {
	seen := make(map[string]struct{}, len(tracks))
	unique := make([]jellyfin.Track, 0, len(tracks))
	for _, track := range tracks {
		if _, ok := seen[track.ID]; ok {
			continue
		}
		seen[track.ID] = struct{}{}
		unique = append(unique, track)
	}
	return unique
}

func (m *Model) cacheNextSubliminalTrackCmd() tea.Cmd {
	if m.subliminalCachedCount >= len(m.subliminalCacheTracks) {
		return nil
	}
	track := m.subliminalCacheTracks[m.subliminalCachedCount]
	cache := m.mediaCache
	client := m.client
	ctx := m.subliminalContext
	generation := m.subliminalGeneration
	return func() tea.Msg {
		_, err := cache.Store(ctx, track.ID, func(downloadCtx context.Context, dst io.Writer) (int64, error) {
			return client.DownloadAudio(downloadCtx, track.ID, dst)
		})
		return subliminalCacheMsg{Generation: generation, ItemID: track.ID, Err: err}
	}
}

func (m *Model) handleSubliminalCacheMsg(msg subliminalCacheMsg) (*Model, tea.Cmd) {
	if msg.Generation != m.subliminalGeneration || m.subliminalState != subliminalPreparing {
		return m, nil
	}
	if m.subliminalCachedCount >= len(m.subliminalCacheTracks) || m.subliminalCacheTracks[m.subliminalCachedCount].ID != msg.ItemID {
		return m, nil
	}
	if msg.Err != nil {
		if jellyfin.IsHTTPStatus(msg.Err, 401) {
			newModel, cmd := m.handleAuthReset(msg.Err)
			return newModel.(*Model), cmd
		}
		if m.subliminalCancel != nil {
			m.subliminalCancel()
		}
		m.resetSubliminalPreparation()
		m.syncQueueScreen()
		return m, m.err.SetError(fmt.Errorf("prepare subliminal track: %w", msg.Err))
	}
	m.subliminalCachedCount++
	m.syncQueueScreen()
	if m.subliminalCachedCount < len(m.subliminalCacheTracks) {
		return m, m.cacheNextSubliminalTrackCmd()
	}
	return m.startSubliminalGroup()
}

func (m *Model) startSubliminalGroup() (*Model, tea.Cmd) {
	paths := make([]string, len(m.subliminalSnapshot))
	for i, track := range m.subliminalSnapshot {
		path, ok := m.mediaCache.Path(track.ID)
		if !ok {
			if m.subliminalCancel != nil {
				m.subliminalCancel()
			}
			m.resetSubliminalPreparation()
			return m, m.err.SetError(fmt.Errorf("subliminal track %q is not cached", track.Name))
		}
		paths[i] = path
	}

	m.normalVolume = m.playerState.Volume
	m.subliminalState = subliminalStarting
	newM, stopCmd := m.stopPlayback()
	m = newM
	m.playerState.Title = fmt.Sprintf("Subliminal mix (%d tracks)", len(paths))
	m.playerState.Playing = false
	m.playerState.Position = 0
	m.playerState.Duration = 0
	m.playerState.Volume = subliminalVolume
	m.playerState.RepeatStatus = ""
	m.playerState.SubliminalCount = len(paths)
	m.syncQueueScreen()
	launchCmd := player.LaunchGroupCmd(m.subliminalContext, m.subliminalGroup, paths, subliminalVolume, m.subliminalGeneration)
	return m, tea.Batch(stopCmd, launchCmd)
}

func (m *Model) handleGroupReadyMsg(msg player.GroupReadyMsg) (*Model, tea.Cmd) {
	if msg.Generation != m.subliminalGeneration || m.subliminalState != subliminalStarting {
		return m, nil
	}
	if msg.Err != nil {
		newM, stopCmd := m.stopSubliminalMode()
		return newM, tea.Batch(stopCmd, m.err.SetError(fmt.Errorf("start subliminal mode: %w", msg.Err)))
	}
	m.subliminalState = subliminalPlaying
	m.playerState.Playing = true
	m.syncQueueScreen()
	return m, player.WatchGroupCmd(m.subliminalContext, m.subliminalGroup, m.subliminalGeneration)
}

func (m *Model) handleGroupEndedMsg(msg player.GroupEndedMsg) (*Model, tea.Cmd) {
	if msg.Generation != m.subliminalGeneration || !m.isSubliminalRunning() {
		return m, nil
	}
	newM, stopCmd := m.stopSubliminalMode()
	if msg.Err == nil || errors.Is(msg.Err, context.Canceled) {
		return newM, stopCmd
	}
	return newM, tea.Batch(stopCmd, m.err.SetError(fmt.Errorf("subliminal playback ended: %w", msg.Err)))
}

func (m *Model) handleGroupStoppedMsg(msg player.GroupStoppedMsg) {
	for index, group := range m.subliminalRetired {
		if group == msg.Group {
			m.subliminalRetired = append(m.subliminalRetired[:index], m.subliminalRetired[index+1:]...)
			return
		}
	}
}

func (m *Model) handleGroupControlMsg(msg player.GroupControlMsg) (*Model, tea.Cmd) {
	if msg.Generation != m.subliminalGeneration || !m.isSubliminalRunning() || msg.Err == nil {
		return m, nil
	}
	newM, stopCmd := m.stopSubliminalMode()
	return newM, tea.Batch(stopCmd, m.err.SetError(fmt.Errorf("control subliminal playback: %w", msg.Err)))
}

func (m *Model) setSubliminalPaused(paused bool) (*Model, tea.Cmd) {
	if !m.isSubliminalRunning() {
		return m, nil
	}
	m.playerState.Playing = !paused
	m.syncQueueScreen()
	return m, player.SetGroupPauseCmd(m.subliminalGroup, paused, m.subliminalGeneration)
}

func (m *Model) setSubliminalVolume(volume int) (*Model, tea.Cmd) {
	if !m.isSubliminalRunning() {
		return m, nil
	}
	if volume < 0 {
		volume = 0
	} else if volume > 150 {
		volume = 150
	}
	m.playerState.Volume = volume
	m.syncQueueScreen()
	return m, player.SetGroupVolumeCmd(m.subliminalGroup, volume, m.subliminalGeneration)
}

func (m *Model) stopSubliminalMode() (*Model, tea.Cmd) {
	if !m.isSubliminalActive() {
		return m, nil
	}
	wasPreparing := m.subliminalState == subliminalPreparing
	if m.subliminalCancel != nil {
		m.subliminalCancel()
	}
	m.subliminalGeneration++
	m.resetSubliminalPreparation()
	if wasPreparing {
		m.subliminalGroup = nil
		m.syncQueueScreen()
		return m, nil
	}

	m.playerState.Title = ""
	m.playerState.Playing = false
	m.playerState.Position = 0
	m.playerState.Duration = 0
	m.playerState.Volume = m.normalVolume
	m.playerState.SubliminalCount = 0
	m.sleepDeadline = time.Time{}
	m.sleepDuration = 0
	m.playerState.SleepRemaining = ""
	m.currentIndex = -1
	group := m.subliminalGroup
	m.subliminalGroup = nil
	if group != nil {
		m.subliminalRetired = append(m.subliminalRetired, group)
	}
	m.syncQueueScreen()
	return m, player.QuitGroupCmd(group)
}

func (m *Model) stopAllPlayback() (*Model, tea.Cmd) {
	var cmds []tea.Cmd
	if m.isSubliminalActive() {
		var cmd tea.Cmd
		m, cmd = m.stopSubliminalMode()
		cmds = append(cmds, cmd)
	}
	if m.IsPlaying() {
		var cmd tea.Cmd
		m, cmd = m.stopPlayback()
		cmds = append(cmds, cmd)
	}
	return m, tea.Batch(cmds...)
}

func (m *Model) resetSubliminalPreparation() {
	m.subliminalState = subliminalInactive
	m.subliminalCancel = nil
	m.subliminalContext = nil
	m.subliminalSnapshot = nil
	m.subliminalCacheTracks = nil
	m.subliminalCachedCount = 0
}

func (m *Model) subliminalStatus() (queue.SubliminalStatus, int, int) {
	switch m.subliminalState {
	case subliminalPreparing:
		return queue.SubliminalPreparing, m.subliminalCachedCount, len(m.subliminalCacheTracks)
	case subliminalStarting:
		return queue.SubliminalStarting, len(m.subliminalCacheTracks), len(m.subliminalCacheTracks)
	case subliminalPlaying:
		if m.playerState.Playing {
			return queue.SubliminalPlaying, len(m.subliminalSnapshot), len(m.subliminalSnapshot)
		}
		return queue.SubliminalPaused, len(m.subliminalSnapshot), len(m.subliminalSnapshot)
	default:
		return queue.SubliminalInactive, 0, 0
	}
}
