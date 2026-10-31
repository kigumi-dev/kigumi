package mir

import (
	midir "kigumi/internal/compiler/mir/ir"
	"kigumi/internal/compiler/sem"
)

// inPlace reports whether argument i is lent rather than handed over: the
// `self`/`mut self` receiver of a method, the target of a field, index or
// cell store, the function value of an indirect call, and every argument
// of a runtime primitive (bodiless std function), which borrows what it
// gets and retains what it keeps. A resource's own registered `drop`
// consumes `self` even when it is a bodiless std primitive: the runtime
// rule that std primitives never release is written for `self` a callee
// merely uses, not for `self` the callee IS the destructor of.
func (p *Program) inPlace(in midir.Inst, i int) bool {
	switch in.Op {
	case midir.OpCallValue:
		// A witness call lends its `self` / `mut self` receiver (mir
		// consumesArg agrees), like a direct method call.
		if in.Ent != 0 && i == 1 {
			recv := p.R.Fn(in.Ent).Recv
			return recv != sem.RecvNone && recv != sem.RecvMove
		}
		return i == 0
	case midir.OpSetField, midir.OpCellSet:
		return i == 0
	case midir.OpSetIndex:
		return i <= 1
	case midir.OpCallC, midir.OpAsm, midir.OpShell:
		return true
	case midir.OpCall:
		if p.R.Entity(in.Ent).Kind != sem.EntFn {
			return false
		}
		info := p.R.Fn(in.Ent)
		if i == 0 && p.isOwnDrop(in.Ent, info) {
			return false
		}
		if info.Body == 0 && info.Abi == "" && p.R.Entity(in.Ent).Flags&sem.EfStd != 0 {
			return true
		}
		if info.Abi != "" || info.Naked {
			return true
		}
		recv := info.Recv
		return i == 0 && recv != sem.RecvNone && recv != sem.RecvMove
	}
	return false
}

// true means argument i is lent, not handed over.
func (p *Program) ArgLent(in midir.Inst, i int) bool { return p.inPlace(in, i) }

// ParamLent is ArgLent for a caller that has no instruction of its own, an adapter say.
func (p *Program) ParamLent(fn sem.EntityID, i int) bool {
	in := midir.Inst{Op: midir.OpCall, Ent: fn}
	return p.inPlace(in, i) || p.lentParam(in, i)
}

// ParamBorrowed is ParamLent without the rule that a primitive lends every argument.
func (p *Program) ParamBorrowed(fn sem.EntityID, i int) bool {
	return p.lentParam(midir.Inst{Op: midir.OpCall, Ent: fn}, i)
}

// isOwnDrop reports whether fn is the registered `drop` of its own owner
// type, the one case where a bodiless std primitive still consumes `self`.
func (p *Program) isOwnDrop(fn sem.EntityID, info *sem.FnInfo) bool {
	return info.Owner != 0 && p.R.TypeDecl(info.Owner).Drop == fn
}

// isStdPrimitive reports whether an entity is a bodiless std function, which
// only the runtime implements.
func (p *Program) IsStdPrimitive(fn sem.EntityID) bool {
	if p.R.Entity(fn).Kind != sem.EntFn {
		return false
	}
	info := p.R.Fn(fn)
	return info.Body == 0 && info.Abi == "" && p.R.Entity(fn).Flags&sem.EfStd != 0
}

// copyable is what a consuming use duplicates instead of moving: Copy
// types, a type parameter included only when a `Copy` constraint proves it.
func (p *Program) copyable(t sem.TypeID) bool {
	switch p.R.Types.Kind(t) {
	case sem.KUntyped, sem.KVar, sem.KPrim:
		return true
	case sem.KRef:
		// A borrow is an alias of the caller's object: never duplicated,
		// never released.
		return false
	}
	return p.R.IsCopy(t)
}

// mustCopy reports whether an instruction consumes a named local that the
// scope still owns: Copy values (a Copy-constrained type parameter
// included) are duplicated (or retained) so the local's drop at scope exit
// stays balanced. Move-only values, type parameters included, are
// transferred as they are.
func (p *Program) mustCopy(f *midir.Func, in midir.Inst, a midir.LocalID) bool {
	l := f.Locals[a]
	if l.Ent == 0 || !p.copyable(l.Type) {
		return false
	}
	switch in.Op {
	case midir.OpDrop, midir.OpCopy, midir.OpBorrow, midir.OpField, midir.OpFieldMove, midir.OpIndex, midir.OpIsVariant, midir.OpIsType, midir.OpPayload, midir.OpUnbox,
		midir.OpCellGet, midir.OpCellTake, midir.OpBinary, midir.OpUnary, midir.OpInterp, midir.OpBuiltin, midir.OpFutureNew, midir.OpFutureDone:
		return false
	}
	return true
}
