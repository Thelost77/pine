package main

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/Thelost77/pine/internal/abs"
	"github.com/Thelost77/pine/internal/app"
	"github.com/Thelost77/pine/internal/cache"
	"github.com/Thelost77/pine/internal/config"
	"github.com/Thelost77/pine/internal/db"
	"github.com/Thelost77/pine/internal/logger"
	tea "github.com/charmbracelet/bubbletea"
)

func main() {
	closeLog := logger.Init()
	defer closeLog()
	logger.Session()
	logger.Info("log file", "path", logger.Path())

	cfgDir := config.ConfigDir()
	cfg, err := config.Load(filepath.Join(cfgDir, "config.toml"))
	if err != nil {
		fmt.Fprintf(os.Stderr, "error loading config: %v\n", err)
		os.Exit(1)
	}

	dbPath := filepath.Join(cfgDir, "pine.db")
	store, err := db.Open(dbPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error opening database: %v\n", err)
		os.Exit(1)
	}
	defer store.Close()

	var absClient *abs.Client
	if acct, err := store.GetDefaultAccount(); err == nil && acct.Token != "" {
		serverURL := acct.ServerURL
		if serverURL == "" {
			serverURL = cfg.Server.Address
		}
		absClient = abs.NewClient(serverURL, acct.Token)
	}
	cacheStore := cache.NewStore(store)
	var cachedClient *cache.Client
	if absClient != nil {
		cachedClient = cache.NewClient(absClient, cacheStore)
	}

	model := app.New(cfg, store, cachedClient, cacheStore)
	p := tea.NewProgram(model, tea.WithAltScreen())
	model.SetProgram(p)
	logger.Info("starting TUI")
	finalModel, err := p.Run()
	if err != nil {
		logger.Error("program exited with error", "err", err)
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
	}

	if m, ok := finalModel.(app.Model); ok {
		m.Cleanup()
	}
	logger.Info("session ended")
}
