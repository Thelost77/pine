//go:build windows

package player

import "syscall"

func openLibrary(name string) (uintptr, error) {
	library, err := syscall.LoadLibrary(name)
	return uintptr(library), err
}

// libmpvCandidates returns the library names to try, in order. Windows looks
// in the directory of the program first.
func libmpvCandidates() []string {
	return []string{"libmpv-2.dll", "mpv-2.dll"}
}

func libmpvInstallHint() string {
	return "put libmpv-2.dll next to the program"
}
