package ownership

import (
	midir "kigumi/internal/compiler/mir/ir"
	"kigumi/internal/compiler/sem"
)

// PrimitiveTakes: a runtime primitive retains what it keeps, so a field place it gets by value is spent like a moved local.
func PrimitiveTakes(r *sem.Result, in *midir.Inst, i int) bool {
	if in.Op != midir.OpCall || !IsStdPrimitive(r, in.Ent) {
		return false
	}
	recv := r.Fn(in.Ent).Recv
	return i != 0 || recv == sem.RecvNone || recv == sem.RecvMove
}

// ConsumesArg reports whether argument i hands ownership over. It mirrors the
// backends' lending rule: a `self`/`mut self` receiver, the callee of an
// indirect call and every argument of a runtime primitive are lent, not
// moved, except `await`'s future (it runs the future's body and discards it)
// and a resource's own registered `drop`, which always consumes `self`.
func ConsumesArg(r *sem.Result, bodies map[sem.EntityID]*midir.Func, in *midir.Inst, i int) bool {
	switch in.Op {
	case midir.OpCall:
		e := r.Entity(in.Ent)
		if e.Kind != sem.EntFn {
			return false
		}
		info := r.Fn(in.Ent)
		if i == 0 && IsOwnDrop(r, in.Ent, info) {
			return true
		}
		if info.Body == 0 && info.Abi == "" && e.Flags&sem.EfStd != 0 {
			return false
		}
		// C code releases nothing: every argument of a foreign call is lent.
		if info.Abi != "" || info.Naked {
			return false
		}
		// A trailing hidden provider argument forwards a lease; it never
		// changes hands.
		if i >= len(in.Args)-ctxArgCount(info) {
			return false
		}
		if g := bodies[in.Ent]; g != nil && i < len(g.Params) && g.Locals[g.Params[i]].Borrowed {
			return false
		}
		return i != 0 || info.Recv == sem.RecvNone || info.Recv == sem.RecvMove
	case midir.OpCallValue:
		// A witness call names its requirement: a `self` / `mut self`
		// receiver is lent, as in a direct method call.
		if in.Ent != 0 && i == 1 {
			recv := r.Fn(in.Ent).Recv
			return recv == sem.RecvNone || recv == sem.RecvMove
		}
		return i != 0
	case midir.OpAwait:
		return i == 0
	}
	return false
}

// ctxArgCount is how many trailing hidden provider arguments a call to info
// carries, the only ones with an actual parameter slot.
func ctxArgCount(info *sem.FnInfo) int {
	n := 0
	for _, k := range info.CtxRequires {
		if info.CtxReal[k] {
			n++
		}
	}
	return n
}

// IsOwnDrop reports whether fn is the registered `drop` of its own owner
// type, the one case where a bodiless std primitive still consumes `self`.
func IsOwnDrop(r *sem.Result, fn sem.EntityID, info *sem.FnInfo) bool {
	return info.Owner != 0 && r.TypeDecl(info.Owner).Drop == fn
}

// IsStdPrimitive reports whether an entity is a bodiless std function, which
// only the runtime implements.
func IsStdPrimitive(r *sem.Result, fn sem.EntityID) bool {
	if r.Entity(fn).Kind != sem.EntFn {
		return false
	}
	info := r.Fn(fn)
	return info.Body == 0 && info.Abi == "" && r.Entity(fn).Flags&sem.EfStd != 0
}

// Constructs is an aggregate literal: a field place among its elements moves into it.
func Constructs(op midir.Opcode) bool {
	return op == midir.OpRecord || op == midir.OpVariant || op == midir.OpArray
}
