package main

import (
	"fmt"
	"os"
	"path/filepath"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/Thelost77/pine/internal/abs"
	"github.com/Thelost77/pine/internal/app"
	"github.com/Thelost77/pine/internal/config"
	"github.com/Thelost77/pine/internal/db"
	"github.com/Thelost77/pine/internal/logger"
)

func main() {
	// Initialize file logger
	closeLog := logger.Init()
	defer closeLog()
	logger.Session()
	logger.Info("log file", "path", logger.Path())

	// Load config
	cfgDir := config.ConfigDir()
	cfg, err := config.Load(filepath.Join(cfgDir, "config.toml"))
	if err != nil {
		fmt.Fprintf(os.Stderr, "error loading config: %v\n", err)
		os.Exit(1)
	}

	// Open database
	dbPath := filepath.Join(cfgDir, "pine.db")
	store, err := db.Open(dbPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error opening database: %v\n", err)
		os.Exit(1)
	}
	defer store.Close()

	// Check for stored credentials
	var client *abs.Client
	if acct, err := store.GetDefaultAccount(); err == nil && acct.Token != "" {
		serverURL := acct.ServerURL
		if serverURL == "" {
			serverURL = cfg.Server.Address
		}
		client = abs.NewClient(serverURL, acct.Token)
	}

	model := app.New(cfg, store, client)
	p := tea.NewProgram(model, tea.WithAltScreen())
	logger.Info("starting TUI")
	finalModel, err := p.Run()
	if err != nil {
		logger.Error("program exited with error", "err", err)
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
	}

	// Run cleanup on any exit path (Ctrl+C, q, tea.Quit, etc.)
	if m, ok := finalModel.(app.Model); ok {
		m.Cleanup()
	}
	logger.Info("session ended")
}
