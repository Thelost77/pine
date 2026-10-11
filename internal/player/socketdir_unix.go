//go:build !windows

package player

import (
	"fmt"
	"os"
	"syscall"
)

func prepareMpvSocketDir(path string) error {
	if _, err := os.Lstat(path); err != nil {
		if !os.IsNotExist(err) {
			return fmt.Errorf("lstat socket directory: %w", err)
		}
		if err := os.Mkdir(path, 0700); err != nil && !os.IsExist(err) {
			return fmt.Errorf("create socket directory: %w", err)
		}
	}

	fd, err := syscall.Open(path, syscall.O_RDONLY|syscall.O_DIRECTORY|syscall.O_NOFOLLOW, 0)
	if err != nil {
		return fmt.Errorf("open socket directory: %w", err)
	}

	var stat syscall.Stat_t
	err = syscall.Fstat(fd, &stat)
	if err == nil && stat.Uid != uint32(os.Getuid()) {
		err = fmt.Errorf("socket directory is not owned by current user")
	}
	if err == nil && stat.Mode&07777 != 0700 {
		err = syscall.Fchmod(fd, 0700)
	}
	closeErr := syscall.Close(fd)
	if err != nil {
		return fmt.Errorf("validate socket directory: %w", err)
	}
	if closeErr != nil {
		return fmt.Errorf("close socket directory: %w", closeErr)
	}
	return nil
}
