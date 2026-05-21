package components

import (
	"strings"
	"testing"

	"github.com/Thelost77/pine/internal/ui"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

func TestPaletteOpenClose(t *testing.T) {
	p := NewPalette()
	if p.Visible() {
		t.Fatal("palette should not be visible initially")
	}

	p.Open([]PaletteItem{
		{Label: "Go Home", Action: ActionGoHome},
	}, nil)
	if !p.Visible() {
		t.Fatal("palette should be visible after Open")
	}

	p.Close()
	if p.Visible() {
		t.Fatal("palette should not be visible after Close")
	}
}

func TestPaletteStartsInListMode(t *testing.T) {
	p := NewPalette()
	p.Open([]PaletteItem{
		{Label: "Go Home", Action: ActionGoHome},
	}, nil)

	if p.focusSearch {
		t.Fatal("palette should start in list mode by default")
	}
}

func TestPaletteTabMovesToSearchAndBack(t *testing.T) {
	p := NewPalette()
	p.Open([]PaletteItem{
		{Label: "Go Home", Action: ActionGoHome},
		{Label: "Go Library", Action: ActionGoLibrary},
	}, nil)

	_, _ = p.Update(tea.KeyMsg{Type: tea.KeyTab})
	if !p.focusSearch {
		t.Fatal("first tab should move focus to search")
	}

	_, _ = p.Update(tea.KeyMsg{Type: tea.KeyTab})
	if p.focusSearch {
		t.Fatal("second tab should move focus back to list")
	}
}

func TestPaletteEscFromSearchReturnsToList(t *testing.T) {
	p := NewPalette()
	p.Open([]PaletteItem{
		{Label: "Go Home", Action: ActionGoHome},
	}, nil)

	_, _ = p.Update(tea.KeyMsg{Type: tea.KeyTab})
	if !p.focusSearch {
		t.Fatal("tab should move to search")
	}

	_, _ = p.Update(tea.KeyMsg{Type: tea.KeyEsc})
	if p.focusSearch {
		t.Fatal("esc from search should return to list")
	}
	if !p.Visible() {
		t.Fatal("palette should still be visible after esc from search")
	}
}

func TestPaletteEnterExecutesAction(t *testing.T) {
	p := NewPalette()
	p.Open([]PaletteItem{
		{Label: "Go Library", Action: ActionGoLibrary},
	}, nil)

	_, _ = p.Update(tea.KeyMsg{Type: tea.KeyEnter})
	if p.Visible() {
		t.Fatal("palette should close after executing action")
	}
	action, _, _, _, _ := p.SelectedAction()
	if action != ActionGoLibrary {
		t.Fatalf("expected ActionGoLibrary, got %v", action)
	}
}

func TestPaletteEscCloses(t *testing.T) {
	p := NewPalette()
	p.Open([]PaletteItem{
		{Label: "Go Home", Action: ActionGoHome},
	}, nil)

	_, _ = p.Update(tea.KeyMsg{Type: tea.KeyEsc})
	if p.Visible() {
		t.Fatal("palette should close on Esc")
	}
}

func TestPaletteNavigationSkipsHeaders(t *testing.T) {
	p := NewPalette()
	p.Open([]PaletteItem{
		{Label: "Navigation", IsHeader: true},
		{Label: "Go Home", Action: ActionGoHome},
		{Label: "Go Library", Action: ActionGoLibrary},
		{Label: "Player", IsHeader: true},
		{Label: "Play / Pause", Action: ActionTogglePlay},
	}, nil)

	idx := p.list.Index()
	if idx < 1 {
		t.Fatalf("expected initial index >= 1 (first non-header), got %d items=%v", idx, len(p.list.Items()))
	}

	_, _ = p.Update(tea.KeyMsg{Type: tea.KeyDown})
	if p.list.Index() != 2 {
		t.Fatalf("expected index 2 after moving down, got %d", p.list.Index())
	}

	_, _ = p.Update(tea.KeyMsg{Type: tea.KeyDown})
	if p.list.Index() != 4 {
		t.Fatalf("expected index 4 (skipping header at 3), got %d", p.list.Index())
	}

	_, _ = p.Update(tea.KeyMsg{Type: tea.KeyUp})
	if p.list.Index() != 2 {
		t.Fatalf("expected index 2 after moving up (skipping header at 3), got %d", p.list.Index())
	}
}

func TestPaletteJKNavigation(t *testing.T) {
	p := NewPalette()
	p.Open([]PaletteItem{
		{Label: "Go Home", Action: ActionGoHome},
		{Label: "Go Library", Action: ActionGoLibrary},
	}, nil)

	_, _ = p.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'j'}})
	if p.list.Index() != 1 {
		t.Fatalf("'j' should move down, got %d", p.list.Index())
	}

	_, _ = p.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'k'}})
	if p.list.Index() != 0 {
		t.Fatalf("'k' should move up, got %d", p.list.Index())
	}
}

