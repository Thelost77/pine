package sessions

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/Thelost77/pine/internal/abs"
	"github.com/Thelost77/pine/internal/cache"
	"github.com/Thelost77/pine/internal/ui"
	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/bubbles/list"
	tea "github.com/charmbracelet/bubbletea"
)

// pageSize is the number of sessions that the screen loads. The server gives
// the newest sessions first.
const pageSize = 50

// Target is the book or the podcast episode whose sessions the screen shows.
// The zero Target shows the sessions of all items.
type Target struct {
	ItemID    string
	EpisodeID string
	Title     string
}

// LoadedMsg carries the sessions of a target.
type LoadedMsg struct {
	Target   Target
	Sessions []abs.ListeningSession
	Total    int
	Err      error
}

// PlayFromMsg requests playback of an item or an episode from a position.
type PlayFromMsg struct {
	ItemID    string
	EpisodeID string
	Position  float64
}

// BackMsg signals leaving the sessions screen.
type BackMsg struct{}

// KeyMap defines sessions screen bindings.
type KeyMap struct {
	Enter    key.Binding
	Back     key.Binding
	PageUp   key.Binding
	PageDown key.Binding
}

// DefaultKeyMap returns default sessions bindings.
func DefaultKeyMap() KeyMap {
	return KeyMap{
		Enter: key.NewBinding(
			key.WithKeys("enter"),
			key.WithHelp("enter", "play from here"),
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

// Model is the Bubble Tea model for the sessions screen.
type Model struct {
	list     list.Model
	keys     KeyMap
	width    int
	height   int
	styles   ui.Styles
	client   *cache.Client
	target   Target
	deviceID string
	loading  bool
	err      error
}

// New creates a sessions screen model. deviceID is the device ID of this pine,
// which marks the sessions of this device.
func New(styles ui.Styles, client *cache.Client, target Target, deviceID string) Model {
	delegate := newDelegate(styles, false)

	l := list.New(nil, delegate, 0, 0)
	l.Title = title(target)
	l.SetShowStatusBar(false)
	l.SetStatusBarItemName("session", "sessions")
	l.SetFilteringEnabled(true)
	l.SetShowHelp(false)
	l.DisableQuitKeybindings()
	l.SetItems(buildSkeletonRows(styles))

	return Model{
		list:     l,
		keys:     DefaultKeyMap(),
		styles:   styles,
		client:   client,
		target:   target,
		deviceID: deviceID,
		loading:  true,
	}
}

// Init starts loading the sessions.
func (m Model) Init() tea.Cmd {
	return m.fetchSessionsCmd()
}

// Update handles sessions screen messages.
func (m Model) Update(msg tea.Msg) (Model, tea.Cmd) {
	switch msg := msg.(type) {
	case LoadedMsg:
		if msg.Target != m.target {
			return m, nil
		}
		m.loading = false
		m.err = msg.Err
		if msg.Err != nil {
			return m, nil
		}
		m.list.SetDelegate(newDelegate(m.styles, true))
		allItems := m.target.ItemID == ""
		items := make([]list.Item, len(msg.Sessions))
		for i, session := range msg.Sessions {
			items[i] = sessionItem{
				session:  session,
				allItems: allItems,
				thisPine: m.deviceID != "" && session.DeviceInfo.DeviceID == m.deviceID,
			}
		}
		m.list.SetItems(items)
		m.list.Title = title(m.target) + count(len(msg.Sessions), msg.Total)
		return m, nil
	case tea.KeyMsg:
		if m.list.FilterState() == list.Filtering {
			break
		}
		switch {
		case key.Matches(msg, m.keys.Back):
			if m.HasActiveFilter() {
				m.list.ResetFilter()
				return m, nil
			}
			return m, func() tea.Msg { return BackMsg{} }
		case key.Matches(msg, m.keys.Enter):
			if sel, ok := m.list.SelectedItem().(sessionItem); ok {
				s := sel.session
				return m, func() tea.Msg {
					return PlayFromMsg{ItemID: s.LibraryItemID, EpisodeID: s.EpisodeID, Position: s.CurrentTime}
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

// View renders the sessions screen.
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

// IsFiltering reports whether the filter input is active.
func (m Model) IsFiltering() bool {
	return m.list.FilterState() == list.Filtering
}

// HasActiveFilter reports whether a filter is typed or applied.
func (m Model) HasActiveFilter() bool {
	return m.list.FilterValue() != "" || m.list.FilterState() == list.FilterApplied
}

func (m Model) fetchSessionsCmd() tea.Cmd {
	target := m.target
	if m.client == nil {
		return func() tea.Msg {
			return LoadedMsg{Target: target, Err: fmt.Errorf("not authenticated")}
		}
	}
	client := m.client
	return func() tea.Msg {
		var resp *abs.ListeningSessionsResponse
		var err error
		if target.ItemID == "" {
			resp, err = client.GetListeningSessions(context.Background(), pageSize)
		} else {
			resp, err = client.GetItemListeningSessions(context.Background(), target.ItemID, target.EpisodeID, pageSize)
		}
		if err != nil {
			return LoadedMsg{Target: target, Err: fmt.Errorf("fetch listening sessions: %w", err)}
		}
		return LoadedMsg{Target: target, Sessions: resp.Sessions, Total: resp.Total}
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

func title(target Target) string {
	if target.ItemID == "" {
		return "Listening History"
	}
	if target.Title == "" {
		return "Listening Sessions"
	}
	return "Listening Sessions — " + target.Title
}

// count gives the number of sessions for the title, for example "(8)", or
// "(50 of 955)" when the server has more sessions than the screen loaded.
func count(loaded, total int) string {
	if total > loaded {
		return fmt.Sprintf("  (%d of %d)", loaded, total)
	}
	return fmt.Sprintf("  (%d)", loaded)
}

func buildSkeletonRows(styles ui.Styles) []list.Item {
	placeholder := func(width int) string {
		return styles.Muted.Render(strings.Repeat("-", width))
	}

	return []list.Item{
		skeletonItem{title: placeholder(34), description: placeholder(28)},
		skeletonItem{title: placeholder(30), description: placeholder(26)},
		skeletonItem{title: placeholder(36), description: placeholder(28)},
		skeletonItem{title: placeholder(32), description: placeholder(24)},
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

// sessionItem is one row. In the sessions of one item, the row starts with
// the positions. In the sessions of all items, it starts with the title.
type sessionItem struct {
	session  abs.ListeningSession
	allItems bool
	thisPine bool
}

func (i sessionItem) Title() string {
	s := i.session
	if i.allItems {
		return itemTitle(s)
	}
	return ui.FormatTimestamp(s.CurrentTime) + "  from " + ui.FormatTimestamp(s.StartTime) + " · " + i.device()
}

func (i sessionItem) Description() string {
	s := i.session
	if i.allItems {
		return ui.FormatTimestamp(s.CurrentTime) + " · " + updated(s) + " · " + i.device()
	}
	parts := []string{updated(s)}
	if client := strings.TrimSpace(s.DeviceInfo.ClientName + " " + s.DeviceInfo.ClientVersion); client != "" {
		parts = append(parts, client)
	}
	parts = append(parts, "listened "+listened(s.TimeListening))
	return strings.Join(parts, " · ")
}

func (i sessionItem) FilterValue() string {
	s := i.session
	return strings.Join([]string{itemTitle(s), i.device(), s.DeviceInfo.ClientName}, " ")
}

// device gives the name of the device of the session. "(this device)" marks a
// session of this pine.
func (i sessionItem) device() string {
	d := i.session.DeviceInfo
	name := "unknown device"
	switch {
	case d.DeviceName != "":
		name = d.DeviceName
	case d.Model != "":
		name = d.Model
	case d.ClientName != "":
		name = d.ClientName
	}
	if i.thisPine {
		name += " (this device)"
	}
	return name
}

// itemTitle gives "podcast — episode" for an episode, and "title — author" for
// a book.
func itemTitle(s abs.ListeningSession) string {
	first, second := s.DisplayTitle, s.DisplayAuthor
	if s.MediaType == "podcast" {
		first, second = s.MediaMetadata.Title, s.DisplayTitle
	}
	switch {
	case first == "":
		return second
	case second == "":
		return first
	}
	return first + " — " + second
}

// updated gives the local time of the last sync of the session.
func updated(s abs.ListeningSession) string {
	return time.UnixMilli(s.UpdatedAt).Local().Format("2006-01-02 15:04")
}

// listened gives the listening time in seconds below one minute. A short
// session is often a jump of the position, so it must not show as "0m".
func listened(seconds float64) string {
	if seconds < 60 {
		return fmt.Sprintf("%ds", int(seconds))
	}
	return ui.FormatDuration(seconds)
}

type skeletonItem struct {
	title       string
	description string
}

func (i skeletonItem) Title() string {
	return i.title
}

func (i skeletonItem) Description() string {
	return i.description
}

func (i skeletonItem) FilterValue() string {
	return ""
}
