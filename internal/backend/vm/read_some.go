package vm

import (
	"io"
	"os"
)

// readSome waits up to timeoutMs for stdin to have data, then makes one
// Read call and returns what it got: nil with no error on timeout, io.EOF
// when stdin is closed. It waits on the raw descriptor (poll(2) on Unix, a
// waitable handle on Windows) instead of a goroutine race, so a timeout never
// strands a blocked read that a later call would collide with; a source that
// isn't a file (only possible in tests) skips the wait and reads directly.
func (m *Machine) readSome(timeoutMs int64) ([]byte, error) {
	m.sh.in.Lock()
	defer m.sh.in.Unlock()
	r := m.stdinSource()
	if f, ok := r.(*os.File); ok {
		ready, err := pollReadable(int(f.Fd()), timeoutMs)
		if err != nil {
			return nil, err
		}
		if !ready {
			return nil, nil
		}
	}
	buf := make([]byte, 65536)
	n, err := r.Read(buf)
	if err != nil {
		if err == io.EOF {
			return nil, io.EOF
		}
		return nil, err
	}
	return buf[:n], nil
}

// pollReadable is platform-specific: poll(2) on Unix, a waitable handle on
// Windows.

func (m *Machine) stdinAll() ([]byte, error) {
	m.sh.in.Lock()
	defer m.sh.in.Unlock()
	r := m.stdinSource()
	var out []byte
	buf := make([]byte, 4096)
	for {
		n, err := r.Read(buf)
		out = append(out, buf[:n]...)
		if err == io.EOF {
			return out, nil
		}
		if err != nil {
			return out, err
		}
	}
}

// stdinLine reads one line including its '\n', one byte at a time so it never
// consumes bytes a later spawned child reads from the same descriptor. Native
// readLine reads one byte per call for the same reason.
func (m *Machine) stdinLine() (string, error) {
	m.sh.in.Lock()
	defer m.sh.in.Unlock()
	r := m.stdinSource()
	var line []byte
	var b [1]byte
	for {
		n, err := r.Read(b[:])
		if n > 0 {
			line = append(line, b[0])
			if b[0] == '\n' {
				return string(line), nil
			}
		}
		if err != nil {
			return string(line), err
		}
	}
}

func (m *Machine) stdinRead(buf []byte) (int, error) {
	m.sh.in.Lock()
	defer m.sh.in.Unlock()
	return m.stdinSource().Read(buf)
}