func TestPaletteFilter(t *testing.T) {
	p := NewPalette()
	p.Open([]PaletteItem{
		{Label: "Go Home", Action: ActionGoHome},
		{Label: "Go Library", Action: ActionGoLibrary},
		{Label: "Play / Pause", Action: ActionTogglePlay},
		{Label: "Seek Forward", Action: ActionSeekForward},
	}, nil)

	p.input.SetValue("go")
	p.applyFilter("go")

	if len(p.list.Items()) != 2 {
		t.Fatalf("filter 'go' should match 2 items, got %d", len(p.list.Items()))
	}
}

func TestPaletteFilterFuzzyMatch(t *testing.T) {
	p := NewPalette()
	p.Open([]PaletteItem{
		{Label: "Sleep Timer: 15m", Action: ActionSleep15},
		{Label: "Sleep Timer: 30m", Action: ActionSleep30},
		{Label: "Switch Library", Action: ActionSwitchLibrary},
	}, nil)

	p.input.SetValue("sl")
	p.applyFilter("sl")

	if len(p.list.Items()) != 3 {
		t.Fatalf("filter 'sl' should match 3 items (2 sleep + switch library), got %d", len(p.list.Items()))
	}
}

func TestPaletteSearchFunctionCalled(t *testing.T) {
	p := NewPalette()
	called := false
	p.Open([]PaletteItem{
		{Label: "Go Home", Action: ActionGoHome},
	}, func(query string) []PaletteItem {
		called = true
		return []PaletteItem{
			{Label: "Dune", Action: ActionContentNavigate, ItemID: "dune-1"},
		}
	})

	p.input.SetValue("dune")
	p.applyFilter("dune")

	if !called {
		t.Fatal("search function should be called when query is non-empty")
	}

	found := false
	for _, item := range p.list.Items() {
		if pi, ok := item.(PaletteItem); ok && pi.Label == "Dune" {
			found = true
		}
	}
	if !found {
		t.Fatal("content item 'Dune' should appear in filtered list")
	}
}

func TestPaletteViewWhenNotVisible(t *testing.T) {
	p := NewPalette()
	if v := p.View(); v != "" {
		t.Fatal("View() should return empty string when not visible")
	}
}

func TestPaletteOpenEmptyList(t *testing.T) {
	p := NewPalette()
	p.Open(nil, nil)
	if !p.Visible() {
		t.Fatal("palette should be visible even with empty items")
	}
}

func TestPaletteClearSelection(t *testing.T) {
	p := NewPalette()
	p.Open([]PaletteItem{
		{Label: "Go Home", Action: ActionGoHome},
	}, nil)
	_, _ = p.Update(tea.KeyMsg{Type: tea.KeyEnter})
	if p.Visible() {
		t.Fatal("palette should close after action")
	}
	p.ClearSelection()
	action, _, _, _, _ := p.SelectedAction()
	if action != ActionNone {
		t.Fatalf("expected ActionNone after ClearSelection, got %v", action)
	}
}

