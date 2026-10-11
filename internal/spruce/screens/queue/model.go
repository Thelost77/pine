package queue

import (
	"github.com/Thelost77/pine/internal/spruce/jellyfin"
	"github.com/Thelost77/pine/internal/spruce/ui"
	"github.com/Thelost77/pine/internal/spruce/ui/components"
	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/bubbles/list"
	tea "github.com/charmbracelet/bubbletea"
)

type SubliminalStatus uint8

const (
	SubliminalInactive SubliminalStatus = iota
	SubliminalPreparing
	SubliminalStarting
	SubliminalPlaying
	SubliminalPaused
)

type Model struct {
	list list.Model

	tracks       []jellyfin.Track
	currentIndex int

	isPlaying       bool
	isPaused        bool
	positionSeconds float64
	durationSeconds float64
	isShuffle       bool

	subliminalStatus   SubliminalStatus
	subliminalComplete int
	subliminalTotal    int
	subliminalVolume   int

	width  int
	height int
	styles ui.Styles
}

func New(styles ui.Styles) Model {
	del := list.NewDefaultDelegate()
	del.Styles.SelectedTitle = del.Styles.SelectedTitle.Foreground(styles.Accent.GetForeground()).BorderForeground(styles.Accent.GetForeground())
	del.Styles.SelectedDesc = del.Styles.SelectedDesc.Foreground(styles.Muted.GetForeground()).BorderForeground(styles.Accent.GetForeground())

	l := list.New(nil, del, 0, 0)
	l.KeyMap.Quit.SetKeys("q")
	l.KeyMap.PrevPage.SetKeys("pgup", "u")
	l.KeyMap.NextPage.SetKeys("pgdown")
	l.Title = "Queue / Now Playing"
	l.SetShowTitle(false)
	l.SetShowHelp(false)
	l.SetShowStatusBar(true)
	l.SetStatusBarItemName("queued", "queued")
	l.SetFilteringEnabled(true)
	l.Filter = components.UnicodeFilter
	l.AdditionalFullHelpKeys = func() []key.Binding {
		return []key.Binding{
			key.NewBinding(key.WithKeys("enter"), key.WithHelp("enter", "jump to")),
			key.NewBinding(key.WithKeys("d", "x", "delete", "backspace"), key.WithHelp("d/x/del", "remove")),
			key.NewBinding(key.WithKeys("c"), key.WithHelp("c", "clear queue")),
			key.NewBinding(key.WithKeys("space"), key.WithHelp("space", "pause/resume")),
			key.NewBinding(key.WithKeys("n", ">"), key.WithHelp("n/>", "next")),
			key.NewBinding(key.WithKeys("p", "<"), key.WithHelp("p/<", "prev")),
			key.NewBinding(key.WithKeys("s"), key.WithHelp("s", "shuffle")),
			key.NewBinding(key.WithKeys("r"), key.WithHelp("r", "repeat track")),
			key.NewBinding(key.WithKeys("R"), key.WithHelp("R", "repeat queue")),
			key.NewBinding(key.WithKeys("b"), key.WithHelp("b", "subliminal mode")),
		}
	}

	return Model{
		list:   l,
		styles: styles,
	}
}

func (m *Model) updateListSize() {
	listH := m.height
	if m.subliminalStatus != SubliminalInactive || (m.isPlaying && m.currentIndex >= 0 && m.currentIndex < len(m.tracks)) {
		listH -= 4
	}
	if listH < 1 {
		listH = 1
	}
	m.list.SetSize(m.width, listH)
}

func (m *Model) SetSize(width, height int) {
	m.width = width
	m.height = height
	m.updateListSize()
}

func (m *Model) SetQueue(tracks []jellyfin.Track, currentIndex int) {
	oldTracks := m.tracks
	oldCurrentIndex := m.currentIndex
	m.tracks = tracks
	m.currentIndex = currentIndex
	items := make([]list.Item, len(tracks))
	for i, t := range tracks {
		items[i] = queueItem{
			Track:     t,
			Index:     i,
			IsCurrent: i == currentIndex,
		}
	}
	cmd := m.list.SetItems(items)
	_ = cmd // SetItems returns cmd for spinner/etc if needed
	if !m.HasActiveFilter() && shouldSelectCurrent(oldTracks, tracks, oldCurrentIndex, currentIndex) {
		if currentIndex >= 0 && currentIndex < len(items) {
			m.list.Select(currentIndex)
		}
	}
	m.updateListSize()
}

