package serieslist

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

const pageLimit = 50

// thresholdPercent is the fraction of loaded items at which a prefetch fires.
const thresholdPercent = 0.8

// LoadedMsg carries a loaded page of series.
type LoadedMsg struct {
	Results []abs.Series
	Total   int
	Page    int
	Err     error
}

// NavigateSeriesMsg requests opening a specific series detail view.
type NavigateSeriesMsg struct {
	LibraryID string
	SeriesID  string
}

// BackMsg signals leaving the series browser screen.
type BackMsg struct{}

// KeyMap defines series browser bindings.
type KeyMap struct {
	Enter key.Binding
	Back  key.Binding
}

// DefaultKeyMap returns default series browser bindings.
func DefaultKeyMap() KeyMap {
	return KeyMap{
		Enter: key.NewBinding(
			key.WithKeys("enter"),
			key.WithHelp("enter", "open series"),
		),
		Back: key.NewBinding(
			key.WithKeys("esc", "left"),
			key.WithHelp("esc/left", "back"),
		),
	}
}

// Model is the Bubble Tea model for the all-series screen.
type Model struct {
	list        list.Model
	keys        KeyMap
	width       int
	height      int
	styles      ui.Styles
	client      *abs.Client
	libraryID   string
	libraryName string
	series      []abs.Series
	page        int
	total       int
	loading     bool
	err         error
}

// New creates a series browser model for a specific library.
func New(styles ui.Styles, client *abs.Client, libraryID, libraryName string) Model {
	delegate := list.NewDefaultDelegate()
	delegate.Styles.SelectedTitle = delegate.Styles.SelectedTitle.
		Foreground(styles.Accent.GetForeground()).
		BorderForeground(styles.Accent.GetForeground())
	delegate.Styles.SelectedDesc = delegate.Styles.SelectedDesc.
		Foreground(styles.Muted.GetForeground()).
		BorderForeground(styles.Accent.GetForeground())

	l := list.New(nil, delegate, 0, 0)
	l.Title = "Series"
	if libraryName != "" {
		l.Title = "Series — " + libraryName
	}
	l.SetShowStatusBar(true)
	l.SetFilteringEnabled(false)
	l.SetShowHelp(false)
	l.DisableQuitKeybindings()

	return Model{
		list:        l,
		keys:        DefaultKeyMap(),
		styles:      styles,
		client:      client,
		libraryID:   libraryID,
		libraryName: libraryName,
		loading:     true,
	}
}

// Init starts loading the first series page.
func (m Model) Init() tea.Cmd {
	return m.fetchSeriesCmd(0)
}

// Update handles messages for the series browser.
func (m Model) Update(msg tea.Msg) (Model, tea.Cmd) {
	switch msg := msg.(type) {
	case LoadedMsg:
		m.loading = false
		if msg.Err != nil {
			m.err = msg.Err
			return m, nil
		}
		m.total = msg.Total
		m.page = msg.Page
		if msg.Page == 0 {
			m.series = append([]abs.Series(nil), msg.Results...)
		} else {
			m.series = append(m.series, msg.Results...)
		}
		m.syncListItems()
		return m, nil

	case tea.KeyMsg:
		switch {
		case key.Matches(msg, m.keys.Back):
			return m, func() tea.Msg { return BackMsg{} }
		case key.Matches(msg, m.keys.Enter):
			if sel, ok := m.list.SelectedItem().(seriesListItem); ok {
				return m, func() tea.Msg {
					return NavigateSeriesMsg{LibraryID: m.libraryID, SeriesID: sel.series.ID}
				}
			}
		}
	}

	var cmd tea.Cmd
	m.list, cmd = m.list.Update(msg)
	return m, tea.Batch(cmd, m.maybePrefetch())
}

// View renders the series browser.
func (m Model) View() string {
	if m.loading && len(m.series) == 0 {
		return m.styles.Muted.Render("Loading series…")
	}
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

// Loading reports whether the screen is currently loading data.
func (m Model) Loading() bool {
	return m.loading
}

func (m *Model) maybePrefetch() tea.Cmd {
	if m.loading {
		return nil
	}
	loaded := len(m.series)
	if loaded == 0 || loaded >= m.total {
		return nil
	}
	threshold := int(float64(loaded) * thresholdPercent)
	if m.list.Index() >= threshold {
		return m.fetchSeriesCmd(m.page + 1)
	}
	return nil
}

func (m *Model) fetchSeriesCmd(page int) tea.Cmd {
	if m.client == nil {
		return func() tea.Msg {
			return LoadedMsg{Err: fmt.Errorf("not authenticated")}
		}
	}
	m.loading = true
	client := m.client
	libraryID := m.libraryID
	return func() tea.Msg {
		resp, err := client.GetLibrarySeries(context.Background(), libraryID, page, pageLimit)
		if err != nil {
			return LoadedMsg{Err: fmt.Errorf("fetch series: %w", err)}
		}
		if resp == nil {
			return LoadedMsg{}
		}
		return LoadedMsg{Results: resp.Results, Total: resp.Total, Page: resp.Page}
	}
}

func (m *Model) syncListItems() {
	items := make([]list.Item, len(m.series))
	for i, series := range m.series {
		items[i] = seriesListItem{series: series}
	}
	m.list.SetItems(items)
}

type seriesListItem struct {
	series abs.Series
}

func (i seriesListItem) Title() string {
	return i.series.Name
}

func (i seriesListItem) Description() string {
	if strings.TrimSpace(i.series.Description) != "" {
		return i.series.Description
	}
	return "Series"
}

func (i seriesListItem) FilterValue() string {
	return i.series.Name
}