func TestPaletteEnterOnHeaderDoesNotExecute(t *testing.T) {
	p := NewPalette()
	p.Open([]PaletteItem{
		{Label: "Navigation", IsHeader: true},
		{Label: "Go Home", Action: ActionGoHome},
	}, nil)

	p.list.Select(0)

	_, _ = p.Update(tea.KeyMsg{Type: tea.KeyEnter})
	if !p.Visible() {
		t.Fatal("palette should stay open when Enter is pressed on header")
	}
	action, _, _, _, _ := p.SelectedAction()
	if action != ActionNone {
		t.Fatalf("expected ActionNone when selecting header, got %v", action)
	}
}

func TestPaletteDataField(t *testing.T) {
	type myData struct{ Name string }
	p := NewPalette()
	p.Open([]PaletteItem{
		{Label: "Test", Action: ActionContentNavigate, ItemID: "id1", Data: myData{Name: "hello"}},
	}, nil)
	_, _ = p.Update(tea.KeyMsg{Type: tea.KeyEnter})
	if p.Visible() {
		t.Fatal("palette should close after action")
	}
	_, _, _, _, data := p.SelectedAction()
	if md, ok := data.(myData); !ok || md.Name != "hello" {
		t.Fatalf("expected Data to contain myData{Name:hello}, got %v", data)
	}
}

func TestPalettePlayDirect(t *testing.T) {
	p := NewPalette()
	p.Open([]PaletteItem{
		{Label: "Go Home", Action: ActionGoHome},
		{Label: "Dune", Action: ActionContentNavigate, ItemID: "dune-1"},
	}, nil)

	p.list.Select(1)

	_, _ = p.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'p'}})
	if p.Visible() {
		t.Fatal("palette should close after play direct")
	}
	action, _, _, _, _ := p.SelectedAction()
	if action != ActionPlayDirect {
		t.Fatalf("expected ActionPlayDirect, got %v", action)
	}
}

func TestPalettePlayDirectOnNonContentDoesNothing(t *testing.T) {
	p := NewPalette()
	p.Open([]PaletteItem{
		{Label: "Go Home", Action: ActionGoHome},
	}, nil)

	_, _ = p.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'p'}})
	if !p.Visible() {
		t.Fatal("palette should stay open when 'p' on non-content item")
	}
	action, _, _, _, _ := p.SelectedAction()
	if action != ActionNone {
		t.Fatalf("expected ActionNone, got %v", action)
	}
}

func TestMatchQuery(t *testing.T) {
	tests := []struct {
		label string
		query string
		want  bool
	}{
		{"Go Home", "gh", true},
		{"Go Home", "go", true},
		{"Go Home", "home", true},
		{"Go Library", "gl", true},
		{"Go Library", "lib", true},
		{"Go Library", "xy", false},
		{"Sleep Timer: 15m", "sl", true},
		{"Sleep Timer: 15m", "st", true},
		{"Sleep Timer: 15m", "15", true},
		{"Play / Pause", "pp", true},
		{"Play / Pause", "/", true},
		{"", "test", false},
		{"Test", "", true},
	}
	for _, tt := range tests {
		got := matchQuery(tt.label, tt.query)
		if got != tt.want {
			t.Errorf("matchQuery(%q, %q) = %v, want %v", tt.label, tt.query, got, tt.want)
		}
	}
}

