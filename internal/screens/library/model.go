package library

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/Thelost77/pine/internal/abs"
	"github.com/Thelost77/pine/internal/ui"
	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/bubbles/list"
	tea "github.com/charmbracelet/bubbletea"
)

const pageLimit = 50

// thresholdPercent is the fraction of loaded items at which a prefetch fires.
const thresholdPercent = 0.8

const loadingRevealDelay = 150 * time.Millisecond

type rowKind int

const (
	rowKindItem rowKind = iota
	rowKindSkeleton
)

type loadingRevealMsg struct {
	generation uint64
}

// FetchLibraryItemsMsg is the command payload to request a page of items.
type FetchLibraryItemsMsg struct {
	Page  int
	Limit int
}

// LibraryItemsMsg carries the result of fetching library items.
type LibraryItemsMsg struct {
	Items     []abs.LibraryItem
	Total     int
	Page      int
	LibraryID string
	Err       error
}

// GoBackMsg requests navigating back from the library screen.
type GoBackMsg struct{}

// NavigateDetailMsg requests navigation to the detail screen for an item.
type NavigateDetailMsg struct {
	Item abs.LibraryItem
}

// NavigateSearchMsg requests navigation to the search screen for the current library.
type NavigateSearchMsg struct {
	LibraryID        string
	LibraryMediaType string
}

// NavigateSeriesListMsg requests navigation to the current library's series browser.
type NavigateSeriesListMsg struct {
	LibraryID   string
	LibraryName string
}

// KeyMap defines keybindings for the library screen.
type KeyMap struct {
	Enter    key.Binding
	Back     key.Binding
	NextLib  key.Binding
	PageUp   key.Binding
	PageDown key.Binding
	Search   key.Binding
	Series   key.Binding
	Select   key.Binding
}

// DefaultKeyMap returns the default keybindings for the library screen.
func DefaultKeyMap() KeyMap {
	return KeyMap{
		Enter: key.NewBinding(
			key.WithKeys("enter"),
			key.WithHelp("enter", "open detail"),
		),
		Back: key.NewBinding(
			key.WithKeys("left"),
			key.WithHelp("←", "back"),
		),
		NextLib: key.NewBinding(
			key.WithKeys("tab"),
			key.WithHelp("tab", "next library"),
		),
		PageUp: key.NewBinding(
			key.WithKeys("H"),
			key.WithHelp("H", "page up"),
		),
		PageDown: key.NewBinding(
			key.WithKeys("L"),
			key.WithHelp("L", "page down"),
		),
		Search: key.NewBinding(
			key.WithKeys("/"),
			key.WithHelp("/", "search"),
		),
		Series: key.NewBinding(
			key.WithKeys("s"),
			key.WithHelp("s", "series"),
		),
		Select: key.NewBinding(
			key.WithKeys("right"),
			key.WithHelp("→", "open detail"),
		),
	}
}

// Model is the bubbletea model for the library screen.
type Model struct {
	list            list.Model
	items           []abs.LibraryItem
	contentLibrary  string
	page            int
	totalItems      int
	cache           map[string]libraryCacheEntry
	loading         bool
	loadingVisible  bool
	loadingGen      uint64
	err             error
	keys            KeyMap
	width           int
	height          int
	styles          ui.Styles
	client          *abs.Client
	libraryID       string
	libraries       []abs.Library
	selectedLibrary int
}

type libraryCacheEntry struct {
	items      []abs.LibraryItem
	page       int
	totalItems int
}

// New creates a new library screen model.
func New(styles ui.Styles, client *abs.Client, libraryID string, libraries []abs.Library) Model {
	delegate := list.NewDefaultDelegate()
	delegate.Styles.SelectedTitle = delegate.Styles.SelectedTitle.
		Foreground(styles.Accent.GetForeground()).
		BorderForeground(styles.Accent.GetForeground())
	delegate.Styles.SelectedDesc = delegate.Styles.SelectedDesc.
		Foreground(styles.Muted.GetForeground()).
		BorderForeground(styles.Accent.GetForeground())

	l := list.New(nil, delegate, 0, 0)
	l.Title = "Library"
	l.SetShowStatusBar(true)
	l.SetFilteringEnabled(false)
	l.SetShowHelp(false)
	l.DisableQuitKeybindings()
	l.SetItems(buildSkeletonRows(styles))

	// Find the selected library index
	selectedIdx := 0
	for i, lib := range libraries {
		if lib.ID == libraryID {
			selectedIdx = i
			break
		}
	}

	libName := ""
	if selectedIdx < len(libraries) {
		libName = libraries[selectedIdx].Name
	}
	if libName != "" && len(libraries) > 1 {
		l.Title = fmt.Sprintf("Library — %s (tab to switch)", libName)
	}

	return Model{
		list:            l,
		loading:         true,
		keys:            DefaultKeyMap(),
		styles:          styles,
		client:          client,
		libraryID:       libraryID,
		libraries:       libraries,
		cache:           make(map[string]libraryCacheEntry),
		selectedLibrary: selectedIdx,
	}
}

