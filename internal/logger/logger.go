// Package logger provides file-based structured logging.
// Logs are written to <dir>/<name>.log with automatic rotation.
package logger

import (
	"fmt"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"sync"
)

var (
	instance *slog.Logger
	logFile  *os.File
	logPath  string
	appName  string
	once     sync.Once
	nop      = slog.New(slog.NewTextHandler(io.Discard, nil))
)

// Init initializes the file logger for the app name in dir. Safe to call multiple times; only the first call takes effect.
// Returns a cleanup function that closes the log file.
func Init(dir, name string) func() {
	var cleanup func()
	once.Do(func() {
		appName = name
		path := filepath.Join(dir, name+".log")
		logPath = path
		if err := os.MkdirAll(dir, 0o700); err != nil {
			return
		}

		// Rotate: if log exceeds 5MB, truncate
		if info, err := os.Stat(path); err == nil && info.Size() > 5*1024*1024 {
			_ = os.Rename(path, path+".old")
		}

		f, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o600)
		if err != nil {
			return
		}
		_ = os.Chmod(path, 0o600)
		if err := lockFile(f); err != nil {
			_ = f.Close()
			path = filepath.Join(dir, fmt.Sprintf("%s-%d.log", name, os.Getpid()))
			logPath = path
			f, err = os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o600)
			if err != nil {
				return
			}
			_ = os.Chmod(path, 0o600)
			_ = lockFile(f)
		}
		logFile = f
		instance = slog.New(slog.NewTextHandler(f, &slog.HandlerOptions{
			Level: slog.LevelDebug,
		}))
		cleanup = func() {
			unlockFile(f)
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
	return logPath
}

// Convenience methods that use the global instance.

func Debug(msg string, args ...any) { Get().Debug(msg, args...) }
func Info(msg string, args ...any)  { Get().Info(msg, args...) }
func Warn(msg string, args ...any)  { Get().Warn(msg, args...) }
func Error(msg string, args ...any) { Get().Error(msg, args...) }

// Session logs a separator with session start info.
func Session() {
	Get().Info(fmt.Sprintf("=== %s session started (pid %d) ===", appName, os.Getpid()))
}
