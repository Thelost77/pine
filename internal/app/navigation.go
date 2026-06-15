package app

import (
	"github.com/Thelost77/pine/internal/logger"
	tea "github.com/charmbracelet/bubbletea"
)

// navigate pushes the current screen onto the back stack and switches.
func (m Model) navigate(target Screen) (Model, tea.Cmd) {
	logger.Debug("screen transition", "from", m.screen, "to", target, "backStackDepth", len(m.backStack)+1)
	m.backStack = append(m.backStack, m.screen)
	m.screen = target
	m.propagateSize()
	cmd := m.initScreen(target)
	return m, cmd
}

// back pops the back stack. No-op if empty.
func (m Model) back() (Model, tea.Cmd) {
	if len(m.backStack) == 0 {
		logger.Debug("back navigation ignored", "screen", m.screen)
		return m, nil
	}
	last := m.backStack[len(m.backStack)-1]
	m.backStack = m.backStack[:len(m.backStack)-1]
	logger.Debug("screen transition", "from", m.screen, "to", last, "backStackDepth", len(m.backStack))
	m.screen = last
	m.propagateSize()
	return m, nil
}

// ActiveScreen returns the currently active screen.
func (m Model) ActiveScreen() Screen {
	return m.screen
}

// BackStack returns a copy of the current back stack.
func (m Model) BackStack() []Screen {
	cp := make([]Screen, len(m.backStack))
	copy(cp, m.backStack)
	return cp
}

// screenHeight returns the available height for screen content.
func (m Model) screenHeight() int {
	h := m.height - headerHeight
	if m.err.HasError() {
		h -= errorBannerHeight
	}
	if m.isPlaying() {
		h -= playerFooterHeight
	}
	if h < 0 {
		return 0
	}
	return h
}

// propagateSize updates sub-model dimensions.
func (m *Model) propagateSize() {
	w := normalizeViewWidth(m.width)
	sh := m.screenHeight()
	m.login.SetSize(w, sh)
	m.home.SetSize(w, sh)
	m.library.SetSize(w, sh)
	m.detail.SetSize(w, sh)
	m.metadataEdit.SetSize(w, sh)
	m.seriesList.SetSize(w, sh)
	m.series.SetSize(w, sh)
	m.palette.SetSize(m.width, m.height)
}

// initScreen returns the Init command for a given screen.
func (m Model) initScreen(s Screen) tea.Cmd {
	switch s {
	case ScreenLogin:
		return m.login.Init()
	case ScreenHome:
		return m.home.Init()
	case ScreenLibrary:
		return m.library.Init()
	case ScreenDetail:
		return m.detail.Init()
	case ScreenMetadataEdit:
		return m.metadataEdit.Init()
	case ScreenSeriesList:
		return m.seriesList.Init()
	case ScreenSeries:
		return m.series.Init()
	default:
		return nil
	}
}

// updateScreen dispatches an update to the currently active screen.
func (m Model) updateScreen(msg tea.Msg) (Model, tea.Cmd) {
	var cmd tea.Cmd
	switch m.screen {
	case ScreenLogin:
		m.login, cmd = m.login.Update(msg)
	case ScreenHome:
		m.home, cmd = m.home.Update(msg)
	case ScreenLibrary:
		m.library, cmd = m.library.Update(msg)
	case ScreenDetail:
		m.detail, cmd = m.detail.Update(msg)
	case ScreenMetadataEdit:
		m.metadataEdit, cmd = m.metadataEdit.Update(msg)
	case ScreenSeriesList:
		m.seriesList, cmd = m.seriesList.Update(msg)
	case ScreenSeries:
		m.series, cmd = m.series.Update(msg)
	}
	return m, cmd
}

