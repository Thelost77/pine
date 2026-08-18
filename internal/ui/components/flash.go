package components

import (
	"io"
	"time"

	"github.com/Thelost77/spruce/internal/ui"
	"github.com/charmbracelet/bubbles/list"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// ItemFlashDuration is how long a queued-item highlight stays visible.
const ItemFlashDuration = 167 * time.Millisecond

// ItemFlashExpiredMsg clears one finished item flash pulse.
type ItemFlashExpiredMsg struct {
	Owner      *ItemFlash
	Generation int
	ItemIDs    []string
}

// FlashIDProvider is implemented by list items that can be highlighted after an action.
type FlashIDProvider interface {
	FlashID() string
}

// ItemFlash tracks temporary highlights by item ID.
type ItemFlash struct {
	active     map[string]int
	generation int
}

// NewItemFlash creates an empty flash tracker.
func NewItemFlash() *ItemFlash {
	return &ItemFlash{}
}

// Active reports whether id is currently highlighted.
func (f *ItemFlash) Active(id string) bool {
	if f == nil || id == "" || len(f.active) == 0 {
		return false
	}
	_, ok := f.active[id]
	return ok
}

// Start highlights the given IDs for ItemFlashDuration and returns the expiry command.
// Existing highlights for other IDs retain their own expiry time.
func (f *ItemFlash) Start(ids ...string) tea.Cmd {
	if f == nil {
		return nil
	}

	unique := make(map[string]struct{}, len(ids))
	for _, id := range ids {
		if id != "" {
			unique[id] = struct{}{}
		}
	}
	if len(unique) == 0 {
		return nil
	}

	f.generation++
	generation := f.generation
	if f.active == nil {
		f.active = make(map[string]int, len(unique))
	}

	itemIDs := make([]string, 0, len(unique))
	for id := range unique {
		f.active[id] = generation
		itemIDs = append(itemIDs, id)
	}

	return tea.Tick(ItemFlashDuration, func(time.Time) tea.Msg {
		return ItemFlashExpiredMsg{
			Owner:      f,
			Generation: generation,
			ItemIDs:    itemIDs,
		}
	})
}

// HandleExpired clears items that still belong to the completed pulse.
func (f *ItemFlash) HandleExpired(msg ItemFlashExpiredMsg) bool {
	if f == nil || msg.Owner != f {
		return false
	}

	cleared := false
	for _, id := range msg.ItemIDs {
		generation, ok := f.active[id]
		if ok && generation == msg.Generation {
			delete(f.active, id)
			cleared = true
		}
	}
	return cleared
}

// VisibleFlashIDs returns IDs for flash-capable items rendered on the current list page.
func VisibleFlashIDs(l list.Model) []string {
	items := l.VisibleItems()
	start, end := l.Paginator.GetSliceBounds(len(items))
	ids := make([]string, 0, end-start)
	for _, item := range items[start:end] {
		if provider, ok := item.(FlashIDProvider); ok {
			if id := provider.FlashID(); id != "" {
				ids = append(ids, id)
			}
		}
	}
	return ids
}

// FlashDelegate wraps list.DefaultDelegate and recolors items that are flashing.
type FlashDelegate struct {
	list.DefaultDelegate
	flash *ItemFlash

	flashNormalTitle   lipgloss.Style
	flashNormalDesc    lipgloss.Style
	flashSelectedTitle lipgloss.Style
	flashSelectedDesc  lipgloss.Style
}

// NewFlashDelegate builds a delegate that highlights items tracked by flash.
func NewFlashDelegate(styles ui.Styles, flash *ItemFlash) FlashDelegate {
	del := list.NewDefaultDelegate()
	del.Styles.SelectedTitle = del.Styles.SelectedTitle.
		Foreground(styles.Accent.GetForeground()).
		BorderForeground(styles.Accent.GetForeground())
	del.Styles.SelectedDesc = del.Styles.SelectedDesc.
		Foreground(styles.Muted.GetForeground()).
		BorderForeground(styles.Accent.GetForeground())

	info := styles.Info.GetForeground()
	return FlashDelegate{
		DefaultDelegate: del,
		flash:           flash,
		flashNormalTitle: del.Styles.NormalTitle.
			Foreground(info).
			Bold(true),
		flashNormalDesc: del.Styles.NormalDesc.
			Foreground(info),
		flashSelectedTitle: del.Styles.SelectedTitle.
			Foreground(info).
			Bold(true),
		flashSelectedDesc: del.Styles.SelectedDesc.
			Foreground(info),
	}
}

// Render draws the item, applying flash styles when the item ID is active.
func (d FlashDelegate) Render(w io.Writer, m list.Model, index int, item list.Item) {
	if d.flash != nil {
		if provider, ok := item.(FlashIDProvider); ok && d.flash.Active(provider.FlashID()) {
			del := d.DefaultDelegate
			del.Styles.NormalTitle = d.flashNormalTitle
			del.Styles.NormalDesc = d.flashNormalDesc
			del.Styles.SelectedTitle = d.flashSelectedTitle
			del.Styles.SelectedDesc = d.flashSelectedDesc
			del.Render(w, m, index, item)
			return
		}
	}
	d.DefaultDelegate.Render(w, m, index, item)
}
