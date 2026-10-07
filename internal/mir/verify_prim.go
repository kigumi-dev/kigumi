package mir

import "kigumi/internal/sem"

// primKind is how a runtime primitive sees a value: raw leaf bits of one
// scalar type, a reference, or a boxed object.
type primKind uint8

const (
	kindUnknown primKind = iota
	kindScalar
	kindRef
	kindObject
)

func (p *Program) primKindOf(t sem.TypeID) primKind {
	tt := p.R.Types
	if t == 0 || t == sem.TyNever {
		return kindUnknown
	}
	switch tt.Kind(t) {
	case sem.KParam, sem.KVar, sem.KUntyped:
		return kindUnknown
	case sem.KRef:
		return kindRef
	}
	if p.R.RegScalar(t) {
		return kindScalar
	}
	return kindObject
}

func (k primKind) String() string {
	return [...]string{"unknown", "scalar", "reference", "object"}[k]
}

// A mismatch is rejected for the boxed and the direct result form alike: the back end picks the runtime entry from the declared signature.
func (p *Program) verifyPrimCalls(f *Func) *Error {
	for bid, blk := range f.Blocks {
		for i, in := range blk.Insts {
			if in.Op != OpCall || !p.IsStdPrimitive(in.Ent) {
				continue
			}
			if msg := p.primMismatch(f, in); msg != "" {
				return fail("types", BlockID(bid), i, "call of %s: %s", p.R.Entity(in.Ent).Name, msg)
			}
		}
	}
	return nil
}

func (p *Program) primMismatch(f *Func, in Inst) string {
	info := p.R.Fn(in.Ent)
	sig := p.R.Types.Node(info.Sig)
	skip := len(in.Args) - len(sig.Args)
	if skip < 0 || skip > 1 || skip == 1 && info.Recv == sem.RecvNone {
		return ""
	}
	if skip == 1 {
		if msg := p.primSlot(p.R.Entity(info.Owner).Type, p.derefAll(f.Locals[in.Args[0]].Type)); msg != "" {
			return "receiver " + msg
		}
	}
	for j, want := range sig.Args {
		if msg := p.primSlot(want, f.Locals[in.Args[j+skip]].Type); msg != "" {
			return "argument " + itoa(j+skip) + " " + msg
		}
	}
	if in.Op.Yields() {
		if msg := p.primSlot(sig.Elem, f.Locals[in.Dst].Type); msg != "" {
			return "result " + msg
		}
	}
	return ""
}

func (p *Program) primSlot(want, got sem.TypeID) string {
	wk, gk := p.primKindOf(want), p.primKindOf(got)
	if wk == kindUnknown || gk == kindUnknown {
		return ""
	}
	if wk != gk || wk == kindScalar && !p.sameType(want, got) {
		return "wants " + wk.String() + " " + p.R.TypeString(want) + ", got " + gk.String() + " " + p.R.TypeString(got)
	}
	return ""
}

func (p *Program) derefAll(t sem.TypeID) sem.TypeID {
	for p.R.Types.Kind(t) == sem.KRef {
		t = p.R.Types.Node(t).Elem
	}
	return t
}
