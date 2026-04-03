package detail

import (
	"strings"

	"github.com/Thelost77/pine/internal/abs"
	"github.com/Thelost77/pine/internal/ui"
	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/bubbles/textinput"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
)

// PlayCmd requests playback of the given library item.
type PlayCmd struct {
	Item abs.LibraryItem
}

// PlayEpisodeCmd requests playback of a specific podcast episode.
type PlayEpisodeCmd struct {
	Item    abs.LibraryItem
	Episode abs.PodcastEpisode
}

// AddBookmarkCmd requests adding a bookmark at the current playback position.
type AddBookmarkCmd struct {
	Item abs.LibraryItem
}

// AddToQueueCmd requests appending the item or selected episode to the queue.
type AddToQueueCmd struct {
	Item    abs.LibraryItem
	Episode *abs.PodcastEpisode
}

// PlayNextCmd requests inserting the item or selected episode at the front of the queue.
type PlayNextCmd struct {
	Item    abs.LibraryItem
	Episode *abs.PodcastEpisode
}

// SeekToBookmarkCmd requests seeking the player to a bookmark's timestamp.
type SeekToBookmarkCmd struct {
	Item abs.LibraryItem
	Time float64
}

// DeleteBookmarkCmd requests deletion of a bookmark.
type DeleteBookmarkCmd struct {
	ItemID   string
	Bookmark abs.Bookmark
}

// UpdateBookmarkCmd requests updating a bookmark title.
type UpdateBookmarkCmd struct {
	ItemID   string
	Bookmark abs.Bookmark
	Title    string
}

// SeekToChapterCmd requests seeking the player to a chapter's start time.
type SeekToChapterCmd struct {
	Time float64
}

// MarkFinishedCmd requests marking an item as finished.
type MarkFinishedCmd struct {
	Item abs.LibraryItem
}

// MarkFinishedMsg updates the item after marking it finished.
type MarkFinishedMsg struct {
	Progress *abs.UserMediaProgress
}

// BookmarksUpdatedMsg updates the bookmark list after an add/delete operation.
type BookmarksUpdatedMsg struct {
	Bookmarks []abs.Bookmark
	Err       error
}

// BackMsg signals that the user wants to go back from the detail screen.
type BackMsg struct{}

// KeyMap defines keybindings for the detail screen.
type KeyMap struct {
	Play         key.Binding
	Bookmark     key.Binding
	Up           key.Binding
	Down         key.Binding
	Back         key.Binding
	Enter        key.Binding
	Delete       key.Binding
	Edit         key.Binding
	ToggleFocus  key.Binding
	MarkFinished key.Binding
	AddToQueue   key.Binding
	PlayNext     key.Binding
}

// DefaultKeyMap returns the default keybindings for the detail screen.
func DefaultKeyMap() KeyMap {
	return KeyMap{
		Play: key.NewBinding(
			key.WithKeys("p", " "),
			key.WithHelp("space/p", "play/pause"),
		),
		Bookmark: key.NewBinding(
			key.WithKeys("b"),
			key.WithHelp("b", "bookmark"),
		),
		Up: key.NewBinding(
			key.WithKeys("k", "up"),
			key.WithHelp("k/↑", "scroll up"),
		),
		Down: key.NewBinding(
			key.WithKeys("j", "down"),
			key.WithHelp("j/↓", "scroll down"),
		),
		Back: key.NewBinding(
			key.WithKeys("esc", "left"),
			key.WithHelp("esc/←", "back"),
		),
		Enter: key.NewBinding(
			key.WithKeys("enter"),
			key.WithHelp("enter", "select"),
		),
		Delete: key.NewBinding(
			key.WithKeys("d"),
			key.WithHelp("d", "delete bookmark"),
		),
		Edit: key.NewBinding(
			key.WithKeys("e"),
			key.WithHelp("e", "edit bookmark"),
		),
		ToggleFocus: key.NewBinding(
			key.WithKeys("tab"),
			key.WithHelp("tab", "toggle bookmarks"),
		),
		MarkFinished: key.NewBinding(
			key.WithKeys("f"),
			key.WithHelp("f", "mark finished"),
		),
		AddToQueue: key.NewBinding(
			key.WithKeys("a"),
			key.WithHelp("a", "add to queue"),
		),
		PlayNext: key.NewBinding(
			key.WithKeys("A"),
			key.WithHelp("A", "play next"),
		),
	}
}

