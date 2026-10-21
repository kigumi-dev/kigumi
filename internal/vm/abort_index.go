package vm

import "fmt"

// abortIndex is the shared `index out of range` panic; the native runtime prints the same text.
func (m *Machine) abortIndex(i, length int64) {
	msg := fmt.Sprintf("index out of range: index %d, length %d", i, length)
	if site := m.abortSite().Text(); site != "" {
		msg += " at " + site
	}
	m.abort(msg)
}