// Configure updates the active library context while preserving cached pages.
func (m *Model) Configure(libraryID string, libraries []abs.Library) {
	m.libraries = libraries

	selectedIdx := 0
	for i, lib := range libraries {
		if lib.ID == libraryID {
			selectedIdx = i
			break
		}
	}

	m.selectedLibrary = selectedIdx
	if libraryID == "" && len(libraries) > 0 {
		libraryID = libraries[selectedIdx].ID
	}
	m.libraryID = libraryID
	m.err = nil
	m.updateListTitle()

	if m.applyCachedLibrary(m.libraryID) {
		return
	}

	m.page = 0
	m.totalItems = 0
	m.loading = true
	m.loadingVisible = false
	m.loadingGen++
	m.refreshListItems()
}

// SetSize updates the terminal dimensions for the library screen.
func (m *Model) SetSize(width, height int) {
	m.width = width
	m.height = height
	m.list.SetSize(width, height)
}

// Init returns the initial command that fetches the first page of library items.
func (m Model) Init() tea.Cmd {
	if m.libraryID != "" {
		if _, ok := m.cache[m.libraryID]; ok {
			return nil
		}
	}
	if m.contentLibrary != "" && m.contentLibrary == m.libraryID {
		return nil
	}
	m.page = 0
	return tea.Batch(m.fetchLibraryItemsCmd(0, pageLimit), m.loadingRevealCmd())
}

// Update handles messages for the library screen.
func (m Model) Update(msg tea.Msg) (Model, tea.Cmd) {
	switch msg := msg.(type) {
	case LibraryItemsMsg:
		m.loading = false
		m.loadingVisible = false
		if msg.LibraryID != "" && m.libraryID != "" && msg.LibraryID != m.libraryID {
			return m, nil
		}
		if msg.Err != nil {
			m.err = msg.Err
			return m, nil
		}
		m.totalItems = msg.Total
		m.page = msg.Page

		if msg.Page == 0 {
			m.items = append([]abs.LibraryItem(nil), msg.Items...)
			m.contentLibrary = msg.LibraryID
		} else {
			m.items = append(m.items, msg.Items...)
		}
		m.storeCurrentLibraryCache()
		m.refreshListItems()
		return m, nil

	case loadingRevealMsg:
		if msg.generation != m.loadingGen || !m.loading {
			return m, nil
		}
		m.loadingVisible = true
		m.refreshListItems()
		return m, nil

	case tea.KeyMsg:
		if m.list.FilterState() == list.Filtering {
			break
		}

		switch {
		case key.Matches(msg, m.keys.Enter), key.Matches(msg, m.keys.Select):
			if sel, ok := m.selectedItem(); ok {
				return m, func() tea.Msg {
					return NavigateDetailMsg{Item: sel.Item}
				}
			}
		case key.Matches(msg, m.keys.Back):
			return m, func() tea.Msg { return GoBackMsg{} }
		case key.Matches(msg, m.keys.Search):
			libID := m.libraryID
			libMediaType := m.SelectedLibraryMediaType()
			return m, func() tea.Msg {
				return NavigateSearchMsg{LibraryID: libID, LibraryMediaType: libMediaType}
			}
		case key.Matches(msg, m.keys.Series):
			if m.SelectedLibraryMediaType() == "book" {
				libID := m.libraryID
				libName := m.selectedLibraryName()
				return m, func() tea.Msg {
					return NavigateSeriesListMsg{LibraryID: libID, LibraryName: libName}
				}
			}
			return m, nil
		case key.Matches(msg, m.keys.PageDown):
			m.pageDown()
			return m, nil
		case key.Matches(msg, m.keys.PageUp):
			m.pageUp()
			return m, nil
		case key.Matches(msg, m.keys.NextLib):
			if len(m.libraries) > 1 {
				m.storeCurrentLibraryCache()
				m.selectedLibrary = (m.selectedLibrary + 1) % len(m.libraries)
				m.libraryID = m.libraries[m.selectedLibrary].ID
				m.updateListTitle()
				m.err = nil
				if m.applyCachedLibrary(m.libraryID) {
					return m, nil
				}
				m.page = 0
				m.totalItems = 0
				m.loading = true
				m.loadingVisible = false
				m.loadingGen++
				m.refreshListItems()
				return m, tea.Batch(m.fetchLibraryItemsCmd(0, pageLimit), m.loadingRevealCmd())
			}
			return m, nil
		}
	}

	var cmd tea.Cmd
	m.list, cmd = m.list.Update(msg)

	// After list update, check if cursor is near the end for infinite scroll
	prefetchCmd := m.maybePrefetch()
	return m, tea.Batch(cmd, prefetchCmd)
}