// Model is the bubbletea model for the detail screen.
type Model struct {
	item             abs.LibraryItem
	viewport         viewport.Model
	ready            bool
	keys             KeyMap
	width            int
	height           int
	styles           ui.Styles
	bookmarks        []abs.Bookmark
	bookmarksLoaded  bool
	bookmarkLoadErr  error
	selectedBookmark int
	focusBookmarks   bool
	editingBookmark  bool
	bookmarkEditErr  string
	bookmarkInput    textinput.Model
	episodes         []abs.PodcastEpisode
	selectedEpisode  int
	focusEpisodes    bool
}

// New creates a new detail screen model for the given library item.
func New(styles ui.Styles, item abs.LibraryItem) Model {
	input := textinput.New()
	input.Placeholder = "Bookmark title"
	input.CharLimit = 256
	input.Prompt = ""

	return Model{
		item:             item,
		keys:             DefaultKeyMap(),
		styles:           styles,
		bookmarkInput:    input,
		selectedBookmark: 0,
		episodes:         item.Media.Episodes,
		selectedEpisode:  0,
		focusEpisodes:    item.MediaType == "podcast" && len(item.Media.Episodes) > 0,
	}
}

func (m *Model) refreshContent() {
	if m.ready {
		m.viewport.SetContent(m.buildContent())
	}
}

// SetBookmarks updates the bookmark list and refreshes the viewport content.
func (m *Model) SetBookmarks(bookmarks []abs.Bookmark) {
	m.bookmarks = bookmarks
	m.bookmarksLoaded = true
	m.bookmarkLoadErr = nil
	m.bookmarkEditErr = ""
	if m.selectedBookmark >= len(bookmarks) {
		m.selectedBookmark = max(0, len(bookmarks)-1)
	}
	if len(bookmarks) == 0 {
		m.focusBookmarks = false
		m.editingBookmark = false
	}
	m.refreshContent()
}

// SetSize updates the terminal dimensions for the detail screen.
func (m *Model) SetSize(width, height int) {
	m.width = width
	m.height = height

	headerLines := m.headerHeight()
	vpHeight := height - headerLines
	if vpHeight < 1 {
		vpHeight = 1
	}
	vpWidth := width
	if vpWidth < 1 {
		vpWidth = 1
	}

	if !m.ready {
		m.viewport = viewport.New(vpWidth, vpHeight)
		m.viewport.SetContent(m.buildContent())
		m.ready = true
	} else {
		m.viewport.Width = vpWidth
		m.viewport.Height = vpHeight
		m.viewport.SetContent(m.buildContent())
	}
	m.bookmarkInput.Width = m.bookmarkEditWidth()
}

// Item returns the library item being displayed.
func (m Model) Item() abs.LibraryItem {
	return m.item
}

// Episodes returns the podcast episodes.
func (m Model) Episodes() []abs.PodcastEpisode {
	return m.episodes
}

// SetEpisodes updates the episode list and refreshes the viewport content.
func (m *Model) SetEpisodes(episodes []abs.PodcastEpisode) {
	m.episodes = episodes
	if m.selectedEpisode >= len(episodes) {
		m.selectedEpisode = max(0, len(episodes)-1)
	}
	if len(episodes) > 0 && m.item.MediaType == "podcast" {
		m.focusEpisodes = true
	}
	if m.ready {
		m.viewport.SetContent(m.buildContent())
	}
}

// SelectedEpisode returns the currently selected episode index.
func (m Model) SelectedEpisode() int {
	return m.selectedEpisode
}

// Bookmarks returns the current bookmark list.
func (m Model) Bookmarks() []abs.Bookmark {
	return m.bookmarks
}

// BookmarksLoaded returns whether bookmark loading has completed.
func (m Model) BookmarksLoaded() bool {
	return m.bookmarksLoaded
}

// BookmarkLoadError returns the last bookmark load error, if any.
func (m Model) BookmarkLoadError() error {
	return m.bookmarkLoadErr
}

// SelectedBookmark returns the currently selected bookmark index.
func (m Model) SelectedBookmark() int {
	return m.selectedBookmark
}

// FocusBookmarks returns whether bookmark navigation is focused.
func (m Model) FocusBookmarks() bool {
	return m.focusBookmarks
}

// EditingBookmark returns whether bookmark title editing is active.
func (m Model) EditingBookmark() bool {
	return m.editingBookmark
}

// BookmarkEditError returns the current bookmark edit validation error.
func (m Model) BookmarkEditError() string {
	return m.bookmarkEditErr
}

// Init returns the initial command (none needed).
func (m Model) Init() tea.Cmd {
	return nil
}

