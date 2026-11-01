package mir

import (
	midir "kigumi/internal/compiler/mir/ir"
	"kigumi/internal/compiler/sem"
)

// SameType counts a closure as its signature and ignores `pure`/`noalloc`/escaping on fn types:
// every back end treats a closure as a function value, and those qualifiers never reach the runtime representation.
func (p *Program) SameType(a, b sem.TypeID) bool {
	a, b = p.FnType(a), p.FnType(b)
	a, b = p.R.Types.WithRefLifetime(a, 0), p.R.Types.WithRefLifetime(b, 0)
	if a == b || p.sameFnRepr(a, b) || p.R.Types.SameIgnoringEscaping(a, b) {
		return true
	}
	// A defined type and its underlying chain share one representation, so
	// OpCopy/OpMove/OpShare cross that boundary at zero cost.
	return p.R.UnderlyingRepr(a) == p.R.UnderlyingRepr(b)
}

func (p *Program) FnType(t sem.TypeID) sem.TypeID {
	if n := p.R.Types.Node(t); n.Kind == sem.KClosure {
		return p.R.Closure(n.Ent).Sig
	}
	return t
}

func (p *Program) sameFnRepr(a, b sem.TypeID) bool {
	tt := p.R.Types
	na, nb := tt.Node(a), tt.Node(b)
	const structural = ^uint16(sem.EffPure | sem.EffNoalloc)
	if na.Kind != sem.KFn || nb.Kind != sem.KFn {
		return false
	}
	if na.Flags&structural != nb.Flags&structural || !p.SameType(na.Elem, nb.Elem) || len(na.Args) != len(nb.Args) {
		return false
	}
	for i := range na.Args {
		if !p.SameType(na.Args[i], nb.Args[i]) {
			return false
		}
	}
	return true
}

func (p *Program) DerefAll(t sem.TypeID) sem.TypeID {
	for p.R.Types.Kind(t) == sem.KRef {
		t = p.R.Types.Node(t).Elem
	}
	return t
}

func (p *Program) LentByType(f *midir.Func, in *midir.Inst, k int) bool {
	if in.Op != midir.OpCallValue || in.Ent != 0 || k == 0 {
		return false
	}
	t := p.FnType(p.DerefAll(f.Locals[in.Args[0]].Type))
	if shape := p.R.FnShapeOf(t); shape != 0 {
		t = shape
	}
	node := p.R.Types.Node(t)
	return node.Kind == sem.KFn && k-1 < len(node.Args) && p.R.Types.Kind(node.Args[k-1]) == sem.KRef
}
