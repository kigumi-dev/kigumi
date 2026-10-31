package vm

import (
	"fmt"

	midir "kigumi/internal/compiler/mir/ir"
)

// maxCallDepth bounds nested Kigumi calls outside a sandbox; Go's 1 GB
// goroutine stack dies fatally near 2.5 million simple frames.
const maxCallDepth = 100_000

func (m *Machine) checkDepth(f *midir.Func) {
	if m.opts.Sandbox {
		if m.depth > m.opts.DepthLimit {
			m.limitAbort(fmt.Sprintf("call depth limit of %d nested calls", m.opts.DepthLimit))
		}
		return
	}
	if m.depth > maxCallDepth {
		m.abort("stack overflow in " + f.Name)
	}
}
