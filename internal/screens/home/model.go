package home

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

const continueListeningLimit = 5
const recentlyAddedLimit = 3

// PersonalizedMsg carries the result of fetching personalized data.
type PersonalizedMsg struct {
	Items         []abs.LibraryItem
	RecentlyAdded []abs.LibraryItem
	Libraries     []abs.Library
	Err           error
}

type rowKind int

const (
	rowKindItem rowKind = iota
	rowKindSection
	rowKindEmpty
)

// listItem wraps a home row for the bubbles list component.
type listItem struct {
	kind        rowKind
	item        abs.LibraryItem
	title       string
	description string
}

func (i listItem) Title() string {
	if i.kind != rowKindItem {
		return i.title
	}
	return itemTitle(i.item)
}

func (i listItem) Description() string {
	if i.kind != rowKindItem {
		return i.description
	}
	return itemDescription(i.item)
}

func (i listItem) FilterValue() string {
	if i.kind != rowKindItem {
		return ""
	}
	return i.item.Media.Metadata.Title
}

// KeyMap defines keybindings for the home screen.
type KeyMap struct {
	Enter      key.Binding
	Back       key.Binding
	Library    key.Binding
	Search     key.Binding
	NextLib    key.Binding
	Select     key.Binding
	AddToQueue key.Binding
	PlayNext   key.Binding
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

// Model is the bubbletea model for the home screen.
type Model struct {
	list            list.Model
	items           []abs.LibraryItem
	recentlyAdded   []abs.LibraryItem
	loading         bool
	err             error
	keys            KeyMap
	width           int
	height          int
	styles          ui.Styles
	client          *abs.Client
	libraries       []abs.Library
	selectedLibrary int
	itemCache       map[string][]abs.LibraryItem // libraryID → items
	recentCache     map[string][]abs.LibraryItem // libraryID → recently added
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
	l.SetShowStatusBar(false)
	l.SetFilteringEnabled(false)
	l.SetShowHelp(false)
	l.DisableQuitKeybindings()

	return Model{
		list:        l,
		loading:     true,
		keys:        DefaultKeyMap(),
		styles:      styles,
		client:      client,
		itemCache:   make(map[string][]abs.LibraryItem),
		recentCache: make(map[string][]abs.LibraryItem),
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
		m.items = limitItems(msg.Items, continueListeningLimit)
		m.recentlyAdded = dedupeRecentlyAdded(m.items, msg.RecentlyAdded, recentlyAddedLimit)
		if libID := m.SelectedLibraryID(); libID != "" {
			m.itemCache[libID] = m.items
			m.recentCache[libID] = m.recentlyAdded
		}
		m.setListItems(m.items)
		m.updateListTitle()
		return m, nil

	case tea.KeyMsg:
		// Don't intercept keys while filtering
		if m.list.FilterState() == list.Filtering {
			break
		}

		switch {
		case key.Matches(msg, m.keys.Enter), key.Matches(msg, m.keys.Select):
			if item, ok := m.selectedItem(); ok {
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
		case key.Matches(msg, m.keys.AddToQueue):
			if item, episode, ok := m.selectedQueueTarget(); ok {
				return m, func() tea.Msg {
					return AddToQueueMsg{Item: item, Episode: episode}
				}
			}
		case key.Matches(msg, m.keys.PlayNext):
			if item, episode, ok := m.selectedQueueTarget(); ok {
				return m, func() tea.Msg {
					return PlayNextMsg{Item: item, Episode: episode}
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
				// Cache current library's items
				if libID := m.SelectedLibraryID(); libID != "" {
					m.itemCache[libID] = m.items
					m.recentCache[libID] = m.recentlyAdded
				}
				m.selectedLibrary = (m.selectedLibrary + 1) % len(m.libraries)
				m.updateListTitle()
				// Use cached items if available, fetch in background either way
				newLibID := m.SelectedLibraryID()
				if cached, ok := m.itemCache[newLibID]; ok {
					m.items = cached
				} else {
					m.items = nil
				}
				if cachedRecent, ok := m.recentCache[newLibID]; ok {
					m.recentlyAdded = cachedRecent
				} else {
					m.recentlyAdded = nil
				}
				m.setListItems(m.items)
				return m, m.fetchPersonalizedCmd()
			}
			return m, nil
		case key.Matches(msg, m.keys.Back):
			return m, func() tea.Msg { return GoBackMsg{} }
		}
	}

	var cmd tea.Cmd
	m.list, cmd = m.list.Update(msg)
	if keyMsg, ok := msg.(tea.KeyMsg); ok {
		m.skipNonInteractiveSelection(selectionStep(keyMsg))
	}
	return m, cmd
}

// fetchPersonalizedCmd creates a command that fetches personalized shelves.
func (m *Model) fetchPersonalizedCmd() tea.Cmd {
	if m.client == nil {
		return func() tea.Msg {
			return PersonalizedMsg{Err: fmt.Errorf("not authenticated")}
		}
	}
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

		var continueListening []abs.LibraryItem
		var recentlyAdded []abs.LibraryItem
		for _, section := range sections {
			switch section.ID {
			case "continue-listening":
				continueListening = section.Entities
			case "recently-added":
				recentlyAdded = section.Entities
			}
		}
		recentlyAdded = hydrateRecentlyAddedPodcasts(context.Background(), client, recentlyAdded)

		return PersonalizedMsg{
			Items:         continueListening,
			RecentlyAdded: recentlyAdded,
			Libraries:     libs,
		}
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

// AddToQueueMsg requests appending the selected home item to the queue.
type AddToQueueMsg struct {
	Item    abs.LibraryItem
	Episode *abs.PodcastEpisode
}

// PlayNextMsg requests inserting the selected home item at the front of the queue.
type PlayNextMsg struct {
	Item    abs.LibraryItem
	Episode *abs.PodcastEpisode
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

// RecentlyAdded returns the secondary recently added subsection items.
func (m Model) RecentlyAdded() []abs.LibraryItem {
	return m.recentlyAdded
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

func (m *Model) setListItems(items []abs.LibraryItem) {
	m.list.SetItems(buildListRows(m.styles, items, m.recentlyAdded))
	m.skipNonInteractiveSelection(1)
}

func limitItems(items []abs.LibraryItem, limit int) []abs.LibraryItem {
	if len(items) <= limit {
		return append([]abs.LibraryItem(nil), items...)
	}
	return append([]abs.LibraryItem(nil), items[:limit]...)
}

func dedupeRecentlyAdded(primary, recent []abs.LibraryItem, limit int) []abs.LibraryItem {
	if len(recent) == 0 {
		return nil
	}

	seenTitles := make(map[string]struct{}, len(primary))
	for _, item := range primary {
		title := strings.TrimSpace(item.Media.Metadata.Title)
		if title == "" {
			continue
		}
		seenTitles[strings.ToLower(title)] = struct{}{}
	}

	result := make([]abs.LibraryItem, 0, limit)
	for _, item := range recent {
		title := strings.TrimSpace(item.Media.Metadata.Title)
		key := strings.ToLower(title)
		if title != "" {
			if _, exists := seenTitles[key]; exists {
				continue
			}
			seenTitles[key] = struct{}{}
		}
		result = append(result, item)
		if len(result) == limit {
			break
		}
	}
	return result
}

func buildListRows(styles ui.Styles, items, recent []abs.LibraryItem) []list.Item {
	rows := make([]list.Item, 0, len(items)+len(recent)+2)
	if len(items) == 0 {
		rows = append(rows, listItem{
			kind:  rowKindEmpty,
			title: styles.Muted.Render("No items in continue listening"),
		})
	} else {
		for _, item := range items {
			rows = append(rows, listItem{kind: rowKindItem, item: item})
		}
	}

	if len(recent) > 0 {
		rows = append(rows, listItem{
			kind:  rowKindSection,
			title: styles.Accent.Bold(true).Render("Recently Added"),
		})
		for _, item := range recent {
			rows = append(rows, listItem{kind: rowKindItem, item: item})
		}
	}

	return rows
}

func itemTitle(item abs.LibraryItem) string {
	if item.MediaType == "podcast" && item.RecentEpisode != nil {
		return item.RecentEpisode.Title
	}
	return item.Media.Metadata.Title
}

func itemDescription(item abs.LibraryItem) string {
	contextLabel := "Unknown author"
	if item.MediaType == "podcast" && item.RecentEpisode != nil {
		contextLabel = item.Media.Metadata.Title
	} else if item.Media.Metadata.AuthorName != nil {
		contextLabel = *item.Media.Metadata.AuthorName
	}

	progress := ""
	if item.UserMediaProgress != nil {
		progress = fmt.Sprintf(" • %d%%", int(item.UserMediaProgress.Progress*100))
	}

	duration := ""
	switch {
	case item.MediaType == "podcast" && item.RecentEpisode != nil && item.RecentEpisode.Duration > 0:
		duration = fmt.Sprintf(" • %s", ui.FormatDuration(item.RecentEpisode.Duration))
	case item.Media.HasDuration():
		duration = fmt.Sprintf(" • %s", ui.FormatDuration(item.Media.TotalDuration()))
	}

	return contextLabel + progress + duration
}

func (m Model) selectedItem() (abs.LibraryItem, bool) {
	sel, ok := m.list.SelectedItem().(listItem)
	if !ok || sel.kind != rowKindItem {
		return abs.LibraryItem{}, false
	}
	return sel.item, true
}

func (m Model) selectedQueueTarget() (abs.LibraryItem, *abs.PodcastEpisode, bool) {
	item, ok := m.selectedItem()
	if !ok {
		return abs.LibraryItem{}, nil, false
	}
	if item.MediaType == "podcast" && item.RecentEpisode != nil {
		return item, cloneEpisode(item.RecentEpisode), true
	}
	return item, nil, true
}

func (m *Model) skipNonInteractiveSelection(step int) {
	items := m.list.Items()
	if len(items) == 0 {
		return
	}

	idx := m.list.Index()
	if idx < 0 || idx >= len(items) {
		return
	}
	if row, ok := items[idx].(listItem); ok && row.kind == rowKindItem {
		return
	}

	if step == 0 {
		step = 1
	}
	for next := idx + step; next >= 0 && next < len(items); next += step {
		row, ok := items[next].(listItem)
		if !ok {
			continue
		}
		if row.kind == rowKindItem {
			m.list.Select(next)
			return
		}
	}
}

func selectionStep(msg tea.KeyMsg) int {
	switch msg.String() {
	case "j", "down":
		return 1
	case "k", "up":
		return -1
	default:
		return 0
	}
}

func hydrateRecentlyAddedPodcasts(ctx context.Context, client *abs.Client, items []abs.LibraryItem) []abs.LibraryItem {
	if client == nil || len(items) == 0 {
		return items
	}

	hydrated := make([]abs.LibraryItem, len(items))
	copy(hydrated, items)
	for i, item := range hydrated {
		if item.MediaType != "podcast" || item.RecentEpisode != nil {
			continue
		}
		fullItem, err := client.GetLibraryItem(ctx, item.ID)
		if err != nil || fullItem == nil {
			continue
		}
		if episode := latestEpisode(fullItem.Media.Episodes); episode != nil {
			item.RecentEpisode = episode
			hydrated[i] = item
		}
	}
	return hydrated
}

func latestEpisode(episodes []abs.PodcastEpisode) *abs.PodcastEpisode {
	if len(episodes) == 0 {
		return nil
	}

	best := episodes[0]
	for _, episode := range episodes[1:] {
		switch {
		case episode.AddedAt > best.AddedAt:
			best = episode
		case episode.AddedAt == best.AddedAt && episode.PublishedAt > best.PublishedAt:
			best = episode
		case episode.AddedAt == best.AddedAt && episode.PublishedAt == best.PublishedAt && episode.Index > best.Index:
			best = episode
		}
	}
	return cloneEpisode(&best)
}

func cloneEpisode(episode *abs.PodcastEpisode) *abs.PodcastEpisode {
	if episode == nil {
		return nil
	}
	cp := *episode
	return &cp
}

// Loading returns whether data is being fetched.
func (m Model) Loading() bool {
	return m.loading
}

// Error returns the last error, if any.
func (m Model) Error() error {
	return m.err
}
