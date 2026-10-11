//go:build !windows

package logger

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestPathReportsTheFallbackFileWhenTheLogIsLocked(t *testing.T) {
	resetForTest(t)
	dir := t.TempDir()
	// A second process of the same app holds the lock on the log file.
	holder, err := os.OpenFile(filepath.Join(dir, "pine.log"), os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o600)
	if err != nil {
		t.Fatalf("open the log file: %v", err)
	}
	defer holder.Close()
	if err := lockFile(holder); err != nil {
		t.Fatalf("lock the log file: %v", err)
	}
	defer unlockFile(holder)

	closeLog := Init(dir, "pine")
	defer closeLog()
	Info("fallback entry")

	want := filepath.Join(dir, fmt.Sprintf("pine-%d.log", os.Getpid()))
	if got := Path(); got != want {
		t.Errorf("Path() = %q, want the fallback file %q", got, want)
	}
	data, err := os.ReadFile(want)
	if err != nil {
		t.Fatalf("read the fallback file: %v", err)
	}
	if !strings.Contains(string(data), "fallback entry") {
		t.Errorf("the fallback file does not contain the log entry: %q", data)
	}
}
