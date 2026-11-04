package llgen

import (
	"strings"

	midir "kigumi/internal/compiler/mir/ir"
)

// paramSlot is where an adapter takes a body parameter from: the environment or the argument array, at index.
type paramSlot struct {
	env   bool
	index int
}

// paramSlots lays out nparams body parameters over nbound environment slots and the argument array, as the value's
// call runs its body.
func paramSlots(trails bool, nbound, nparams int) []paramSlot {
	env := make([]paramSlot, nbound)
	for i := range env {
		env[i] = paramSlot{env: true, index: i}
	}
	args := make([]paramSlot, max(nparams-nbound, 0))
	for i := range args {
		args[i] = paramSlot{index: i}
	}
	return midir.AppendValueParams(nil, trails, env, args)
}

// slotArg loads s from %env or %args into v.
func (e *emitter) slotArg(sb *strings.Builder, v string, s paramSlot) {
	if s.env {
		e.irb(sb).CallAs(v, rtAt, vptr("%env"), vi64(s.index))
		return
	}
	e.irb(sb).CallAs(v, rtAt, vptr("%args"), vi64(s.index))
}