// maybePrefetch checks if the cursor has reached 80% of loaded items and
// fires a fetch for the next page if more items are available.
func (m *Model) maybePrefetch() tea.Cmd {
	if m.loading {
		return nil
	}
	loaded := len(m.items)
	if loaded == 0 {
		return nil
	}
	// All items loaded already
	if loaded >= m.totalItems {
		return nil
	}

	cursor := m.list.Index()
	threshold := int(float64(loaded) * thresholdPercent)
	if cursor >= threshold {
		return m.fetchLibraryItemsCmd(m.page+1, pageLimit)
	}
	return nil
}

// fetchLibraryItemsCmd creates a command that fetches a page of library items.
func (m *Model) fetchLibraryItemsCmd(page, limit int) tea.Cmd {
	if m.client == nil {
		return func() tea.Msg {
			return LibraryItemsMsg{Err: fmt.Errorf("not authenticated")}
		}
	}
	m.loading = true
	client := m.client
	libID := m.libraryID
	libraryID := m.libraryID
	return func() tea.Msg {
		// Fallback: if no libraryID was provided, fetch the first library
		if libID == "" {
			libs, err := client.GetLibraries(context.Background())
			if err != nil {
				return LibraryItemsMsg{LibraryID: libraryID, Err: fmt.Errorf("fetch libraries: %w", err)}
			}
			libs, _ = client.FilterAudioLibraries(context.Background(), libs)
			if len(libs) == 0 {
				return LibraryItemsMsg{Items: nil, Total: 0, Page: 0, LibraryID: libraryID}
			}
			libID = libs[0].ID
		}

		resp, err := client.GetLibraryItems(context.Background(), libID, page, limit)
		if err != nil {
			return LibraryItemsMsg{LibraryID: libID, Err: fmt.Errorf("fetch library items: %w", err)}
		}

		return LibraryItemsMsg{
			Items:     resp.Results,
			Total:     resp.Total,
			Page:      resp.Page,
			LibraryID: libID,
		}
	}
}

func (m *Model) refreshListItems() {
	if m.shouldShowSkeletons() {
		m.list.SetItems(buildSkeletonRows(m.styles))
		return
	}

	items := make([]list.Item, len(m.items))
	for i, item := range m.items {
		items[i] = libraryListItem{kind: rowKindItem, Item: item}
	}
	m.list.SetItems(items)
}

func (m Model) loadingRevealCmd() tea.Cmd {
	generation := m.loadingGen
	return tea.Tick(loadingRevealDelay, func(time.Time) tea.Msg {
		return loadingRevealMsg{generation: generation}
	})
}

func (m Model) shouldShowSkeletons() bool {
	if !m.loading || !m.loadingVisible {
		return false
	}
	return m.contentLibrary != m.libraryID
}

