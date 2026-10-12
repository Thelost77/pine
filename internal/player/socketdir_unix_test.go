//go:build !windows

package player

import (
	"os"
	"path/filepath"
	"testing"
)

func TestPrepareMpvSocketDirCreatesPrivateDirectory(t *testing.T) {
	path := filepath.Join(t.TempDir(), "socket")

	if err := prepareMpvSocketDir(path); err != nil {
		t.Fatalf("prepareMpvSocketDir() error: %v", err)
	}
	info, err := os.Stat(path)
	if err != nil {
		t.Fatalf("stat socket directory: %v", err)
	}
	if !info.IsDir() {
		t.Fatal("expected socket path to be a directory")
	}
	if info.Mode().Perm()&0077 != 0 {
		t.Fatalf("socket directory mode = %o, want no group or other permissions", info.Mode().Perm())
	}
}

func TestPrepareMpvSocketDirRepairsSameUserPermissions(t *testing.T) {
	path := filepath.Join(t.TempDir(), "socket")
	if err := os.Mkdir(path, 0755); err != nil {
		t.Fatalf("create socket directory: %v", err)
	}

	if err := prepareMpvSocketDir(path); err != nil {
		t.Fatalf("prepareMpvSocketDir() error: %v", err)
	}
	info, err := os.Stat(path)
	if err != nil {
		t.Fatalf("stat socket directory: %v", err)
	}
	if got := info.Mode().Perm(); got != 0700 {
		t.Fatalf("socket directory mode = %o, want 700", got)
	}
}

func TestPrepareMpvSocketDirRejectsFileAndSymlink(t *testing.T) {
	root := t.TempDir()
	file := filepath.Join(root, "file")
	if err := os.WriteFile(file, nil, 0600); err != nil {
		t.Fatalf("create file: %v", err)
	}
	if err := prepareMpvSocketDir(file); err == nil {
		t.Fatal("expected file to be rejected")
	}

	target := filepath.Join(root, "target")
	if err := os.Mkdir(target, 0700); err != nil {
		t.Fatalf("create target directory: %v", err)
	}
	link := filepath.Join(root, "link")
	if err := os.Symlink(target, link); err != nil {
		t.Fatalf("create symlink: %v", err)
	}
	if err := prepareMpvSocketDir(link); err == nil {
		t.Fatal("expected symlink to be rejected")
	}
}
