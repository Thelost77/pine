// Package logger provides file-based structured logging for pine.
// Logs are written to <configDir>/pine.log with automatic rotation.
package logger

import (
	"fmt"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"sync"

	"github.com/Thelost77/pine/internal/config"
)

var (
	instance *slog.Logger
	logFile  *os.File
	once     sync.Once
	nop      = slog.New(slog.NewTextHandler(io.Discard, nil))
)

// Init initializes the file logger. Safe to call multiple times; only the first call takes effect.
// Returns a cleanup function that closes the log file.
func Init() func() {
	var cleanup func()
	once.Do(func() {
		dir := config.ConfigDir()
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return
		}
		path := filepath.Join(dir, "pine.log")

		// Rotate: if log exceeds 5MB, truncate
		if info, err := os.Stat(path); err == nil && info.Size() > 5*1024*1024 {
			_ = os.Rename(path, path+".old")
		}

		f, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o644)
		if err != nil {
			return
		}
		logFile = f
		instance = slog.New(slog.NewTextHandler(f, &slog.HandlerOptions{
			Level: slog.LevelDebug,
		}))
		cleanup = func() { f.Close() }
	})
	if cleanup == nil {
		cleanup = func() {}
	}
	return cleanup
}

// Get returns the logger instance. Returns a no-op logger if Init hasn't been called.
func Get() *slog.Logger {
	if instance != nil {
		return instance
	}
	return nop
}

// Log path for user reference.
func Path() string {
	return filepath.Join(config.ConfigDir(), "pine.log")
}

// Convenience methods that use the global instance.

func Debug(msg string, args ...any) { Get().Debug(msg, args...) }
func Info(msg string, args ...any)  { Get().Info(msg, args...) }
func Warn(msg string, args ...any)  { Get().Warn(msg, args...) }
func Error(msg string, args ...any) { Get().Error(msg, args...) }

// Session logs a separator with session start info.
func Session() {
	Get().Info(fmt.Sprintf("=== pine session started (pid %d) ===", os.Getpid()))
}
