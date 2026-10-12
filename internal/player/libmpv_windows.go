//go:build windows

package player

import "golang.org/x/sys/windows"

// openLibrary looks only in the directory of the program and in the system
// directory. The default search of Windows also includes the current
// directory, where another user can put a library.
func openLibrary(name string) (uintptr, error) {
	library, err := windows.LoadLibraryEx(name, 0,
		windows.LOAD_LIBRARY_SEARCH_APPLICATION_DIR|windows.LOAD_LIBRARY_SEARCH_SYSTEM32)
	return uintptr(library), err
}

// libmpvCandidates returns the library names to try, in order.
func libmpvCandidates() []string {
	return []string{"libmpv-2.dll", "mpv-2.dll"}
}

func libmpvInstallHint() string {
	return "put libmpv-2.dll next to the program"
}