func (m *Model) SetCursor(idx int) {
	if len(m.tracks) == 0 {
		m.list.Select(0)
		return
	}
	if idx < 0 {
		idx = 0
	} else if idx >= len(m.tracks) {
		idx = len(m.tracks) - 1
	}
	m.list.Select(idx)
}

func shouldSelectCurrent(oldTracks, newTracks []jellyfin.Track, oldCurrentIndex, newCurrentIndex int) bool {
	if newCurrentIndex < 0 || newCurrentIndex >= len(newTracks) {
		return false
	}
	if oldCurrentIndex < 0 || oldCurrentIndex >= len(oldTracks) {
		return true
	}
	if oldCurrentIndex != newCurrentIndex || len(oldTracks) != len(newTracks) {
		return true
	}
	for i := range newTracks {
		if oldTracks[i].ID != newTracks[i].ID {
			return true
		}
	}
	return false
}

func (m *Model) SetPlaybackState(isPlaying, isPaused bool, position, duration float64) {
	m.isPlaying = isPlaying
	m.isPaused = isPaused
	m.positionSeconds = position
	m.durationSeconds = duration
	m.updateListSize()
}

func (m *Model) SetShuffle(isShuffle bool) {
	m.isShuffle = isShuffle
}

func (m *Model) SetSubliminalState(status SubliminalStatus, complete, total, volume int) {
	m.subliminalStatus = status
	m.subliminalComplete = complete
	m.subliminalTotal = total
	m.subliminalVolume = volume
	m.updateListSize()
}

func (m Model) Init() tea.Cmd {
	return nil
}

func (m Model) Update(msg tea.Msg) (Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		if m.list.FilterState() == list.Filtering {
			break
		}
		switch msg.String() {
		case "esc", "left":
			if m.HasActiveFilter() {
				m.list.ResetFilter()
				return m, nil
			}
		case "enter":
			if sel, ok := m.list.SelectedItem().(queueItem); ok {
				m.list.ResetFilter()
				return m, func() tea.Msg { return JumpQueueMsg{Index: sel.Index} }
			}
		case "d", "x", "delete", "backspace":
			if sel, ok := m.list.SelectedItem().(queueItem); ok {
				return m, func() tea.Msg { return RemoveQueueMsg{Index: sel.Index} }
			}
		case "c":
			if len(m.tracks) > 0 {
				return m, func() tea.Msg { return QueueActionMsg{Action: "clear"} }
			}
		case " ", "space":
			return m, func() tea.Msg { return QueueActionMsg{Action: "toggle_pause"} }
		case "n", ">":
			return m, func() tea.Msg { return QueueActionMsg{Action: "next"} }
		case "p", "<":
			return m, func() tea.Msg { return QueueActionMsg{Action: "prev"} }
		case "s":
			return m, func() tea.Msg { return QueueActionMsg{Action: "shuffle"} }
		case "m":
			if sel, ok := m.list.SelectedItem().(queueItem); ok {
				return m, func() tea.Msg {
					return QueueActionMsg{Action: "edit_metadata", Index: sel.Index, TrackID: sel.Track.ID}
				}
			}
		case "r":
			if sel, ok := m.list.SelectedItem().(queueItem); ok {
				return m, func() tea.Msg {
					return QueueActionMsg{Action: "repeat_track", Index: sel.Index, TrackID: sel.Track.ID}
				}
			}
		case "R":
			return m, func() tea.Msg { return QueueActionMsg{Action: "repeat_queue"} }
		case "b":
			return m, func() tea.Msg { return QueueActionMsg{Action: "subliminal"} }
		case "L":
			before := m.list.GlobalIndex()
			m.list.NextPage()
			if m.list.GlobalIndex() == before {
				m.list.GoToEnd()
			}
			return m, nil
		case "H":
			before := m.list.GlobalIndex()
			m.list.PrevPage()
			if m.list.GlobalIndex() == before {
				m.list.GoToStart()
			}
			return m, nil
		}
	}

	var cmd tea.Cmd
	m.list, cmd = m.list.Update(msg)
	return m, cmd
}

func (m Model) CurrentIndex() int {
	return m.currentIndex
}

func (m Model) Tracks() []jellyfin.Track {
	return m.tracks
}

func (m Model) IsFiltering() bool {
	return m.list.FilterState() == list.Filtering
}

func (m Model) HasActiveFilter() bool {
	return m.list.FilterValue() != "" || m.list.FilterState() == list.FilterApplied
}
