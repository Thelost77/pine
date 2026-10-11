//go:build windows

package player

import "os"

// Windows builds create the directory without the owner and mode checks.
func prepareMpvSocketDir(path string) error {
	return os.MkdirAll(path, 0o700)
}
