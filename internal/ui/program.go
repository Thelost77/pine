package ui

import (
	"os"
	"os/signal"
	"syscall"

	"github.com/Thelost77/pine/internal/logger"
	tea "github.com/charmbracelet/bubbletea"
)

// RunProgram runs the program until it quits, and returns its final model. It
// quits the program when the process gets SIGHUP, which is the close of its
// terminal. Without that, the process ends at the signal, and the app does
// not do its cleanup after the program. Bubble Tea quits the program at
// SIGTERM.
func RunProgram(p *tea.Program) (tea.Model, error) {
	hangup := make(chan os.Signal, 1)
	signal.Notify(hangup, syscall.SIGHUP)
	go func() {
		<-hangup
		logger.Info("terminal closed, quitting")
		p.Quit()
	}()
	return p.Run()
}
