package sessions

import (
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/Thelost77/pine/internal/abs"
	"github.com/Thelost77/pine/internal/ui"
	tea "github.com/charmbracelet/bubbletea"
)

const thisDeviceID = "pine-laptop-0001"

var episodeTarget = Target{ItemID: "pod-001", EpisodeID: "ep-001", Title: "Braid and The Witness"}

func newTestModel(target Target) Model {
	m := New(ui.DefaultStyles(), nil, target, thisDeviceID)
	m.SetSize(120, 40)
	return m
}

func session(id, itemID, episodeID string, start, current, listened float64, updatedAt int64, device abs.DeviceInfo) abs.ListeningSession {
	s := abs.ListeningSession{
		ID:            id,
		LibraryItemID: itemID,
		EpisodeID:     episodeID,
		MediaType:     "podcast",
		DisplayTitle:  "Braid and The Witness",
		StartTime:     start,
		CurrentTime:   current,
		TimeListening: listened,
		UpdatedAt:     updatedAt,
		DeviceInfo:    device,
	}
	s.MediaMetadata.Title = "JBlow"
	return s
}

var (
	pineLaptop = abs.DeviceInfo{DeviceID: thisDeviceID, ClientName: "pine", ClientVersion: "v0.8.3", DeviceName: "Pine laptop"}
	iPhone     = abs.DeviceInfo{DeviceID: "phone-0002", ClientName: "ShelfPlayer", ClientVersion: "3.3.1", DeviceName: "Apple iPhone17,3"}
)

// The sessions of the morning of 2026-10-21: the phone moved the position
// back from 1:20:17 to 0:46:39.
func episodeSessions() []abs.ListeningSession {
	return []abs.ListeningSession{
		session("ls-3", "pod-001", "ep-001", 0, 2799, 7.39, 1792570411000, iPhone),
		session("ls-2", "pod-001", "ep-001", 2830, 4817.7, 1439.6, 1792570351000, pineLaptop),
	}
}

func localTime(ms int64) string {
	return time.UnixMilli(ms).Local().Format("2006-01-02 15:04")
}

func TestItemRowsShowPositionsDeviceTimeAndListening(t *testing.T) {
	m := newTestModel(episodeTarget)
	m, _ = m.Update(LoadedMsg{Target: episodeTarget, Sessions: episodeSessions(), Total: 2})
	view := m.View()

	for _, want := range []string{
		"Listening Sessions — Braid and The Witness  (2)",
		"46:39  from 0:00 · Apple iPhone17,3",
		localTime(1792570411000) + " · ShelfPlayer 3.3.1 · listened 7s",
		"1:20:17  from 47:10 · Pine laptop (this device)",
		localTime(1792570351000) + " · pine v0.8.3 · listened 23m",
	} {
		if !strings.Contains(view, want) {
			t.Errorf("view has no %q:\n%s", want, view)
		}
	}
	if strings.Contains(view, "Apple iPhone17,3 (this device)") {
		t.Errorf("view marks the phone as this device:\n%s", view)
	}
}

func TestAllItemsRowsStartWithTitle(t *testing.T) {
	book := abs.ListeningSession{
		LibraryItemID: "li-001",
		MediaType:     "book",
		DisplayTitle:  "Ostatnie rozdanie",
		DisplayAuthor: "Wiesław Myśliwski",
		CurrentTime:   15045,
		UpdatedAt:     1792178008500,
		DeviceInfo:    iPhone,
	}
	m := newTestModel(Target{})
	m, _ = m.Update(LoadedMsg{Sessions: []abs.ListeningSession{episodeSessions()[1], book}, Total: 955})
	view := m.View()

	for _, want := range []string{
		"Listening History  (2 of 955)",
		"JBlow — Braid and The Witness",
		"1:20:17 · " + localTime(1792570351000) + " · Pine laptop (this device)",
		"Ostatnie rozdanie — Wiesław Myśliwski",
		"4:10:45 · " + localTime(1792178008500) + " · Apple iPhone17,3",
	} {
		if !strings.Contains(view, want) {
			t.Errorf("view has no %q:\n%s", want, view)
		}
	}
}

func TestEnterPlaysFromEndOfSelectedSession(t *testing.T) {
	m := newTestModel(episodeTarget)
	m, _ = m.Update(LoadedMsg{Target: episodeTarget, Sessions: episodeSessions(), Total: 2})
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("j")})

	_, cmd := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	if cmd == nil {
		t.Fatal("enter gave no command")
	}
	got, ok := cmd().(PlayFromMsg)
	if !ok {
		t.Fatalf("enter gave %T, want PlayFromMsg", cmd())
	}
	want := PlayFromMsg{ItemID: "pod-001", EpisodeID: "ep-001", Position: 4817.7}
	if got != want {
		t.Errorf("PlayFromMsg = %+v, want %+v", got, want)
	}
}

func TestEscGoesBack(t *testing.T) {
	m := newTestModel(episodeTarget)
	m, _ = m.Update(LoadedMsg{Target: episodeTarget, Sessions: episodeSessions(), Total: 2})

	_, cmd := m.Update(tea.KeyMsg{Type: tea.KeyEsc})
	if cmd == nil {
		t.Fatal("esc gave no command")
	}
	if _, ok := cmd().(BackMsg); !ok {
		t.Errorf("esc gave %T, want BackMsg", cmd())
	}
}

func TestErrorShowsCause(t *testing.T) {
	m := newTestModel(episodeTarget)
	m, _ = m.Update(LoadedMsg{Target: episodeTarget, Err: errors.New("fetch listening sessions: boom")})

	if view := m.View(); !strings.Contains(view, "fetch listening sessions: boom") {
		t.Errorf("view has no error:\n%s", view)
	}
}

// TestLoadedMsgOfOtherTargetIsIgnored verifies that a late answer for a
// different item does not fill the screen.
func TestLoadedMsgOfOtherTargetIsIgnored(t *testing.T) {
	m := newTestModel(episodeTarget)
	m, _ = m.Update(LoadedMsg{Target: Target{ItemID: "li-001"}, Sessions: episodeSessions(), Total: 2})

	if view := m.View(); strings.Contains(view, "Apple iPhone17,3") {
		t.Errorf("view shows the sessions of another target:\n%s", view)
	}
}
