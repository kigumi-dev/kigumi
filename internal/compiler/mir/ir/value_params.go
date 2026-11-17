package ir

// BoundTrails reports whether the operands a callable value of op binds come after a call's arguments in its body's
// parameters, as a bind's witnesses do, instead of before them, as captures and a method value's receiver do.
func BoundTrails(op Opcode) bool { return op == OpBind }

// AppendValueParams appends to dst the parameters a body runs with when a value binding bound is called with args.
func AppendValueParams[T any](dst []T, trails bool, bound, args []T, open ...int) []T {
	if trails {
		gap := 0
		if len(open) > 0 {
			gap = open[0]
		}
		at := len(args) - gap
		return append(append(append(dst, args[:at]...), bound...), args[at:]...)
	}
	return append(append(dst, bound...), args...)
}

// ArgParam is the body parameter argument k of a call lands on when the value binds nbound operands.
func ArgParam(trails bool, nbound, k int) int {
	if trails {
		return k
	}
	return nbound + k
}

// BoundParam is the body parameter bound operand j lands on when the call passes nargs arguments.
func BoundParam(trails bool, nargs, j int, open ...int) int {
	if trails {
		if len(open) > 0 {
			nargs -= open[0]
		}
		return nargs + j
	}
	return j
}
