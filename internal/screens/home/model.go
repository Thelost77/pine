package home

import (
	"context"
	"fmt"

	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/bubbles/list"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/Thelost77/pine/internal/abs"
	"github.com/Thelost77/pine/internal/ui"
)

// PersonalizedMsg carries the result of fetching personalized data.
type PersonalizedMsg struct {
	Items     []abs.LibraryItem
	Libraries []abs.Library
	Err       error
}

// listItem wraps a LibraryItem for the bubbles list component.
type listItem struct {
	item abs.LibraryItem
}

func (i listItem) Title() string {
	if i.item.MediaType == "podcast" && i.item.RecentEpisode != nil {
		return i.item.RecentEpisode.Title
	}
	return i.item.Media.Metadata.Title
}

func (i listItem) Description() string {
	author := "Unknown author"
	if i.item.MediaType == "podcast" && i.item.RecentEpisode != nil {
		// For podcasts, show podcast name as description context
		author = i.item.Media.Metadata.Title
	} else if i.item.Media.Metadata.AuthorName != nil {
		author = *i.item.Media.Metadata.AuthorName
	}

	progress := ""
	if i.item.UserMediaProgress != nil {
		progress = fmt.Sprintf(" • %d%%", int(i.item.UserMediaProgress.Progress*100))
	}

	duration := ""
	if i.item.Media.HasDuration() {
		duration = fmt.Sprintf(" • %s", ui.FormatDuration(i.item.Media.TotalDuration()))
	}

	return author + progress + duration
}

func (i listItem) FilterValue() string {
	return i.item.Media.Metadata.Title
}

// KeyMap defines keybindings for the home screen.
type KeyMap struct {
	Enter   key.Binding
	Back    key.Binding
	Library key.Binding
	Search  key.Binding
	NextLib key.Binding
	Select  key.Binding
}

// DefaultKeyMap returns the default keybindings for the home screen.
func DefaultKeyMap() KeyMap {
	return KeyMap{
		Enter: key.NewBinding(
			key.WithKeys("enter"),
			key.WithHelp("enter", "select"),
		),
		Back: key.NewBinding(
			key.WithKeys("left"),
			key.WithHelp("←", "back"),
		),
		Select: key.NewBinding(
			key.WithKeys("right"),
			key.WithHelp("→", "select"),
		),
		Library: key.NewBinding(
			key.WithKeys("o"),
			key.WithHelp("o", "open library"),
		),
		Search: key.NewBinding(
			key.WithKeys("/"),
			key.WithHelp("/", "search"),
		),
		NextLib: key.NewBinding(
			key.WithKeys("tab"),
			key.WithHelp("tab", "next library"),
		),
	}
}

// Model is the bubbletea model for the home screen.
type Model struct {
	list            list.Model
	items           []abs.LibraryItem
	loading         bool
	err             error
	keys            KeyMap
	width           int
	height          int
	styles          ui.Styles
	client          *abs.Client
	libraries       []abs.Library
	selectedLibrary int
}

// New creates a new home screen model.
func New(styles ui.Styles, client *abs.Client) Model {
	delegate := list.NewDefaultDelegate()
	delegate.Styles.SelectedTitle = delegate.Styles.SelectedTitle.
		Foreground(styles.Accent.GetForeground()).
		BorderForeground(styles.Accent.GetForeground())
	delegate.Styles.SelectedDesc = delegate.Styles.SelectedDesc.
		Foreground(styles.Muted.GetForeground()).
		BorderForeground(styles.Accent.GetForeground())

	l := list.New(nil, delegate, 0, 0)
	l.Title = "Continue Listening"
	l.SetShowStatusBar(true)
	l.SetFilteringEnabled(false)
	l.SetShowHelp(false)
	l.DisableQuitKeybindings()

	return Model{
		list:   l,
		keys:   DefaultKeyMap(),
		styles: styles,
		client: client,
	}
}

// SetSize updates the terminal dimensions for the home screen.
func (m *Model) SetSize(width, height int) {
	m.width = width
	m.height = height
	m.list.SetSize(width, height)
}

// Init returns the initial command that fetches personalized data.
func (m Model) Init() tea.Cmd {
	return m.fetchPersonalizedCmd()
}