// Update handles messages for the detail screen.
func (m Model) Update(msg tea.Msg) (Model, tea.Cmd) {
	switch msg := msg.(type) {
	case MarkFinishedMsg:
		if msg.Progress != nil {
			m.item.UserMediaProgress = msg.Progress
			if m.ready {
				m.viewport.SetContent(m.buildContent())
			}
		}
		return m, nil

	case BookmarksUpdatedMsg:
		m.bookmarksLoaded = true
		m.bookmarkLoadErr = msg.Err
		if msg.Err == nil {
			m.bookmarks = msg.Bookmarks
			m.editingBookmark = false
			m.bookmarkEditErr = ""
			if m.selectedBookmark >= len(m.bookmarks) {
				m.selectedBookmark = max(0, len(m.bookmarks)-1)
			}
		}
		if !m.hasFocusableBookmarks() {
			m.focusBookmarks = false
			m.editingBookmark = false
			m.bookmarkEditErr = ""
		}
		m.refreshContent()
		return m, nil

	case tea.KeyMsg:
		if m.editingBookmark {
			return m.updateBookmarkEditor(msg)
		}
		switch {
		case key.Matches(msg, m.keys.Back):
			return m, func() tea.Msg {
				return BackMsg{}
			}
		case key.Matches(msg, m.keys.Play):
			// For podcasts with episodes, always play the selected episode
			if len(m.episodes) > 0 && m.selectedEpisode < len(m.episodes) {
				item := m.item
				ep := m.episodes[m.selectedEpisode]
				return m, func() tea.Msg {
					return PlayEpisodeCmd{Item: item, Episode: ep}
				}
			}
			item := m.item
			return m, func() tea.Msg {
				return PlayCmd{Item: item}
			}
		case key.Matches(msg, m.keys.Bookmark):
			item := m.item
			return m, func() tea.Msg {
				return AddBookmarkCmd{Item: item}
			}
		case key.Matches(msg, m.keys.AddToQueue):
			item, episode, ok := m.queueTarget()
			if !ok {
				return m, nil
			}
			return m, func() tea.Msg {
				return AddToQueueCmd{Item: item, Episode: episode}
			}
		case key.Matches(msg, m.keys.PlayNext):
			item, episode, ok := m.queueTarget()
			if !ok {
				return m, nil
			}
			return m, func() tea.Msg {
				return PlayNextCmd{Item: item, Episode: episode}
			}
		case key.Matches(msg, m.keys.MarkFinished):
			item := m.item
			return m, func() tea.Msg {
				return MarkFinishedCmd{Item: item}
			}
		case key.Matches(msg, m.keys.ToggleFocus):
			m.cycleFocus()
			if m.ready {
				m.viewport.SetContent(m.buildContent())
			}
			return m, nil
		case key.Matches(msg, m.keys.Enter):
			if m.focusEpisodes && len(m.episodes) > 0 && m.selectedEpisode < len(m.episodes) {
				item := m.item
				ep := m.episodes[m.selectedEpisode]
				return m, func() tea.Msg {
					return PlayEpisodeCmd{Item: item, Episode: ep}
				}
			}
			if m.focusBookmarks && m.hasFocusableBookmarks() && m.selectedBookmark < len(m.bookmarks) {
				item := m.item
				bm := m.bookmarks[m.selectedBookmark]
				return m, func() tea.Msg {
					return SeekToBookmarkCmd{Item: item, Time: bm.Time}
				}
			}
			if !m.focusBookmarks && !m.focusEpisodes {
				item := m.item
				return m, func() tea.Msg {
					return PlayCmd{Item: item}
				}
			}
		case key.Matches(msg, m.keys.Delete):
			if m.focusBookmarks && m.hasFocusableBookmarks() && m.selectedBookmark < len(m.bookmarks) {
				bm := m.bookmarks[m.selectedBookmark]
				itemID := m.item.ID
				return m, func() tea.Msg {
					return DeleteBookmarkCmd{ItemID: itemID, Bookmark: bm}
				}
			}
		case key.Matches(msg, m.keys.Edit):
			if m.focusBookmarks && m.hasFocusableBookmarks() && m.selectedBookmark < len(m.bookmarks) {
				cmd := m.startBookmarkEdit()
				return m, cmd
			}
		case key.Matches(msg, m.keys.Up):
			if m.focusEpisodes && len(m.episodes) > 0 {
				if m.selectedEpisode > 0 {
					m.selectedEpisode--
				}
				m.refreshContent()
				return m, nil
			}
			if m.focusBookmarks && m.hasFocusableBookmarks() {
				if m.selectedBookmark > 0 {
					m.selectedBookmark--
				}
				m.refreshContent()
				return m, nil
			}
		case key.Matches(msg, m.keys.Down):
			if m.focusEpisodes && len(m.episodes) > 0 {
				if m.selectedEpisode < len(m.episodes)-1 {
					m.selectedEpisode++
				}
				m.refreshContent()
				return m, nil
			}
			if m.focusBookmarks && m.hasFocusableBookmarks() {
				if m.selectedBookmark < len(m.bookmarks)-1 {
					m.selectedBookmark++
				}
				m.refreshContent()
				return m, nil
			}
		}
	}

	if m.editingBookmark {
		var cmd tea.Cmd
		m.bookmarkInput, cmd = m.bookmarkInput.Update(msg)
		m.refreshContent()
		return m, cmd
	}

	var cmd tea.Cmd
	m.viewport, cmd = m.viewport.Update(msg)
	return m, cmd
}

