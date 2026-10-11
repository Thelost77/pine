package components

import (
	"bytes"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/Thelost77/pine/internal/ui"
	"github.com/charmbracelet/bubbles/list"
	"github.com/charmbracelet/lipgloss"
	"github.com/muesli/termenv"
)

type flashItem struct {
	id    string
	title string
}

func (i flashItem) Title() string       { return i.title }
func (i flashItem) Description() string { return "desc" }
func (i flashItem) FilterValue() string { return i.title }
func (i flashItem) FlashID() string     { return i.id }

func TestItemFlashDuration(t *testing.T) {
	if ItemFlashDuration != 167*time.Millisecond {
		t.Fatalf("ItemFlashDuration = %s, want 167ms", ItemFlashDuration)
	}
}

func TestItemFlashStartAndExpire(t *testing.T) {
	flash := NewItemFlash()
	if cmd := flash.Start("a", "", "b"); cmd == nil {
		t.Fatal("expected expiry command")
	}
	if !flash.Active("a") || !flash.Active("b") {
		t.Fatal("expected IDs to be active")
	}

	msg := ItemFlashExpiredMsg{
		Owner:      flash,
		Generation: flash.generation,
		ItemIDs:    []string{"a", "b"},
	}
	if !flash.HandleExpired(msg) {
		t.Fatal("expected expiry to clear current flash")
	}
	if flash.Active("a") || flash.Active("b") {
		t.Fatal("expected flash cleared")
	}
}

func TestItemFlashTracksOverlappingPulsesPerItem(t *testing.T) {
	flash := NewItemFlash()
	_ = flash.Start("old", "same")
	firstGeneration := flash.generation
	_ = flash.Start("new", "same")
	secondGeneration := flash.generation

	firstExpiry := ItemFlashExpiredMsg{
		Owner:      flash,
		Generation: firstGeneration,
		ItemIDs:    []string{"old", "same"},
	}
	if !flash.HandleExpired(firstExpiry) {
		t.Fatal("expected first expiry to clear its unchanged item")
	}
	if flash.Active("old") {
		t.Fatal("expected old item cleared")
	}
	if !flash.Active("new") || !flash.Active("same") {
		t.Fatal("newer pulses must remain active")
	}

	secondExpiry := ItemFlashExpiredMsg{
		Owner:      flash,
		Generation: secondGeneration,
		ItemIDs:    []string{"new", "same"},
	}
	if !flash.HandleExpired(secondExpiry) {
		t.Fatal("expected second expiry to clear remaining items")
	}
}

func TestItemFlashIgnoresOtherOwner(t *testing.T) {
	flash := NewItemFlash()
	other := NewItemFlash()
	_ = flash.Start("a")

	if flash.HandleExpired(ItemFlashExpiredMsg{
		Owner:      other,
		Generation: flash.generation,
		ItemIDs:    []string{"a"},
	}) {
		t.Fatal("expiry from another tracker must be ignored")
	}
	if !flash.Active("a") {
		t.Fatal("other owner's expiry cleared active item")
	}
}

func TestItemFlashStartEmptyIsNoop(t *testing.T) {
	flash := NewItemFlash()
	if cmd := flash.Start("", ""); cmd != nil {
		t.Fatal("expected nil command for empty IDs")
	}
}

func TestVisibleFlashIDsReturnsCurrentPage(t *testing.T) {
	flash := NewItemFlash()
	del := NewFlashDelegate(ui.DefaultStyles(), flash)
	del.SetHeight(1)
	del.SetSpacing(0)
	items := []list.Item{
		flashItem{id: "a", title: "A"},
		flashItem{id: "b", title: "B"},
		flashItem{id: "c", title: "C"},
	}
	m := list.New(items, del, 40, 3)
	m.SetShowTitle(false)
	m.SetShowStatusBar(false)
	m.SetShowPagination(false)
	m.SetShowHelp(false)
	m.Paginator.PerPage = 2

	if got := VisibleFlashIDs(m); !reflect.DeepEqual(got, []string{"a", "b"}) {
		t.Fatalf("first page IDs = %v, want [a b]", got)
	}
	m.NextPage()
	if got := VisibleFlashIDs(m); !reflect.DeepEqual(got, []string{"c"}) {
		t.Fatalf("second page IDs = %v, want [c]", got)
	}
}

func TestFlashDelegateChangesTextWithoutChangingBackground(t *testing.T) {
	oldProfile := lipgloss.ColorProfile()
	lipgloss.SetColorProfile(termenv.TrueColor)
	t.Cleanup(func() { lipgloss.SetColorProfile(oldProfile) })

	styles := ui.DefaultStyles()
	flash := NewItemFlash()
	del := NewFlashDelegate(styles, flash)
	if !reflect.DeepEqual(del.flashSelectedTitle.GetBackground(), del.Styles.SelectedTitle.GetBackground()) {
		t.Fatal("flash must not change selected title background")
	}
	if !reflect.DeepEqual(del.flashSelectedDesc.GetBackground(), del.Styles.SelectedDesc.GetBackground()) {
		t.Fatal("flash must not change selected description background")
	}
	if !reflect.DeepEqual(del.flashSelectedTitle.GetForeground(), styles.Info.GetForeground()) {
		t.Fatal("flash title must use the theme info text color")
	}

	items := []list.Item{flashItem{id: "t-1", title: "Track One"}}
	m := list.New(items, del, 40, 10)
	m.SetShowStatusBar(false)
	m.SetShowHelp(false)
	m.SetShowTitle(false)

	var before bytes.Buffer
	del.Render(&before, m, 0, items[0])
	_ = flash.Start("t-1")
	var after bytes.Buffer
	del.Render(&after, m, 0, items[0])

	if before.String() == after.String() {
		t.Fatalf("expected flash styles to change rendered output\nbefore=%q\nafter=%q", before.String(), after.String())
	}
	if !strings.Contains(after.String(), "Track One") {
		t.Fatalf("flash render lost title: %q", after.String())
	}
}
