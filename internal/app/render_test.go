package app

import (
	"testing"

	"github.com/Thelost77/pine/internal/abs"
)

func TestViewRendersChapterOverlay(t *testing.T) {
	m := newPlaybackTestModel()
	m.width = 100
	m.height = 30
	m.sessionID = "sess-123"
	m.player.Title = "Test Book"
	m.player.Playing = true
	m.chapters = []abs.Chapter{
		{ID: 0, Start: 0, End: 60, Title: "Opening Credits"},
		{ID: 1, Start: 60, End: 120, Title: "Chapter One"},
		{ID: 2, Start: 120, End: 180, Title: "Chapter Two"},
	}
	m.chapterOverlayVisible = true
	m.chapterOverlayIndex = 1

	view := m.View()

	for _, want := range []string{
		"Chapter Navigation",
		"Test Book",
		"  1. Opening Credits",
		"› 2. Chapter One",
		"  3. Chapter Two",
	} {
		if !containsString(view, want) {
			t.Fatalf("View() missing %q\n%s", want, view)
		}
	}
}

func TestViewRendersChapterOrdinalsAlongsideRawTitles(t *testing.T) {
	m := newPlaybackTestModel()
	m.width = 100
	m.height = 30
	m.sessionID = "sess-123"
	m.player.Title = "Test Book"
	m.player.Playing = true
	m.chapters = []abs.Chapter{
		{ID: 0, Start: 0, End: 60, Title: "01-Poziom-smierci"},
		{ID: 1, Start: 60, End: 120, Title: "04-Poziom-smierci"},
		{ID: 2, Start: 120, End: 180, Title: "98-Poziom-smierci"},
	}
	m.chapterOverlayVisible = true
	m.chapterOverlayIndex = 2

	view := m.View()

	for _, want := range []string{
		"  1. 01-Poziom-smierci",
		"  2. 04-Poziom-smierci",
		"› 3. 98-Poziom-smierci",
	} {
		if !containsString(view, want) {
			t.Fatalf("View() missing %q\n%s", want, view)
		}
	}
}

func TestViewHintsAdvertiseChaptersOnlyWhenAvailable(t *testing.T) {
	m := newPlaybackTestModel()

	if containsString(m.viewHints(), "c chapters") {
		t.Fatal("chapter hint should be hidden when playback is inactive")
	}

	m.sessionID = "sess-123"
	m.player.Playing = true
	if containsString(m.viewHints(), "c chapters") {
		t.Fatal("chapter hint should be hidden when no chapters exist")
	}

	m.chapters = []abs.Chapter{{ID: 0, Start: 0, End: 60, Title: "One"}}
	if !containsString(m.viewHints(), "c chapters") {
		t.Fatalf("chapter hint should be shown when playback chapters exist\n%s", m.viewHints())
	}
}

func TestViewHintsAdvertiseQueueActionsAndCountOnDetail(t *testing.T) {
	m := newPlaybackTestModel()
	m.screen = ScreenDetail
	m.sessionID = "sess-123"
	m.player.Playing = true
	m.queue = []QueueEntry{{Item: abs.LibraryItem{ID: "item-1"}}}

	hints := m.viewHints()
	if !containsString(hints, "a queue") {
		t.Fatalf("detail hints should advertise add to queue\n%s", hints)
	}
	if !containsString(hints, "A next") {
		t.Fatalf("detail hints should advertise play next\n%s", hints)
	}
	if !containsString(hints, "> next queued") {
		t.Fatalf("detail hints should advertise queue skip\n%s", hints)
	}
	if !containsString(hints, "1 queued") {
		t.Fatalf("detail hints should surface queue count\n%s", hints)
	}
}

func TestHelpOverlayDocumentsChapterOverlay(t *testing.T) {
	m := newPlaybackTestModel()
	m.help.Toggle()

	view := m.View()
	if !containsString(view, "c") || !containsString(view, "open chapters") {
		t.Fatalf("help overlay should document chapter overlay binding\n%s", view)
	}
	if !containsString(view, ">") || !containsString(view, "play next queued") {
		t.Fatalf("help overlay should document next queued binding\n%s", view)
	}
}
