//go:build linux || darwin || freebsd || netbsd || openbsd

package vm

import (
	"os"
	"sync"
	"syscall"
)

// devNullID is /dev/null's identity. The probe uses syscall.Fstat rather than os.File so nothing registers a finalizer
// that could close fd 0, 1 or 2.
type devNullID struct{ dev, ino uint64 }

var (
	devNullOnce sync.Once
	devNull     devNullID
	devNullOK   bool
)

func devNullIdentity() (devNullID, bool) {
	devNullOnce.Do(func() {
		var st syscall.Stat_t
		if err := syscall.Stat(os.DevNull, &st); err != nil {
			return
		}
		devNull, devNullOK = devNullID{dev: uint64(st.Dev), ino: uint64(st.Ino)}, true
	})
	return devNull, devNullOK
}

// fdClosedAtStart reports whether fd was closed when the process started. The Go runtime replaces a closed 0/1/2 with
// /dev/null before main runs, so the descriptor alone no longer says which happened: the placeholder is the only
// /dev/null opened O_RDWR, where a shell redirection opens it O_RDONLY.
func fdClosedAtStart(fd uintptr) bool {
	var st syscall.Stat_t
	if err := syscall.Fstat(int(fd), &st); err != nil {
		// Fstat failing means nothing is open on fd; the runtime only substitutes what it could open.
		return true
	}
	null, ok := devNullIdentity()
	if !ok || uint64(st.Dev) != null.dev || uint64(st.Ino) != null.ino {
		return false
	}
	flags, _, errno := syscall.Syscall(syscall.SYS_FCNTL, fd, syscall.F_GETFL, 0)
	if errno != 0 {
		return false
	}
	return int(flags)&syscall.O_ACCMODE == syscall.O_RDWR
}
