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

// maxLogSize is the size above which Init renames the log to <name>.log.old.
const maxLogSize = 5 * 1024 * 1024

var (
	instance *slog.Logger
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

		f, err := openLog(path)
		if err != nil {
			return
		}
		locked := lockFile(f) == nil
		// Rotate only while this process holds the lock, so that the log of a
		// running instance keeps its name. Windows cannot rename an open file,
		// so the file closes first.
		info, err := f.Stat()
		if locked && err == nil && info.Size() > maxLogSize {
			_ = f.Close()
			_ = os.Rename(path, path+".old")
			if f, err = openLog(path); err != nil {
				return
			}
			locked = lockFile(f) == nil
		}
		if !locked {
			_ = f.Close()
			path = filepath.Join(dir, fmt.Sprintf("%s-%d.log", name, os.Getpid()))
			logPath = path
			if f, err = openLog(path); err != nil {
				return
			}
			_ = lockFile(f)
		}
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

// openLog opens the log file at path for appending, with the mode 0600.
func openLog(path string) (*os.File, error) {
	f, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o600)
	if err != nil {
		return nil, err
	}
	_ = os.Chmod(path, 0o600)
	return f, nil
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
