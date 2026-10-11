package main

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"

	"github.com/Thelost77/pine/internal/logger"
	"github.com/Thelost77/pine/internal/spruce/app"
	"github.com/Thelost77/pine/internal/spruce/buildinfo"
	"github.com/Thelost77/pine/internal/spruce/config"
	"github.com/Thelost77/pine/internal/spruce/player"
	tea "github.com/charmbracelet/bubbletea"
)

func main() {
	flag.Usage = func() {
		fmt.Fprintln(flag.CommandLine.Output(), "Usage: spruce [--version]")
		flag.PrintDefaults()
	}
	showVersion := flag.Bool("version", false, "Print version and exit")
	flag.Parse()
	if *showVersion {
		fmt.Printf("spruce %s\n", buildinfo.Current())
		return
	}

	cfgDir := config.ConfigDir()
	closeLog := logger.Init(cfgDir, "spruce")
	defer closeLog()
	logger.Info("starting spruce TUI")

	cfg, err := config.Load(filepath.Join(cfgDir, "config.toml"))
	if err != nil {
		fmt.Fprintf(os.Stderr, "error loading config: %v\n", err)
		os.Exit(1)
	}

	mpv := player.NewMpv()
	rootModel := app.New(&cfg, mpv)

	p := tea.NewProgram(rootModel, tea.WithAltScreen())
	rootModel.SetProgram(p)

	finalModel, err := p.Run()
	if err != nil {
		logger.Error("program exited with error", "err", err)
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
	}

	if m, ok := finalModel.(*app.Model); ok {
		m.Cleanup()
	}
	logger.Info("spruce session ended")
}
