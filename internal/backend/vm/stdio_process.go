package vm

import (
	"os"
	"sync"
	"syscall"
)

// errReader is an io.Reader that fails the same way on every read: a stdin the process closed is unreadable input, not
// an empty one.
type errReader struct{ err error }

func (r errReader) Read([]byte) (int, error) { return 0, r.err }

var (
	stdinClosed  = sync.OnceValue(func() bool { return fdClosedAtStart(os.Stdin.Fd()) })
	stdoutClosed = sync.OnceValue(func() bool { return fdClosedAtStart(os.Stdout.Fd()) })
)

// stdinWasClosed reports whether the process started with fd 0 closed.
func stdinWasClosed() bool { return stdinClosed() }

// stdoutWasClosed reports whether the process started with fd 1 closed.
func stdoutWasClosed() bool { return stdoutClosed() }

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

// writeFailure is the first failed print/eprint of a run: the program has no way to see it, so the run reports it once
// when the entry returns instead of losing output silently.
type writeFailure struct {
	stream string
	err    error
}

func (m *Machine) noteWriteError(stream string, err error) {
	if err == nil {
		return
	}
	m.sh.out.Lock()
	defer m.sh.out.Unlock()
	if m.sh.writeFail == nil {
		m.sh.writeFail = &writeFailure{stream: stream, err: err}
	}
}

// writeFailureOf is the run's first lost output, if any.
func (m *Machine) writeFailureOf() *writeFailure {
	m.sh.out.Lock()
	defer m.sh.out.Unlock()
	return m.sh.writeFail
}
