package app

import (
	tea "github.com/charmbracelet/bubbletea"
)

// navigate pushes the current screen onto the back stack and switches.
func (m *Model) navigate(target Screen) (*Model, tea.Cmd) {
	m.backStack = append(m.backStack, m.screen)
	m.screen = target
	m.propagateSize()
	return m, m.initScreen(target)
}

// back pops the back stack. No-op if empty.
func (m *Model) back() (*Model, tea.Cmd) {
	if len(m.backStack) == 0 {
		return m, nil
	}
	last := m.backStack[len(m.backStack)-1]
	m.backStack = m.backStack[:len(m.backStack)-1]
	m.screen = last
	m.propagateSize()
	return m, nil
}

// propagateSize updates sub-model dimensions after a screen transition.
func (m *Model) propagateSize() {
	w := m.width
	sh := m.screenHeight()
	m.loginScreen.SetSize(w, sh)
	m.libraryScreen.SetSize(w, sh)
	m.playlistsScreen.SetSize(w, sh)
	m.queueScreen.SetSize(w, sh)
	m.metadataEditScreen.SetSize(w, sh)
	m.palette.SetSize(m.width, m.height)
}

// initScreen returns the Init command for a given screen.
func (m *Model) initScreen(s Screen) tea.Cmd {
	switch s {
	case ScreenLogin:
		return m.loginScreen.Init()
	case ScreenLibrary:
		return m.libraryScreen.Init()
	case ScreenPlaylists:
		return m.playlistsScreen.Init()
	case ScreenQueue:
		return m.queueScreen.Init()
	case ScreenMetadataEdit:
		return m.metadataEditScreen.Init()
	}
	return nil
}
