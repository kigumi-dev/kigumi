package borrow

import (
	"slices"

	midir "kigumi/internal/compiler/mir/ir"
	"kigumi/internal/compiler/sem"
)

// holdsCallable: a value of t may keep a callable, counting a type parameter as anything.
func holdsCallable(bc *borrowCheck, t sem.TypeID) bool {
	return bc.typeRuns(t, runsCall) || bc.facts().NamesTypeParam(t)
}

// callee is the lowered body a direct call runs, when its parameters line up with the arguments.
func (e *erasedReaders) callee(in midir.Inst) *midir.Func {
	if in.Op != midir.OpCall || in.Ent == 0 || e.p.R.Entity(in.Ent).Kind != sem.EntFn {
		return nil
	}
	if g := e.p.ByEnt[in.Ent]; g != nil && len(g.Params) == len(in.Args) {
		return g
	}
	return nil
}

// resultMayHold: in's result may keep what src hands it.
func (e *erasedReaders) resultMayHold(in midir.Inst, src midir.LocalID) bool {
	g := e.callee(in)
	if g == nil {
		return true
	}
	for i, a := range in.Args {
		if a == src && e.fate(g, i).result {
			return true
		}
	}
	return false
}

// escapesAt: in may keep argument i where no flow of this body follows it (a global, a task, an unknown callee).
func (e *erasedReaders) escapesAt(f *midir.Func, in midir.Inst, i int) bool {
	switch in.Op {
	case midir.OpCall:
		if e.p.R.Entity(in.Ent).Kind != sem.EntFn {
			return true
		}
		info := e.p.R.Fn(in.Ent)
		if p := paramAt(info, i); p != 0 && e.p.R.NonEscapingParam(p) || info.Declared&sem.EffPure != 0 {
			return false
		}
		if g := e.callee(in); g != nil {
			return e.fate(g, i).escapes
		}
		return true
	case midir.OpCallValue:
		t := f.Locals[in.Args[0]].Type
		return i != 0 && sem.Effects(e.p.R.Types.Node(t).Flags)&sem.EffPure == 0
	case midir.OpGlobalSet, midir.OpCallC, midir.OpBuiltin, midir.OpAsm, midir.OpShell:
		return true
	}
	return false
}

func paramAt(info *sem.FnInfo, i int) sem.EntityID {
	if info.Recv != sem.RecvNone {
		if i == 0 {
			return 0
		}
		i--
	}
	if i < len(info.Params) {
		return info.Params[i]
	}
	return 0
}

// escapes lists the holders in hands to a place no flow of this body follows, or to its caller.
func (e *erasedReaders) escapes(in midir.Inst, caps map[midir.LocalID][]capturedPlace) []midir.LocalID {
	out := e.intoCaller(in, caps)
	for i, a := range in.Args {
		if _, ok := caps[a]; ok && e.escapesAt(e.f, in, i) && !slices.Contains(out, a) {
			out = append(out, a)
		}
	}
	return out
}

// dropRuns: dropping l may run code that calls what it holds, a user `drop` that may run a value or an interface's.
func (e *erasedReaders) dropRuns(l midir.LocalID) bool {
	return e.bc.typeHolds(e.f.Locals[l].Type, func(t sem.TypeID) bool {
		n := e.p.R.Types.Node(t)
		if n.Kind == sem.KIface {
			return true
		}
		if n.Kind != sem.KNamed || e.p.R.Entity(n.Ent).Kind != sem.EntType {
			return false
		}
		d := e.p.R.TypeDecl(n.Ent)
		return d != nil && d.Form == sem.FormResource && d.Drop != 0 && e.bc.mayRun(d.Drop)
	})
}
