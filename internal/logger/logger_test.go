package logger

import (
	"os"
	"path/filepath"
	"runtime"
	"sync"
	"testing"
)

// resetForTest clears the logger state so that a test can call Init again.
func resetForTest(t *testing.T) {
	t.Helper()
	reset := func() {
		once = sync.Once{}
		instance = nil
		logPath = ""
		appName = ""
	}
	reset()
	t.Cleanup(reset)
}

func TestInitCreatesPrivateLogFileNamedAfterApp(t *testing.T) {
	resetForTest(t)
	dir := filepath.Join(t.TempDir(), "config")
	closeLog := Init(dir, "pine")
	defer closeLog()

	path := filepath.Join(dir, "pine.log")
	if got := Path(); got != path {
		t.Fatalf("Path() = %q, want %q", got, path)
	}
	info, err := os.Stat(path)
	if err != nil {
		t.Fatalf("stat log file: %v", err)
	}
	dirInfo, err := os.Stat(dir)
	if err != nil {
		t.Fatalf("stat log directory: %v", err)
	}
	// Windows has no Unix file modes.
	if runtime.GOOS == "windows" {
		return
	}
	if perm := info.Mode().Perm(); perm != 0o600 {
		t.Errorf("log file mode = %o, want 600", perm)
	}
	if perm := dirInfo.Mode().Perm(); perm != 0o700 {
		t.Errorf("log directory mode = %o, want 700", perm)
	}
}

func TestInitRotatesALargeLog(t *testing.T) {
	resetForTest(t)
	dir := t.TempDir()
	path := filepath.Join(dir, "pine.log")
	if err := os.WriteFile(path, nil, 0o600); err != nil {
		t.Fatalf("create the log file: %v", err)
	}
	if err := os.Truncate(path, maxLogSize+1); err != nil {
		t.Fatalf("grow the log file: %v", err)
	}

	closeLog := Init(dir, "pine")
	defer closeLog()

	if got := Path(); got != path {
		t.Errorf("Path() = %q, want %q", got, path)
	}
	if info, err := os.Stat(path + ".old"); err != nil || info.Size() != maxLogSize+1 {
		t.Errorf("stat %s.old = %v, %v; want the previous log", path, info, err)
	}
	if info, err := os.Stat(path); err != nil || info.Size() != 0 {
		t.Errorf("stat %s = %v, %v; want a new empty log", path, info, err)
	}
}
