package components

import (
	"fmt"
	"io"
	"strings"

	"github.com/Thelost77/pine/internal/ui"
	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/bubbles/list"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

const (
	paletteWidth        = 64
	maxItems            = 20
	paletteChromeHeight = 7 // border(2) + padding(2) + title(1) + input(1) + footer(1)
)

type PaletteAction int

const (
	ActionNone PaletteAction = iota
	ActionGoHome
	ActionGoLibrary
	ActionGoSeriesList
	ActionTogglePlay
	ActionNextChapter
	ActionPrevChapter
	ActionSeekForward
	ActionSeekBackward
	ActionSpeedUp
	ActionSpeedDown
	ActionSleep15
	ActionSleep30
	ActionSleep45
	ActionSleep60
	ActionSleepOff
	ActionShowQueue
	ActionClearQueue
	ActionOpenSelected
	ActionQueueItem
	ActionPlayNextItem
	ActionMarkFinished
	ActionAddBookmark
	ActionGoToSeries
	ActionBrowseSeries
	ActionSwitchLibrary
	ActionOpenDetail
	ActionContentNavigate
	ActionPlayDirect
)

type PaletteItem struct {
	Label     string
	Action    PaletteAction
	IsHeader  bool
	Payload   string
	LibraryID string
	ItemID    string
	Data      any
}

func (i PaletteItem) Title() string       { return i.Label }
func (i PaletteItem) Description() string { return "" }
func (i PaletteItem) FilterValue() string { return i.Label }

type SearchFunc func(query string) []PaletteItem

type Palette struct {
	visible    bool
	input      textinput.Model
	list       list.Model
	delegate   paletteDelegate
	styles     ui.Styles
	focusSearch bool

	staticItems []PaletteItem
	searchFn    SearchFunc

	lastAction    PaletteAction
	lastPayload   string
	lastLibraryID string
	lastItemID    string
	lastData      any

	width  int
	height int
}

type paletteDelegate struct {
	delegate list.DefaultDelegate
	styles   ui.Styles
}

func (d paletteDelegate) Height() int  { return 1 }
func (d paletteDelegate) Spacing() int { return 0 }

func (d paletteDelegate) Render(w io.Writer, l list.Model, index int, item list.Item) {
	pi, ok := item.(PaletteItem)
	if !ok {
		d.delegate.Render(w, l, index, item)
		return
	}

	itemWidth := l.Width() - 6
	label := pi.Label
	if lipgloss.Width(label) > itemWidth {
		label = ansiTruncate(label, itemWidth, "…")
	}

	if pi.IsHeader {
		style := d.styles.Accent.Bold(true)
		line := strings.Repeat("─", 4) + " " + style.Render(strings.ToUpper(label))
		_, _ = fmt.Fprint(w, line)
		return
	}

	isSelected := index == l.Index()
	if isSelected {
		_, _ = fmt.Fprint(w, d.delegate.Styles.SelectedTitle.Render("› "+label))
	} else {
		_, _ = fmt.Fprint(w, d.delegate.Styles.NormalTitle.Render("  "+label))
	}
}

func (d paletteDelegate) Update(msg tea.Msg, m *list.Model) tea.Cmd {
	return d.delegate.Update(msg, m)
}

func NewPalette() Palette {
	delegate := paletteDelegate{
		delegate: list.NewDefaultDelegate(),
	}

	ti := textinput.New()
	ti.Placeholder = "Type to filter…"
	ti.Prompt = "> "
	ti.CharLimit = 128
	ti.Width = paletteWidth - 4

	l := list.New(nil, delegate, paletteWidth, maxItems)
	l.SetShowTitle(false)
	l.SetShowStatusBar(false)
	l.SetFilteringEnabled(false)
	l.SetShowHelp(false)
	l.DisableQuitKeybindings()
	l.SetShowPagination(false)
	l.KeyMap.NextPage = key.NewBinding(key.WithKeys("ctrl+n"))
	l.KeyMap.PrevPage = key.NewBinding(key.WithKeys("ctrl+p"))
	l.Styles.NoItems = lipgloss.NewStyle()

	return Palette{
		list:        l,
		delegate:    delegate,
		input:       ti,
		focusSearch: false,
	}
}

func (p *Palette) SetStyles(styles ui.Styles) {
	p.styles = styles
	p.delegate.styles = styles
	p.delegate.delegate.Styles.SelectedTitle = styles.Selected.Foreground(lipgloss.Color("#d3c6aa")).Bold(true).Padding(0, 0)
	p.delegate.delegate.Styles.NormalTitle = styles.Muted.Padding(0, 0)
	p.list.SetDelegate(p.delegate)

	p.input.PromptStyle = styles.Accent
	p.input.TextStyle = styles.Accent
	p.input.PlaceholderStyle = styles.Muted
}

func (p *Palette) Open(staticItems []PaletteItem, searchFn SearchFunc) {
	p.visible = true
	p.focusSearch = false
	p.staticItems = staticItems
	p.searchFn = searchFn
	p.lastAction = ActionNone
	p.lastPayload = ""
	p.lastLibraryID = ""
	p.lastItemID = ""
	p.lastData = nil

	p.input.Reset()
	p.input.Blur()

	p.applyFilter("")
}

func (p *Palette) Close() {
	p.visible = false
}

func (p *Palette) Visible() bool {
	return p.visible
}

func (p *Palette) SelectedAction() (PaletteAction, string, string, string, any) {
	return p.lastAction, p.lastPayload, p.lastLibraryID, p.lastItemID, p.lastData
}

func (p *Palette) ClearSelection() {
	p.lastAction = ActionNone
	p.lastPayload = ""
	p.lastLibraryID = ""
	p.lastItemID = ""
	p.lastData = nil
}

func (p *Palette) SetSize(width, height int) {
	p.width = width
	p.height = height
	listHeight := max(1, height-2-paletteChromeHeight)
	if listHeight > maxItems {
		listHeight = maxItems
	}
	p.list.SetSize(paletteWidth, listHeight)
}

func (p *Palette) Update(msg tea.Msg) (tea.Cmd, bool) {
	if !p.visible {
		return nil, false
	}

	switch msg := msg.(type) {
	case tea.KeyMsg:
		if msg.Type == tea.KeyEsc {
			if p.focusSearch {
				p.focusSearch = false
				p.input.Blur()
				return nil, true
			}
			p.Close()
			return nil, true
		}

		if p.focusSearch {
			return p.updateSearchInput(msg)
		}
		return p.updateList(msg)
	}

	if p.focusSearch {
		var cmd tea.Cmd
		p.input, cmd = p.input.Update(msg)
		return cmd, true
	}
	var cmd tea.Cmd
	p.list, cmd = p.list.Update(msg)
	return cmd, true
}

func (p *Palette) updateSearchInput(msg tea.KeyMsg) (tea.Cmd, bool) {
	if msg.Type == tea.KeyEsc {
		p.focusSearch = false
		p.input.Blur()
		return nil, true
	}

	if msg.Type == tea.KeyEnter || msg.Type == tea.KeyTab {
		p.focusSearch = false
		p.input.Blur()
		p.list.ResetSelected()
		if len(p.list.Items()) > 0 {
			p.list.Select(p.firstActionIndex())
		}
		return nil, true
	}

	if msg.Type == tea.KeyDown || msg.Type == tea.KeyCtrlJ {
		p.focusSearch = false
		p.input.Blur()
		if len(p.list.Items()) > 0 {
			p.list.Select(p.firstActionIndex())
			p.moveListSelection(1)
		}
		return nil, true
	}

	if msg.Type == tea.KeyUp || msg.Type == tea.KeyCtrlK {
		p.focusSearch = false
		p.input.Blur()
		items := p.list.Items()
		if len(items) > 0 {
			for i := len(items) - 1; i >= 0; i-- {
				if pi, ok := items[i].(PaletteItem); ok && !pi.IsHeader {
					p.list.Select(i)
					break
				}
			}
		}
		return nil, true
	}

	prev := p.input.Value()
	var cmd tea.Cmd
	p.input, cmd = p.input.Update(msg)
	if p.input.Value() != prev {
		p.applyFilter(p.input.Value())
	}
	return cmd, true
}

func (p *Palette) updateList(msg tea.KeyMsg) (tea.Cmd, bool) {
	if msg.Type == tea.KeyTab {
		p.focusSearch = true
		p.input.Focus()
		return nil, true
	}

	if msg.Type == tea.KeyEnter {
		if sel, ok := p.list.SelectedItem().(PaletteItem); ok {
			if sel.IsHeader {
				return nil, true
			}
			p.lastAction = sel.Action
			p.lastPayload = sel.Payload
			p.lastLibraryID = sel.LibraryID
			p.lastItemID = sel.ItemID
			p.lastData = sel.Data
			p.Close()
			return nil, true
		}
		return nil, true
	}

	if msg.String() == "p" {
		if sel, ok := p.list.SelectedItem().(PaletteItem); ok && !sel.IsHeader && sel.Action == ActionContentNavigate {
			p.lastAction = ActionPlayDirect
			p.lastPayload = sel.Payload
			p.lastLibraryID = sel.LibraryID
			p.lastItemID = sel.ItemID
			p.lastData = sel.Data
			p.Close()
			return nil, true
		}
		return nil, true
	}

	delta := 0
	switch {
	case msg.Type == tea.KeyUp || msg.String() == "k" || msg.Type == tea.KeyCtrlK:
		delta = -1
	case msg.Type == tea.KeyDown || msg.String() == "j" || msg.Type == tea.KeyCtrlJ:
		delta = 1
	}

	if delta != 0 {
		p.moveListSelection(delta)
		return nil, true
	}

	if msg.Type == tea.KeyRunes || msg.Type == tea.KeySpace {
		p.focusSearch = true
		p.input.Focus()
		var cmd tea.Cmd
		p.input, cmd = p.input.Update(msg)
		p.applyFilter(p.input.Value())
		return cmd, true
	}

	var cmd tea.Cmd
	p.list, cmd = p.list.Update(msg)
	if p.list.Index() >= len(p.list.Items()) {
		p.list.ResetSelected()
		if len(p.list.Items()) > 0 {
			p.list.Select(0)
		}
	}
	return cmd, true
}

func (p *Palette) moveListSelection(delta int) {
	items := p.list.Items()
	if len(items) == 0 {
		return
	}

	newIdx := p.list.Index() + delta
	if newIdx < 0 {
		newIdx = 0
	}
	if newIdx >= len(items) {
		newIdx = len(items) - 1
	}

	iter := 0
	for iter < len(items) {
		if item, ok := items[newIdx].(PaletteItem); ok && item.IsHeader {
			newIdx += delta
			if newIdx < 0 {
				newIdx = 0
			}
			if newIdx >= len(items) {
				newIdx = len(items) - 1
			}
		} else {
			break
		}
		iter++
	}

	if newIdx >= 0 && newIdx < len(items) {
		if item, ok := items[newIdx].(PaletteItem); !ok || !item.IsHeader {
			p.list.ResetSelected()
			p.list.Select(newIdx)
		}
	}
}

func (p *Palette) firstActionIndex() int {
	for i, item := range p.list.Items() {
		if pi, ok := item.(PaletteItem); ok && !pi.IsHeader {
			return i
		}
	}
	return 0
}

func (p *Palette) applyFilter(query string) {
	normalized := strings.TrimSpace(query)
	items := filterPaletteItems(p.staticItems, normalized)

	if normalized != "" && p.searchFn != nil {
		contentItems := p.searchFn(normalized)
		if len(contentItems) > 0 {
			items = append(items, PaletteItem{Label: "Content", IsHeader: true})
			items = append(items, contentItems...)
		}
	}

	listItems := make([]list.Item, len(items))
	for i := range items {
		listItems[i] = items[i]
	}
	p.list.SetItems(listItems)
	p.list.ResetSelected()
	if len(listItems) > 0 {
		p.list.Select(p.firstActionIndex())
	}
}

func (p Palette) View() string {
	if !p.visible {
		return ""
	}

	title := p.styles.Title.PaddingBottom(0).Render("Command Palette")

	searchLine := p.input.View()

	listContent := p.list.View()

	// Pad or trim listContent to match configured list height to keep dialog height static
	targetHeight := p.list.Height()
	lines := strings.Split(listContent, "\n")
	for len(lines) < targetHeight {
		lines = append(lines, "")
	}
	if len(lines) > targetHeight {
		lines = lines[:targetHeight]
	}
	listContent = strings.Join(lines, "\n")

	var footer string
	if p.focusSearch {
		footer = p.styles.Muted.Render("type to filter  esc/enter → results  esc esc close")
	} else {
		footer = p.styles.Muted.Render("↑↓/jk navigate  enter open  p play  tab → search  esc close")
	}

	box := lipgloss.JoinVertical(lipgloss.Left, title, searchLine, listContent, footer)
	return p.styles.Border.Width(paletteWidth).Render(box)
}

func filterPaletteItems(items []PaletteItem, query string) []PaletteItem {
	if query == "" {
		result := make([]PaletteItem, len(items))
		copy(result, items)
		return result
	}

	query = strings.ToLower(query)
	filtered := make([]PaletteItem, 0)
	currentHeader := ""
	hasHeader := false

	for _, item := range items {
		if item.IsHeader {
			currentHeader = item.Label
			hasHeader = false
			continue
		}
		if matchQuery(item.Label, query) {
			if !hasHeader && currentHeader != "" {
				filtered = append(filtered, PaletteItem{Label: currentHeader, IsHeader: true})
				hasHeader = true
			}
			filtered = append(filtered, item)
		}
	}
	return filtered
}

func matchQuery(label, query string) bool {
	label = strings.ToLower(label)
	qi := 0
	for _, r := range label {
		if qi < len(query) && r == rune(query[qi]) {
			qi++
		}
	}
	return qi == len(query)
}

func ansiTruncate(s string, maxWidth int, ellipsis string) string {
	w := lipgloss.Width(s)
	if w <= maxWidth {
		return s
	}
	keep := maxWidth - lipgloss.Width(ellipsis)
	if keep < 0 {
		keep = 0
	}
	runes := []rune(s)
	width := 0
	for i, r := range runes {
		rw := lipgloss.Width(string(r))
		if width+rw > keep {
			return string(runes[:i]) + ellipsis
		}
		width += rw
	}
	return s
}
