package sem

import "slices"

// ctxStackFind returns a key's binding and the lambda depth its frame was
// pushed at, innermost frame first.
func ctxStackFind(stack []ctxFrame, key EntityID) (ctxBindSource, int, bool) {
	for i := len(stack) - 1; i >= 0; i-- {
		if s, ok := stack[i].bind[key]; ok {
			return s, stack[i].depth, true
		}
	}
	return ctxBindSource{}, 0, false
}

// ctxCaptureChain adds local as a capture of every closure in chain, so a
// provider bound outside a closure still reaches a call made inside it.
func (r *Result) ctxCaptureChain(chain []EntityID, local EntityID) {
	for _, cl := range chain {
		info := r.closure(cl)
		if slices.ContainsFunc(info.Captures, func(c Capture) bool { return c.Local == local }) {
			continue
		}
		mode := capMove
		if r.isCopy(r.Entities[local].Type) {
			mode = capCopy
		}
		if info.Immediate && mode == capMove {
			mode = capBorrow
		}
		if mode == capCopy && r.Entities[local].Flags&EfCapturedMut != 0 {
			mode = capCell
		}
		info.Captures = append(info.Captures, Capture{Local: local, Mode: mode})
		if mode != capCopy {
			info.Copy = false
		}
	}
}
