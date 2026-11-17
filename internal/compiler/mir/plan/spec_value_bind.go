package plan

import (
	midir "kigumi/internal/compiler/mir/ir"
	"kigumi/internal/compiler/sem"
)

// bindValueArgs binds the type arguments of a function value from its own type; a method value's receiver binds
// the owner's parameters.
func (p *planner) bindValueArgs(f *midir.Func, in midir.Inst, want map[sem.EntityID]bool, bound map[sem.EntityID]sem.TypeID) bool {
	r := p.R
	info := r.Fn(in.Ent)
	if len(info.Witnesses) > 0 {
		return false
	}
	switch {
	case in.Op == midir.OpFnItem && info.Recv == sem.RecvNone:
	case in.Form() == midir.FormMethod && info.Recv != sem.RecvNone && len(in.Args) == 1:
		recv := f.Locals[in.Args[0]].Type
		if r.Types.Kind(recv) == sem.KRef {
			recv = r.Types.Node(recv).Elem
		}
		if !r.Types.BindParams(r.Entity(info.SelfParam).Type, recv, want, bound) {
			return false
		}
	default:
		return false
	}
	return p.bindValueSig(info.Sig, f.Locals[in.Dst].Type, in.Async, want, bound)
}

// bindValueSig matches a declared signature against a function value's type, whose result an async value wraps in
// a Future.
func (p *planner) bindValueSig(sig, t sem.TypeID, async bool, want map[sem.EntityID]bool, bound map[sem.EntityID]sem.TypeID) bool {
	tt := p.R.Types
	if !async {
		return tt.BindParams(sig, t, want, bound)
	}
	decl, val := tt.Node(sig), tt.Node(t)
	if val.Kind != sem.KFn || len(val.Args) != len(decl.Args) {
		return false
	}
	fut := tt.Node(val.Elem)
	if fut.Kind != sem.KNamed || fut.Ent != tt.FutureEnt() || len(fut.Args) != 1 {
		return false
	}
	for i, a := range decl.Args {
		if !tt.BindParams(a, val.Args[i], want, bound) {
			return false
		}
	}
	return tt.BindParams(decl.Elem, fut.Args[0], want, bound)
}
