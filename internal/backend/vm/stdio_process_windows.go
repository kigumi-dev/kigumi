//go:build windows

package vm

import "golang.org/x/sys/windows"

// fdClosedAtStart reports whether fd was closed when the process started.
// Unlike Unix, the Windows runtime does not substitute NUL for a closed
// standard handle; GetStdHandle returns 0 and GetFileType then fails with
// ERROR_INVALID_HANDLE, which is the only signal that the stream was closed.
func fdClosedAtStart(fd uintptr) bool {
	if fd == 0 {
		return true
	}
	_, err := windows.GetFileType(windows.Handle(fd))
	return err != nil
}
