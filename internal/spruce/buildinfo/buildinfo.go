package buildinfo

import (
	"runtime/debug"
	"strings"
)

// Current returns the installed module version or dev for a local build.
func Current() string {
	if info, ok := debug.ReadBuildInfo(); ok {
		return normalize(info.Main.Version)
	}
	return "dev"
}

func normalize(version string) string {
	version = strings.TrimSpace(version)
	if version == "" || version == "(devel)" {
		return "dev"
	}
	return version
}
