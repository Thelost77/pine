package library

import (
	"fmt"

	"github.com/charmbracelet/lipgloss"
)

func (m Model) View() string {
	if m.err != nil {
		errStr := m.styles.Error.Render(fmt.Sprintf("error loading library: %v", m.err))
		return lipgloss.Place(m.width, m.height, lipgloss.Center, lipgloss.Center, errStr)
	}

	switch m.level {
	case LevelTracks:
		return m.trackList.View()
	default:
		return m.albumList.View()
	}
}

