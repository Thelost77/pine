package metadataedit

import (
	"context"
	"fmt"
	"strconv"
	"strings"

	"github.com/Thelost77/pine/internal/spruce/jellyfin"
	"github.com/Thelost77/pine/internal/ui"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
)

// SavedMsg reports the result of a metadata save.
type SavedMsg struct {
	ItemID string
	Req    jellyfin.UpdateItemRequest
	Err    error
}

// BackMsg requests returning to the previous screen.
type BackMsg struct{}

// Model is the bubbletea model for editing track or album metadata.
type Model struct {
	client   *jellyfin.Client
	itemID   string
	itemType string // "Track" or "Album"

	inputs  []textinput.Model
	labels  []string
	focused int

	saving  bool
	errText string

	width  int
	height int
	styles ui.Styles
}

// New creates a metadata editor screen for a track or album.
func New(styles ui.Styles, client *jellyfin.Client, itemID, itemType string, track *jellyfin.Track, album *jellyfin.Album) Model {
	var inputs []textinput.Model
	var labels []string

	if itemType == "Track" && track != nil {
		labels = []string{"Title", "Artists (comma separated)", "Album", "Track Number", "Disc Number"}
		inputs = make([]textinput.Model, len(labels))

		inputs[0] = newInput(track.Name, 256)
		inputs[1] = newInput(strings.Join(track.Artists, ", "), 256)
		inputs[2] = newInput(track.Album, 256)

		trackNum := ""
		if track.IndexNumber > 0 {
			trackNum = strconv.Itoa(track.IndexNumber)
		}
		inputs[3] = newInput(trackNum, 16)

		discNum := ""
		if track.ParentIndexNumber > 0 {
			discNum = strconv.Itoa(track.ParentIndexNumber)
		}
		inputs[4] = newInput(discNum, 16)
	} else if album != nil {
		labels = []string{"Title", "Artists (comma separated)", "Production Year"}
		inputs = make([]textinput.Model, len(labels))

		inputs[0] = newInput(album.Name, 256)
		inputs[1] = newInput(strings.Join(album.Artists, ", "), 256)

		yearStr := ""
		if album.ProductionYear > 0 {
			yearStr = strconv.Itoa(album.ProductionYear)
		}
		inputs[2] = newInput(yearStr, 16)
	} else {
		labels = []string{"Title"}
		inputs = []textinput.Model{newInput("", 256)}
	}

	m := Model{
		client:   client,
		itemID:   itemID,
		itemType: itemType,
		inputs:   inputs,
		labels:   labels,
		focused:  0,
		styles:   styles,
	}
	if len(m.inputs) > 0 {
		m.inputs[0].Focus()
	}
	return m
}

func newInput(val string, limit int) textinput.Model {
	ti := textinput.New()
	ti.CharLimit = limit
	ti.SetValue(val)
	ti.Prompt = ""
	return ti
}

func (m Model) Init() tea.Cmd {
	return textinput.Blink
}

func (m *Model) SetSize(width, height int) {
	m.width = width
	m.height = height
	for i := range m.inputs {
		m.inputs[i].Width = max(20, width-30)
	}
}

func (m Model) Update(msg tea.Msg) (Model, tea.Cmd) {
	switch msg := msg.(type) {
	case SavedMsg:
		m.saving = false
		if msg.Err != nil {
			m.errText = fmt.Sprintf("Save failed: %v", msg.Err)
			return m, nil
		}
		return m, func() tea.Msg { return BackMsg{} }

	case tea.KeyMsg:
		if m.saving {
			return m, nil
		}
		switch msg.String() {
		case "esc":
			return m, func() tea.Msg { return BackMsg{} }
		case "tab", "down":
			m.focused = (m.focused + 1) % len(m.inputs)
			return m, m.updateFocus()
		case "shift+tab", "up":
			m.focused = (m.focused - 1 + len(m.inputs)) % len(m.inputs)
			return m, m.updateFocus()
		case "enter":
			return m.save()
		}
	}

	if m.focused >= 0 && m.focused < len(m.inputs) {
		var cmd tea.Cmd
		m.inputs[m.focused], cmd = m.inputs[m.focused].Update(msg)
		return m, cmd
	}
	return m, nil
}

func (m *Model) updateFocus() tea.Cmd {
	var cmds []tea.Cmd
	for i := range m.inputs {
		if i == m.focused {
			cmds = append(cmds, m.inputs[i].Focus())
		} else {
			m.inputs[i].Blur()
		}
	}
	return tea.Batch(cmds...)
}

func (m Model) save() (Model, tea.Cmd) {
	m.saving = true
	m.errText = ""

	req := jellyfin.UpdateItemRequest{
		ID: m.itemID,
	}

	if m.itemType == "Track" && len(m.inputs) >= 5 {
		req.Name = strings.TrimSpace(m.inputs[0].Value())
		artistsStr := strings.TrimSpace(m.inputs[1].Value())
		if artistsStr != "" {
			parts := strings.Split(artistsStr, ",")
			var artists []string
			for _, p := range parts {
				if s := strings.TrimSpace(p); s != "" {
					artists = append(artists, s)
				}
			}
			req.Artists = artists
		}
		req.Album = strings.TrimSpace(m.inputs[2].Value())

		if tNumStr := strings.TrimSpace(m.inputs[3].Value()); tNumStr != "" {
			if n, err := strconv.Atoi(tNumStr); err == nil {
				req.IndexNumber = &n
			}
		}
		if dNumStr := strings.TrimSpace(m.inputs[4].Value()); dNumStr != "" {
			if n, err := strconv.Atoi(dNumStr); err == nil {
				req.ParentIndexNumber = &n
			}
		}
	} else if len(m.inputs) >= 3 {
		req.Name = strings.TrimSpace(m.inputs[0].Value())
		artistsStr := strings.TrimSpace(m.inputs[1].Value())
		if artistsStr != "" {
			parts := strings.Split(artistsStr, ",")
			var artists []string
			for _, p := range parts {
				if s := strings.TrimSpace(p); s != "" {
					artists = append(artists, s)
				}
			}
			req.Artists = artists
		}
		if yStr := strings.TrimSpace(m.inputs[2].Value()); yStr != "" {
			if n, err := strconv.Atoi(yStr); err == nil {
				req.ProductionYear = &n
			}
		}
	}

	client := m.client
	itemID := m.itemID

	return m, func() tea.Msg {
		if client == nil {
			return SavedMsg{ItemID: itemID, Req: req, Err: fmt.Errorf("client not configured")}
		}
		err := client.UpdateItem(context.Background(), itemID, req)
		return SavedMsg{ItemID: itemID, Req: req, Err: err}
	}
}
