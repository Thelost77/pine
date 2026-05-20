package series

import (
	"context"
	"fmt"
	"strings"

	"github.com/Thelost77/pine/internal/abs"
	"github.com/Thelost77/pine/internal/ui"
	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/bubbles/list"
	tea "github.com/charmbracelet/bubbletea"
)

// LoadedMsg carries a loaded series payload.
type LoadedMsg struct {
	Contents abs.SeriesContents
	Err      error
}

// NavigateDetailMsg requests opening a book from the series list.
type NavigateDetailMsg struct {
	Item abs.LibraryItem
}

// BackMsg signals leaving the series screen.
type BackMsg struct{}

// KeyMap defines series screen bindings.
type KeyMap struct {
	Enter    key.Binding
	Back     key.Binding
	PageUp   key.Binding
	PageDown key.Binding
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
		PageUp: key.NewBinding(
			key.WithKeys("H"),
			key.WithHelp("H", "page up"),
		),
		PageDown: key.NewBinding(
			key.WithKeys("L"),
			key.WithHelp("L", "page down"),
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
	series        abs.SeriesContents
	loading       bool
	err           error
}

// New creates a series screen model.
func New(styles ui.Styles, client *abs.Client, libraryID, seriesID, currentItemID string) Model {
	delegate := newDelegate(styles, false)

	l := list.New(nil, delegate, 0, 0)
	l.Title = "Series"
	l.SetShowStatusBar(false)
	l.SetFilteringEnabled(false)
	l.SetShowHelp(false)
	l.DisableQuitKeybindings()
	l.SetItems(buildSkeletonRows(styles))

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
		m.list.SetDelegate(newDelegate(m.styles, true))
		m.series = msg.Contents
		m.list.Title = "Series"
		if msg.Contents.Series.Name != "" {
			m.list.Title = "Series — " + msg.Contents.Series.Name
		}
		items := make([]list.Item, len(msg.Contents.Items))
		selected := 0
		for i, item := range msg.Contents.Items {
			items[i] = seriesBookItem{item: item, current: item.ID == m.currentItemID}
			if item.ID == m.currentItemID {
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
					return NavigateDetailMsg{Item: sel.item}
				}
			}
		case key.Matches(msg, m.keys.PageDown):
			m.pageDown()
			return m, nil
		case key.Matches(msg, m.keys.PageUp):
			m.pageUp()
			return m, nil
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
		return sel.item.ID
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
		contents, err := client.GetSeriesContents(context.Background(), libraryID, seriesID)
		if err != nil {
			return LoadedMsg{Err: fmt.Errorf("fetch series: %w", err)}
		}
		if contents == nil {
			return LoadedMsg{Err: fmt.Errorf("series not found")}
		}
		return LoadedMsg{Contents: *contents}
	}
}

func (m *Model) pageDown() {
	before := m.list.GlobalIndex()
	m.list.NextPage()
	if m.list.GlobalIndex() == before {
		m.list.GoToEnd()
	}
}

func (m *Model) pageUp() {
	before := m.list.GlobalIndex()
	m.list.PrevPage()
	if m.list.GlobalIndex() == before {
		m.list.GoToStart()
	}
}

func buildSkeletonRows(styles ui.Styles) []list.Item {
	placeholder := func(width int) string {
		return styles.Muted.Render(strings.Repeat("-", width))
	}

	return []list.Item{
		seriesSkeletonItem{title: placeholder(22), description: placeholder(7)},
		seriesSkeletonItem{title: placeholder(18), description: placeholder(7)},
		seriesSkeletonItem{title: placeholder(24), description: placeholder(7)},
		seriesSkeletonItem{title: placeholder(20), description: placeholder(7)},
		seriesSkeletonItem{title: placeholder(19), description: placeholder(7)},
	}
}

func newDelegate(styles ui.Styles, highlightSelected bool) list.DefaultDelegate {
	delegate := list.NewDefaultDelegate()
	if highlightSelected {
		delegate.Styles.SelectedTitle = delegate.Styles.SelectedTitle.
			Foreground(styles.Accent.GetForeground()).
			BorderForeground(styles.Accent.GetForeground())
		delegate.Styles.SelectedDesc = delegate.Styles.SelectedDesc.
			Foreground(styles.Muted.GetForeground()).
			BorderForeground(styles.Accent.GetForeground())
		return delegate
	}

	delegate.Styles.SelectedTitle = delegate.Styles.NormalTitle
	delegate.Styles.SelectedDesc = delegate.Styles.NormalDesc
	return delegate
}

type seriesBookItem struct {
	item    abs.LibraryItem
	current bool
}

func (i seriesBookItem) Title() string {
	title := i.item.Media.Metadata.Title
	if i.current {
		return title + "  (current)"
	}
	return title
}

func (i seriesBookItem) Description() string {
	if i.item.Media.Metadata.Series == nil || i.item.Media.Metadata.Series.Sequence == "" {
		return "Book"
	}
	return "Book #" + i.item.Media.Metadata.Series.Sequence
}

func (i seriesBookItem) FilterValue() string {
	return i.item.Media.Metadata.Title
}

type seriesSkeletonItem struct {
	title       string
	description string
}

func (i seriesSkeletonItem) Title() string {
	return i.title
}

func (i seriesSkeletonItem) Description() string {
	return i.description
}

func (i seriesSkeletonItem) FilterValue() string {
	return ""
}