// Update handles messages for the home screen.
func (m Model) Update(msg tea.Msg) (Model, tea.Cmd) {
	switch msg := msg.(type) {
	case PersonalizedMsg:
		m.loading = false
		if msg.Err != nil {
			m.err = msg.Err
			return m, nil
		}
		if len(msg.Libraries) > 0 {
			m.libraries = msg.Libraries
		}
		m.items = msg.Items
		items := make([]list.Item, len(msg.Items))
		for i, item := range msg.Items {
			items[i] = listItem{item: item}
		}
		m.list.SetItems(items)
		m.updateListTitle()
		return m, nil

	case tea.KeyMsg:
		// Don't intercept keys while filtering
		if m.list.FilterState() == list.Filtering {
			break
		}

		switch {
		case key.Matches(msg, m.keys.Enter), key.Matches(msg, m.keys.Select):
			if sel, ok := m.list.SelectedItem().(listItem); ok {
				item := sel.item
				// Podcast with a recent episode → play it directly
				if item.MediaType == "podcast" && item.RecentEpisode != nil {
					ep := *item.RecentEpisode
					return m, func() tea.Msg {
						return PlayEpisodeMsg{Item: item, Episode: ep}
					}
				}
				return m, func() tea.Msg {
					return NavigateDetailMsg{Item: item}
				}
			}
		case key.Matches(msg, m.keys.Library):
			libID := m.SelectedLibraryID()
			libs := m.libraries
			return m, func() tea.Msg {
				return NavigateLibraryMsg{LibraryID: libID, Libraries: libs}
			}
		case key.Matches(msg, m.keys.Search):
			libID := m.SelectedLibraryID()
			return m, func() tea.Msg {
				return NavigateSearchMsg{LibraryID: libID}
			}
		case key.Matches(msg, m.keys.NextLib):
			if len(m.libraries) > 1 {
				m.selectedLibrary = (m.selectedLibrary + 1) % len(m.libraries)
				m.updateListTitle()
				return m, m.fetchPersonalizedCmd()
			}
			return m, nil
		case key.Matches(msg, m.keys.Back):
			return m, func() tea.Msg { return GoBackMsg{} }
		}
	}

	var cmd tea.Cmd
	m.list, cmd = m.list.Update(msg)
	return m, cmd
}

// fetchPersonalizedCmd creates a command that fetches personalized shelves.
func (m *Model) fetchPersonalizedCmd() tea.Cmd {
	if m.client == nil {
		return func() tea.Msg {
			return PersonalizedMsg{Err: fmt.Errorf("not authenticated")}
		}
	}
	m.loading = true
	client := m.client
	selectedIdx := m.selectedLibrary
	existingLibs := m.libraries
	return func() tea.Msg {
		libs := existingLibs
		// Fetch libraries if we don't have them yet
		if len(libs) == 0 {
			var err error
			libs, err = client.GetLibraries(context.Background())
			if err != nil {
				return PersonalizedMsg{Err: fmt.Errorf("fetch libraries: %w", err)}
			}
			libs, _ = client.FilterAudioLibraries(context.Background(), libs)
		}
		if len(libs) == 0 {
			return PersonalizedMsg{Items: nil, Libraries: libs}
		}

		idx := selectedIdx
		if idx >= len(libs) {
			idx = 0
		}

		sections, err := client.GetPersonalized(context.Background(), libs[idx].ID)
		if err != nil {
			return PersonalizedMsg{Err: fmt.Errorf("fetch personalized: %w", err), Libraries: libs}
		}

		// Find the "continue-listening" section
		for _, section := range sections {
			if section.ID == "continue-listening" {
				return PersonalizedMsg{Items: section.Entities, Libraries: libs}
			}
		}

		// No continue-listening section found; return empty
		return PersonalizedMsg{Items: nil, Libraries: libs}
	}
}

// NavigateDetailMsg requests navigation to the detail screen for an item.
type NavigateDetailMsg struct {
	Item abs.LibraryItem
}

// PlayEpisodeMsg requests direct playback of a podcast episode from the home screen.
type PlayEpisodeMsg struct {
	Item    abs.LibraryItem
	Episode abs.PodcastEpisode
}

// PlayMsg requests direct playback of an audiobook from the home screen.
type PlayMsg struct {
	Item abs.LibraryItem
}

// NavigateLibraryMsg requests navigation to the library screen.
type NavigateLibraryMsg struct {
	LibraryID string
	Libraries []abs.Library
}

// NavigateSearchMsg requests navigation to the search screen.
type NavigateSearchMsg struct {
	LibraryID string
}

// GoBackMsg requests navigating back from the home screen.
type GoBackMsg struct{}

// Items returns the current library items.
func (m Model) Items() []abs.LibraryItem {
	return m.items
}

// Libraries returns the available libraries.
func (m Model) Libraries() []abs.Library {
	return m.libraries
}

// SelectedLibraryID returns the ID of the currently selected library, or empty string.
func (m Model) SelectedLibraryID() string {
	if len(m.libraries) == 0 {
		return ""
	}
	idx := m.selectedLibrary
	if idx >= len(m.libraries) {
		idx = 0
	}
	return m.libraries[idx].ID
}

// updateListTitle updates the list title to show the selected library name.
func (m *Model) updateListTitle() {
	title := "Continue Listening"
	if len(m.libraries) > 1 && m.selectedLibrary < len(m.libraries) {
		title = fmt.Sprintf("Continue Listening — %s (tab to switch)", m.libraries[m.selectedLibrary].Name)
	}
	m.list.Title = title
}

// Loading returns whether data is being fetched.
func (m Model) Loading() bool {
	return m.loading
}

// Error returns the last error, if any.
func (m Model) Error() error {
	return m.err
}

