package logger

import (
	"os"
	"path/filepath"
	"testing"
)

func TestInitCreatesPrivateLogFileNamedAfterApp(t *testing.T) {
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
	if perm := info.Mode().Perm(); perm != 0o600 {
		t.Errorf("log file mode = %o, want 600", perm)
	}
	dirInfo, err := os.Stat(dir)
	if err != nil {
		t.Fatalf("stat log directory: %v", err)
	}
	if perm := dirInfo.Mode().Perm(); perm != 0o700 {
		t.Errorf("log directory mode = %o, want 700", perm)
	}
}
