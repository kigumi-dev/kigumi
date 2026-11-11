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
	r := m.stdinReader()
	if r.Buffered() == 0 {
		if f, ok := m.sh.stdinSrc.(*os.File); ok {
			ready, err := pollReadable(int(f.Fd()), timeoutMs)
			if err != nil {
				return nil, err
			}
			if !ready {
				return nil, nil
			}
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
	r := m.stdinReader()
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

func (m *Machine) stdinLine() (string, error) {
	m.sh.in.Lock()
	defer m.sh.in.Unlock()
	return m.stdinReader().ReadString('\n')
}

func (m *Machine) stdinRead(buf []byte) (int, error) {
	m.sh.in.Lock()
	defer m.sh.in.Unlock()
	return m.stdinReader().Read(buf)
}
