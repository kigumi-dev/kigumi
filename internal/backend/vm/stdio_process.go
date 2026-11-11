package vm

import (
	"os"
	"sync"
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

// devNullID, devNullIdentity and fdClosedAtStart are platform-specific: Unix
// identifies the runtime's /dev/null placeholder, while Windows can simply ask
// whether the std handle is valid.

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
