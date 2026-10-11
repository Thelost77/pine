// Package logger provides file-based structured logging for spruce.
// Logs are written to <configDir>/spruce.log with automatic rotation.
package logger

import (
	"fmt"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"sync"
	"syscall"

	"github.com/Thelost77/pine/internal/spruce/config"
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
		if err := os.MkdirAll(dir, 0o700); err != nil {
			return
		}
		path := filepath.Join(dir, "spruce.log")

		// Rotate: if log exceeds 5MB, truncate
		if info, err := os.Stat(path); err == nil && info.Size() > 5*1024*1024 {
			_ = os.Rename(path, path+".old")
		}

		f, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o600)
		if err != nil {
			return
		}
		_ = os.Chmod(path, 0o600)
		if err := syscall.Flock(int(f.Fd()), syscall.LOCK_EX|syscall.LOCK_NB); err != nil {
			_ = f.Close()
			path = filepath.Join(dir, fmt.Sprintf("spruce-%d.log", os.Getpid()))
			f, err = os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o600)
			if err != nil {
				return
			}
			_ = os.Chmod(path, 0o600)
			_ = syscall.Flock(int(f.Fd()), syscall.LOCK_EX|syscall.LOCK_NB)
		}
		logFile = f
		instance = slog.New(slog.NewTextHandler(f, &slog.HandlerOptions{
			Level: slog.LevelDebug,
		}))
		cleanup = func() {
			_ = syscall.Flock(int(f.Fd()), syscall.LOCK_UN)
			_ = f.Close()
		}
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
	return filepath.Join(config.ConfigDir(), "spruce.log")
}

// Convenience methods that use the global instance.

func Debug(msg string, args ...any) { Get().Debug(msg, args...) }
func Info(msg string, args ...any)  { Get().Info(msg, args...) }
func Warn(msg string, args ...any)  { Get().Warn(msg, args...) }
func Error(msg string, args ...any) { Get().Error(msg, args...) }

// Session logs a separator with session start info.
func Session() {
	Get().Info(fmt.Sprintf("=== spruce session started (pid %d) ===", os.Getpid()))
}
