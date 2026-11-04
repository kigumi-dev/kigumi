package ir

// BoundTrails reports whether the operands a callable value of op binds come after a call's arguments in its body's
// parameters, as a bind's witnesses do, instead of before them, as captures and a method value's receiver do.
func BoundTrails(op Opcode) bool { return op == OpBind }

// AppendValueParams appends to dst the parameters a body runs with when a value binding bound is called with args.
func AppendValueParams[T any](dst []T, trails bool, bound, args []T) []T {
	if trails {
		return append(append(dst, args...), bound...)
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
func BoundParam(trails bool, nargs, j int) int {
	if trails {
		return nargs + j
	}
	return j
}
