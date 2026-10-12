//go:build !windows

package player

import (
	"runtime"

	"github.com/ebitengine/purego"
)

func openLibrary(name string) (uintptr, error) {
	return purego.Dlopen(name, purego.RTLD_NOW|purego.RTLD_GLOBAL)
}

// libmpvCandidates returns the library names to try, in order.
func libmpvCandidates() []string {
	if runtime.GOOS == "darwin" {
		// The Homebrew and MacPorts paths. A bare name is not in the list:
		// its search does not include the Homebrew directory of Apple
		// Silicon, and it does include the current directory.
		return []string{
			"/opt/homebrew/lib/libmpv.2.dylib",
			"/usr/local/lib/libmpv.2.dylib",
			"/opt/local/lib/libmpv.2.dylib",
		}
	}
	return []string{"libmpv.so.2", "libmpv.so.1"}
}

func libmpvInstallHint() string {
	if runtime.GOOS == "darwin" {
		return "install mpv with Homebrew (brew install mpv)"
	}
	return "install libmpv2 (Debian, Ubuntu) or mpv (Arch)"
}
