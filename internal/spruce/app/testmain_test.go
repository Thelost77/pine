package app

import (
	"os"
	"testing"
)

func TestMain(m *testing.M) {
	dir, err := os.MkdirTemp("", "spruce-app-test-config-*")
	if err != nil {
		panic(err)
	}

	oldConfigHome, hadConfigHome := os.LookupEnv("XDG_CONFIG_HOME")
	if err := os.Setenv("XDG_CONFIG_HOME", dir); err != nil {
		panic(err)
	}
	code := m.Run()
	if hadConfigHome {
		_ = os.Setenv("XDG_CONFIG_HOME", oldConfigHome)
	} else {
		_ = os.Unsetenv("XDG_CONFIG_HOME")
	}
	_ = os.RemoveAll(dir)
	os.Exit(code)
}