// cycleFocus cycles focus between sections.
// Podcasts: episodes → bookmarks → none → episodes
// Books with bookmarks: none ↔ bookmarks
func (m *Model) cycleFocus() {
	if m.item.MediaType == "podcast" && len(m.episodes) > 0 {
		if !m.focusEpisodes && !m.focusBookmarks {
			m.focusEpisodes = true
		} else if m.focusEpisodes {
			m.focusEpisodes = false
			if m.hasFocusableBookmarks() {
				m.focusBookmarks = true
			}
		} else {
			m.focusBookmarks = false
		}
	} else if m.hasFocusableBookmarks() {
		m.focusBookmarks = !m.focusBookmarks
	}
}

func (m Model) hasFocusableBookmarks() bool {
	return m.bookmarkLoadErr == nil && len(m.bookmarks) > 0
}

func (m Model) queueTarget() (abs.LibraryItem, *abs.PodcastEpisode, bool) {
	if m.item.MediaType == "podcast" {
		if !m.focusEpisodes || len(m.episodes) == 0 || m.selectedEpisode >= len(m.episodes) {
			return m.item, nil, false
		}
		ep := m.episodes[m.selectedEpisode]
		return m.item, &ep, true
	}
	return m.item, nil, true
}

func (m Model) bookmarkEditWidth() int {
	width := m.width - 20
	if width < 12 {
		width = 12
	}
	if width > 48 {
		width = 48
	}
	return width
}

func (m *Model) startBookmarkEdit() tea.Cmd {
	if !m.hasFocusableBookmarks() || m.selectedBookmark >= len(m.bookmarks) {
		return nil
	}

	ti := textinput.New()
	ti.Placeholder = "Bookmark title"
	ti.CharLimit = 256
	ti.Prompt = ""
	ti.Width = m.bookmarkEditWidth()
	ti.SetValue(m.bookmarks[m.selectedBookmark].Title)

	m.bookmarkInput = ti
	m.editingBookmark = true
	m.bookmarkEditErr = ""
	m.refreshContent()
	return m.bookmarkInput.Focus()
}

func (m Model) updateBookmarkEditor(msg tea.KeyMsg) (Model, tea.Cmd) {
	switch {
	case key.Matches(msg, m.keys.Back):
		m.editingBookmark = false
		m.bookmarkEditErr = ""
		m.refreshContent()
		return m, nil
	case key.Matches(msg, m.keys.Enter):
		title := strings.TrimSpace(m.bookmarkInput.Value())
		if title == "" {
			m.bookmarkEditErr = "Bookmark title cannot be empty"
			m.refreshContent()
			return m, nil
		}
		bm := m.bookmarks[m.selectedBookmark]
		if title == bm.Title {
			m.editingBookmark = false
			m.bookmarkEditErr = ""
			m.refreshContent()
			return m, nil
		}
		itemID := m.item.ID
		m.bookmarkEditErr = ""
		m.refreshContent()
		return m, func() tea.Msg {
			return UpdateBookmarkCmd{ItemID: itemID, Bookmark: bm, Title: title}
		}
	default:
		if msg.Type != tea.KeyCtrlC {
			m.bookmarkEditErr = ""
		}
		var cmd tea.Cmd
		m.bookmarkInput, cmd = m.bookmarkInput.Update(msg)
		m.refreshContent()
		return m, cmd
	}
}

// headerHeight returns the number of lines used by the fixed header section.
func (m Model) headerHeight() int {
	// title + author + duration + progress bar + blank separator line
	lines := 4 // title, author, duration/progress, blank
	if m.item.UserMediaProgress != nil {
		lines++ // progress bar line
	}
	return lines
}
