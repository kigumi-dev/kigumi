package vm

import "kigumi/internal/mir"

// maxCallDepth bounds nested Kigumi calls outside a sandbox; Go's 1 GB
// goroutine stack dies fatally near 2.5 million simple frames.
const maxCallDepth = 100_000

func (m *Machine) checkDepth(f *mir.Func) {
	if m.opts.Sandbox {
		if m.depth > m.opts.DepthLimit {
			m.sandboxAbort("comptime recursion too deep")
		}
		return
	}
	if m.depth > maxCallDepth {
		m.abort("stack overflow in " + f.Name)
	}
}
