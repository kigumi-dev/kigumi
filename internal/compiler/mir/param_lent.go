package mir

import (
	midir "kigumi/internal/compiler/mir/ir"
	ownquery "kigumi/internal/compiler/mir/ownership"
	"kigumi/internal/compiler/sem"
)

// InPlace reports whether argument i is lent rather than handed over: the
// `self`/`mut self` receiver of a method, the target of a field, index or
// cell store, the function value of an indirect call, and every argument
// of a runtime primitive (bodiless std function), which borrows what it
// gets and retains what it keeps. A resource's own registered `drop`
// consumes `self` even when it is a bodiless std primitive: the runtime
// rule that std primitives never release is written for `self` a callee
// merely uses, not for `self` the callee IS the destructor of.
func (p *Program) InPlace(in midir.Inst, i int) bool {
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
		if i == 0 && ownquery.IsOwnDrop(p.R, in.Ent, info) {
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
func (p *Program) ArgLent(in midir.Inst, i int) bool { return p.InPlace(in, i) }

// ParamLent is ArgLent for a caller that has no instruction of its own, an adapter say.
func (p *Program) ParamLent(fn sem.EntityID, i int) bool {
	in := midir.Inst{Op: midir.OpCall, Ent: fn}
	return p.InPlace(in, i) || p.LentParam(in, i)
}

// ParamBorrowed is ParamLent without the rule that a primitive lends every argument.
func (p *Program) ParamBorrowed(fn sem.EntityID, i int) bool {
	return p.LentParam(midir.Inst{Op: midir.OpCall, Ent: fn}, i)
}
