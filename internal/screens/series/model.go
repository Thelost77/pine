package series

import (
	"context"
	"fmt"

	"github.com/Thelost77/pine/internal/abs"
	"github.com/Thelost77/pine/internal/ui"
	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/bubbles/list"
	tea "github.com/charmbracelet/bubbletea"
)

// LoadedMsg carries a loaded series payload.
type LoadedMsg struct {
	Series abs.Series
	Err    error
}

// NavigateDetailMsg requests opening a book from the series list.
type NavigateDetailMsg struct {
	Item abs.LibraryItem
}

// BackMsg signals leaving the series screen.
type BackMsg struct{}

// KeyMap defines series screen bindings.
type KeyMap struct {
	Enter key.Binding
	Back  key.Binding
}

// DefaultKeyMap returns default series bindings.
func DefaultKeyMap() KeyMap {
	return KeyMap{
		Enter: key.NewBinding(
			key.WithKeys("enter"),
			key.WithHelp("enter", "open detail"),
		),
		Back: key.NewBinding(
			key.WithKeys("esc", "left"),
			key.WithHelp("esc/left", "back"),
		),
	}
}

// Model is the Bubble Tea model for the series screen.
type Model struct {
	list          list.Model
	keys          KeyMap
	width         int
	height        int
	styles        ui.Styles
	client        *abs.Client
	libraryID     string
	seriesID      string
	currentItemID string
	series        abs.Series
	loading       bool
	err           error
}

// New creates a series screen model.
func New(styles ui.Styles, client *abs.Client, libraryID, seriesID, currentItemID string) Model {
	delegate := list.NewDefaultDelegate()
	delegate.Styles.SelectedTitle = delegate.Styles.SelectedTitle.
		Foreground(styles.Accent.GetForeground()).
		BorderForeground(styles.Accent.GetForeground())
	delegate.Styles.SelectedDesc = delegate.Styles.SelectedDesc.
		Foreground(styles.Muted.GetForeground()).
		BorderForeground(styles.Accent.GetForeground())

	l := list.New(nil, delegate, 0, 0)
	l.Title = "Series"
	l.SetShowStatusBar(false)
	l.SetFilteringEnabled(false)
	l.SetShowHelp(false)
	l.DisableQuitKeybindings()

	return Model{
		list:          l,
		keys:          DefaultKeyMap(),
		styles:        styles,
		client:        client,
		libraryID:     libraryID,
		seriesID:      seriesID,
		currentItemID: currentItemID,
		loading:       true,
	}
}

// Init starts loading the series.
func (m Model) Init() tea.Cmd {
	return m.fetchSeriesCmd()
}

// Update handles series screen messages.
func (m Model) Update(msg tea.Msg) (Model, tea.Cmd) {
	switch msg := msg.(type) {
	case LoadedMsg:
		m.loading = false
		m.err = msg.Err
		if msg.Err != nil {
			return m, nil
		}
		m.series = msg.Series
		m.list.Title = "Series"
		if msg.Series.Name != "" {
			m.list.Title = "Series — " + msg.Series.Name
		}
		items := make([]list.Item, len(msg.Series.Books))
		selected := 0
		for i, book := range msg.Series.Books {
			items[i] = seriesBookItem{book: book, current: book.LibraryItem.ID == m.currentItemID}
			if book.LibraryItem.ID == m.currentItemID {
				selected = i
			}
		}
		m.list.SetItems(items)
		if len(items) > 0 {
			m.list.Select(selected)
		}
		return m, nil
	case tea.KeyMsg:
		switch {
		case key.Matches(msg, m.keys.Back):
			return m, func() tea.Msg { return BackMsg{} }
		case key.Matches(msg, m.keys.Enter):
			if sel, ok := m.list.SelectedItem().(seriesBookItem); ok {
				return m, func() tea.Msg {
					return NavigateDetailMsg{Item: sel.book.LibraryItem}
				}
			}
		}
	}

	var cmd tea.Cmd
	m.list, cmd = m.list.Update(msg)
	return m, cmd
}

// View renders the series screen.
func (m Model) View() string {
	if m.err != nil {
		return m.styles.Error.Render(m.err.Error())
	}
	if m.loading {
		return m.styles.Muted.Render("Loading series…")
	}
	return m.list.View()
}

// SetSize updates the screen size.
func (m *Model) SetSize(width, height int) {
	m.width = width
	m.height = height
	m.list.SetSize(width, height)
}

// Loading reports whether the series is loading.
func (m Model) Loading() bool {
	return m.loading
}

// SelectedItemID returns the selected item's ID.
func (m Model) SelectedItemID() string {
	if sel, ok := m.list.SelectedItem().(seriesBookItem); ok {
		return sel.book.LibraryItem.ID
	}
	return ""
}

func (m Model) fetchSeriesCmd() tea.Cmd {
	if m.client == nil {
		return func() tea.Msg {
			return LoadedMsg{Err: fmt.Errorf("not authenticated")}
		}
	}
	client := m.client
	libraryID := m.libraryID
	seriesID := m.seriesID
	return func() tea.Msg {
		series, err := client.GetSeries(context.Background(), libraryID, seriesID)
		if err != nil {
			return LoadedMsg{Err: fmt.Errorf("fetch series: %w", err)}
		}
		if series == nil {
			return LoadedMsg{Err: fmt.Errorf("series not found")}
		}
		return LoadedMsg{Series: *series}
	}
}

type seriesBookItem struct {
	book    abs.SeriesBook
	current bool
}

func (i seriesBookItem) Title() string {
	title := i.book.LibraryItem.Media.Metadata.Title
	if i.current {
		return title + "  (current)"
	}
	return title
}

func (i seriesBookItem) Description() string {
	if i.book.Sequence == "" {
		return "Book"
	}
	return "Book #" + i.book.Sequence
}

func (i seriesBookItem) FilterValue() string {
	return i.book.LibraryItem.Media.Metadata.Title
}
