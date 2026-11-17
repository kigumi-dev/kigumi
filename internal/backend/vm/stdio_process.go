package vm

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
