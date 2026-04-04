package search

import (
	"context"
	"fmt"
	"time"

	"github.com/Thelost77/pine/internal/abs"
	"github.com/Thelost77/pine/internal/ui"
	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/bubbles/list"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
)

const debounceDelay = 50 * time.Millisecond

// inputHeight is the number of lines reserved above the results body.
const inputHeight = 2

// SearchResultsMsg carries the results of a search query.
type SearchResultsMsg struct {
	Items []abs.LibraryItem
	Query string
	Err   error
}

// debounceTickMsg is sent after the debounce delay to trigger a search.
type debounceTickMsg struct {
	seq int
}

// NavigateDetailMsg requests navigation to the detail screen for an item.
type NavigateDetailMsg struct {
	Item abs.LibraryItem
}

// BackMsg signals that the user wants to leave the search screen.
type BackMsg struct{}

// KeyMap defines keybindings for the search screen.
type KeyMap struct {
	Enter key.Binding
	Back  key.Binding
}

// DefaultKeyMap returns the default keybindings for the search screen.
func DefaultKeyMap() KeyMap {
	return KeyMap{
		Enter: key.NewBinding(
			key.WithKeys("enter"),
			key.WithHelp("enter", "open detail"),
		),
		Back: key.NewBinding(
			key.WithKeys("esc", "left"),
			key.WithHelp("esc/←", "back"),
		),
	}
}

// Model is the bubbletea model for the search screen.
type Model struct {
	input            textinput.Model
	list             list.Model
	items            []abs.LibraryItem
	query            string
	debounceSeq      int
	loading          bool
	searched         bool
	err              error
	keys             KeyMap
	width            int
	height           int
	styles           ui.Styles
	cache            *Cache
	libraryID        string
	libraryMediaType string
}

// New creates a new search screen model.
func New(styles ui.Styles, cache *Cache, libraryID, libraryMediaType string) Model {
	ti := textinput.New()
	ti.Placeholder = searchPlaceholder(libraryMediaType)
	ti.CharLimit = 256
	ti.Focus()

	delegate := list.NewDefaultDelegate()
	delegate.Styles.SelectedTitle = delegate.Styles.SelectedTitle.
		Foreground(styles.Accent.GetForeground()).
		BorderForeground(styles.Accent.GetForeground())
	delegate.Styles.SelectedDesc = delegate.Styles.SelectedDesc.
		Foreground(styles.Muted.GetForeground()).
		BorderForeground(styles.Accent.GetForeground())

	l := list.New(nil, delegate, 0, 0)
	l.Title = "Results"
	l.SetShowTitle(false)
	l.SetShowStatusBar(false)
	l.SetFilteringEnabled(false)
	l.SetShowHelp(false)
	l.DisableQuitKeybindings()

	return Model{
		input:            ti,
		list:             l,
		keys:             DefaultKeyMap(),
		styles:           styles,
		cache:            cache,
		libraryID:        libraryID,
		libraryMediaType: libraryMediaType,
	}
}

// SetSize updates the terminal dimensions for the search screen.
func (m *Model) SetSize(width, height int) {
	m.width = width
	m.height = height
	listHeight := height - inputHeight
	if listHeight < 0 {
		listHeight = 0
	}
	m.list.SetSize(width, listHeight)
}

// Init returns the initial command (focus text input cursor).
func (m Model) Init() tea.Cmd {
	return textinput.Blink
}

// Update handles messages for the search screen.
func (m Model) Update(msg tea.Msg) (Model, tea.Cmd) {
	switch msg := msg.(type) {
	case SearchResultsMsg:
		if msg.Query != m.query {
			return m, nil
		}
		m.loading = false
		if msg.Err != nil {
			m.err = msg.Err
			return m, nil
		}
		m.err = nil
		m.searched = true
		m.items = msg.Items
		items := make([]list.Item, len(msg.Items))
		for i, item := range msg.Items {
			items[i] = ui.ListItem{Item: item}
		}
		m.list.SetItems(items)
		return m, nil

	case debounceTickMsg:
		if msg.seq != m.debounceSeq {
			return m, nil
		}
		if m.query == "" {
			return m, nil
		}
		return m, m.searchCmd(m.query)

	case tea.KeyMsg:
		switch {
		case key.Matches(msg, m.keys.Back):
			return m, func() tea.Msg { return BackMsg{} }
		case msg.Type == tea.KeyEnter:
			if sel, ok := m.list.SelectedItem().(ui.ListItem); ok {
				return m, func() tea.Msg {
					return NavigateDetailMsg{Item: sel.Item}
				}
			}
			return m, nil

		case msg.Type == tea.KeyUp || msg.Type == tea.KeyDown:
			var listCmd tea.Cmd
			m.list, listCmd = m.list.Update(msg)
			return m, listCmd
		}
	}

	// Forward remaining messages to text input
	prevQuery := m.input.Value()
	var inputCmd tea.Cmd
	m.input, inputCmd = m.input.Update(msg)
	newQuery := m.input.Value()

	var cmds []tea.Cmd
	if inputCmd != nil {
		cmds = append(cmds, inputCmd)
	}

	if newQuery != prevQuery {
		m.query = newQuery
		if normalizeQuery(m.query) == "" {
			m.items = nil
			m.list.SetItems(nil)
			m.loading = false
			m.searched = false
			m.err = nil
		} else {
			m.loading = true
			m.err = nil
			m.debounceSeq++
			seq := m.debounceSeq
			cmds = append(cmds, tea.Tick(debounceDelay, func(t time.Time) tea.Msg {
				return debounceTickMsg{seq: seq}
			}))
		}
	}

	if len(cmds) == 0 {
		return m, nil
	}
	return m, tea.Batch(cmds...)
}

// searchCmd creates a command that performs a library-local search.
func (m *Model) searchCmd(query string) tea.Cmd {
	if m.cache == nil {
		return func() tea.Msg {
			return SearchResultsMsg{Query: query, Err: fmt.Errorf("not authenticated")}
		}
	}
	m.loading = true
	cache := m.cache
	libID := m.libraryID
	libMediaType := m.libraryMediaType
	return func() tea.Msg {
		items, err := cache.Search(context.Background(), libID, libMediaType, query)
		if err != nil {
			return SearchResultsMsg{Query: query, Err: fmt.Errorf("search: %w", err)}
		}
		return SearchResultsMsg{Query: query, Items: items}
	}
}

// Query returns the current search query.
func (m Model) Query() string { return m.query }

// Items returns the current search results.
func (m Model) Items() []abs.LibraryItem { return m.items }

// Loading returns whether a search is in progress.
func (m Model) Loading() bool { return m.loading }

// Error returns the last error, if any.
func (m Model) Error() error { return m.err }

// Searched returns whether at least one search has completed.
func (m Model) Searched() bool { return m.searched }

func searchPlaceholder(libraryMediaType string) string {
	if libraryMediaType == "podcast" {
		return "Search episodes…"
	}
	return "Search audiobooks…"
}
