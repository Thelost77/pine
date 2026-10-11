//go:build windows

package logger

import "os"

// Windows builds do not lock the log file.
func lockFile(*os.File) error { return nil }

func unlockFile(*os.File) {}