func TestPaletteViewStaticHeight(t *testing.T) {
	styles := ui.DefaultStyles()
	p := NewPalette()
	p.SetStyles(styles)
	
	// Test on two different sized terminal heights
	for _, terminalHeight := range []int{20, 30} {
		p.SetSize(80, terminalHeight)
		
		// Expected list height calculation:
		// listHeight := max(1, height - 2 - paletteChromeHeight) = max(1, height - 9).
		// paletteChromeHeight = 7 (border(2) + padding(2) + title(1) + input(1) + footer(1)).
		// If height=20, listHeight=11.
		// If height=30, listHeight=20 (capped at maxItems).
		expectedListHeight := terminalHeight - 9
		if expectedListHeight > maxItems {
			expectedListHeight = maxItems
		}
		if expectedListHeight < 1 {
			expectedListHeight = 1
		}
		
		// Open with 2 items (well below expectedListHeight)
		p.Open([]PaletteItem{
			{Label: "Item 1", Action: ActionGoHome},
			{Label: "Item 2", Action: ActionGoLibrary},
		}, nil)
		
		view1 := p.View()
		height1 := len(strings.Split(view1, "\n"))
		
		// Open with 5 items (still below/different number of items)
		p.Open([]PaletteItem{
			{Label: "Item 1", Action: ActionGoHome},
			{Label: "Item 2", Action: ActionGoLibrary},
			{Label: "Item 3", Action: ActionGoSearch},
			{Label: "Item 4", Action: ActionShowQueue},
			{Label: "Item 5", Action: ActionClearQueue},
		}, nil)
		
		view2 := p.View()
		height2 := len(strings.Split(view2, "\n"))
		
		if height1 != height2 {
			t.Errorf("expected palette height to be constant, got height1=%d (2 items) and height2=%d (5 items) for terminalHeight=%d", height1, height2, terminalHeight)
		}
		
		// Expected view height = listHeight + paletteChromeHeight = listHeight + 7
		expectedViewHeight := expectedListHeight + paletteChromeHeight
		if height1 != expectedViewHeight {
			t.Errorf("expected view height %d, got %d for terminalHeight=%d", expectedViewHeight, height1, terminalHeight)
		}
	}
}

func TestPaletteTruncationNoWrapping(t *testing.T) {
	styles := ui.DefaultStyles()
	p := NewPalette()
	p.SetStyles(styles)
	p.SetSize(80, 40)

	longTitle := "The Very Long Episode Title That Just Keeps Going And Going For Way Too Many Characters To Fit In A Single Line — With A Very Long Podcast Name"

	p.Open([]PaletteItem{
		{Label: longTitle, Action: ActionContentNavigate},
	}, func(query string) []PaletteItem {
		return []PaletteItem{
			{Label: "Search: " + longTitle, Action: ActionContentNavigate},
			{Label: "Search: Another Episode With An Even Longer Title That Should Definitely Be Truncated Properly Without Any Wrapping Whatsoever", Action: ActionContentNavigate},
		}
	})

	// Trigger filtering to add search results
	_, _ = p.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'t'}})

	view := p.View()
	if view == "" {
		t.Fatal("view should not be empty")
	}

	lines := strings.Split(view, "\n")
	for i, line := range lines {
		w := lipgloss.Width(line)
		// Border.Width(64) renders at 66 due to lipgloss border char handling
		if w > 66 {
			t.Errorf("line %d has width %d, too wide: %q", i, w, line)
		}
	}

	// Verify dialog height is constant - open with different items and check same height
	p2 := NewPalette()
	p2.SetStyles(styles)
	p2.SetSize(80, 40)
	p2.Open([]PaletteItem{
		{Label: "Short 1", Action: ActionGoHome},
		{Label: "Short 2", Action: ActionGoLibrary},
		{Label: "Short 3", Action: ActionGoSearch},
	}, func(query string) []PaletteItem {
		return []PaletteItem{
			{Label: "Search: " + longTitle, Action: ActionContentNavigate},
		}
	})

	// Trigger filtering
	_, _ = p2.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'q'}})

	view2 := p2.View()
	h1 := len(strings.Split(view, "\n"))
	h2 := len(strings.Split(view2, "\n"))
	if h1 != h2 {
		t.Errorf("dialog height should be constant: got %d lines and %d lines", h1, h2)
	}
}