func buildSkeletonRows(styles ui.Styles) []list.Item {
	placeholder := func(width int) string {
		return styles.Muted.Render(strings.Repeat("-", width))
	}

	return []list.Item{
		libraryListItem{kind: rowKindSkeleton, title: placeholder(22), description: placeholder(14) + " • " + placeholder(5)},
		libraryListItem{kind: rowKindSkeleton, title: placeholder(18), description: placeholder(12) + " • " + placeholder(6)},
		libraryListItem{kind: rowKindSkeleton, title: placeholder(24), description: placeholder(15) + " • " + placeholder(4)},
		libraryListItem{kind: rowKindSkeleton, title: placeholder(19), description: placeholder(13) + " • " + placeholder(5)},
		libraryListItem{kind: rowKindSkeleton, title: placeholder(21), description: placeholder(11) + " • " + placeholder(6)},
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

func (m Model) selectedItem() (libraryListItem, bool) {
	if m.loading && m.contentLibrary != "" && m.contentLibrary != m.libraryID {
		return libraryListItem{}, false
	}
	sel, ok := m.list.SelectedItem().(libraryListItem)
	if !ok || sel.kind != rowKindItem {
		return libraryListItem{}, false
	}
	return sel, true
}

func (m *Model) storeCurrentLibraryCache() {
	if m.libraryID == "" || m.contentLibrary != m.libraryID {
		return
	}
	m.cache[m.libraryID] = libraryCacheEntry{
		items:      append([]abs.LibraryItem(nil), m.items...),
		page:       m.page,
		totalItems: m.totalItems,
	}
}

func (m *Model) applyCachedLibrary(libraryID string) bool {
	entry, ok := m.cache[libraryID]
	if !ok {
		return false
	}
	m.items = append([]abs.LibraryItem(nil), entry.items...)
	m.page = entry.page
	m.totalItems = entry.totalItems
	m.loading = false
	m.loadingVisible = false
	m.contentLibrary = libraryID
	m.refreshListItems()
	return true
}

// Items returns the current library items.
func (m Model) Items() []abs.LibraryItem {
	return m.items
}

// SelectedLibraryMediaType returns the media type of the current library, or empty string.
func (m Model) SelectedLibraryMediaType() string {
	if len(m.libraries) > 0 && m.selectedLibrary < len(m.libraries) {
		return m.libraries[m.selectedLibrary].MediaType
	}
	if len(m.items) > 0 {
		return m.items[0].MediaType
	}
	return ""
}

func (m Model) selectedLibraryName() string {
	if len(m.libraries) > 0 && m.selectedLibrary < len(m.libraries) {
		return m.libraries[m.selectedLibrary].Name
	}
	return ""
}

// updateListTitle updates the list title to reflect the selected library.
func (m *Model) updateListTitle() {
	if len(m.libraries) > 1 && m.selectedLibrary < len(m.libraries) {
		m.list.Title = fmt.Sprintf("Library — %s (tab to switch)", m.libraries[m.selectedLibrary].Name)
	} else {
		m.list.Title = "Library"
	}
}

// Loading returns whether data is being fetched.
func (m Model) Loading() bool {
	return m.loading
}

// Error returns the last error, if any.
func (m Model) Error() error {
	return m.err
}

type libraryListItem struct {
	kind        rowKind
	Item        abs.LibraryItem
	title       string
	description string
}

func (i libraryListItem) Title() string {
	if i.kind != rowKindItem {
		return i.title
	}
	if i.Item.MediaType == "podcast" && i.Item.RecentEpisode != nil {
		return i.Item.RecentEpisode.Title
	}
	return i.Item.Media.Metadata.Title
}

func (i libraryListItem) Description() string {
	if i.kind != rowKindItem {
		return i.description
	}
	context := "Unknown author"
	if i.Item.MediaType == "podcast" && i.Item.RecentEpisode != nil {
		context = i.Item.Media.Metadata.Title
	} else if i.Item.Media.Metadata.AuthorName != nil {
		context = *i.Item.Media.Metadata.AuthorName
	}

	duration := ""
	if i.Item.MediaType == "podcast" && i.Item.RecentEpisode != nil && i.Item.RecentEpisode.Duration > 0 {
		duration = ui.FormatDuration(i.Item.RecentEpisode.Duration)
	} else if i.Item.Media.HasDuration() {
		duration = ui.FormatDuration(i.Item.Media.TotalDuration())
	}

	parts := []string{context}
	if duration != "" {
		parts = append(parts, duration)
	}
	return strings.Join(parts, " • ")
}

func (i libraryListItem) FilterValue() string {
	if i.kind != rowKindItem {
		return ""
	}
	return i.Item.Media.Metadata.Title
}

// Page returns the current page number.
func (m Model) Page() int {
	return m.page
}

// TotalItems returns the total number of items available.
func (m Model) TotalItems() int {
	return m.totalItems
}
