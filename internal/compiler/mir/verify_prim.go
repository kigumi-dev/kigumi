package mir

import "kigumi/internal/compiler/sem"

// primKind is how a runtime primitive sees a value: raw leaf bits of one
// scalar type, a reference, or a boxed object.
type primKind uint8

const (
	kindUnknown primKind = iota
	kindScalar
	kindRef
	kindObject
)

func (p *verifier) primKindOf(t sem.TypeID) primKind {
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
func (p *verifier) verifyPrimCalls(f *Func) *Error {
	for bid, blk := range f.Blocks {
		for i, in := range blk.Insts {
			if in.Op != OpCall || !p.IsStdPrimitive(in.Ent) {
				continue
			}
			msg := p.primMismatch(f, in)
			if msg == "" {
				msg = p.opDescMismatch(f, in)
			}
			if msg != "" {
				return fail("types", BlockID(bid), i, "call of %s: %s", p.R.Entity(in.Ent).Name, msg)
			}
		}
	}
	return nil
}

func (p *verifier) primMismatch(f *Func, in Inst) string {
	info := p.R.Fn(in.Ent)
	sig := p.R.Types.Node(info.Sig)
	skip := len(in.Args) - len(sig.Args)
	if skip < 0 || skip > 1 || skip == 1 && info.Recv == sem.RecvNone {
		return ""
	}
	if skip == 1 {
		if msg := p.receiverMismatch(p.receiverOwner(info), f.Locals[in.Args[0]].Type); msg != "" {
			return "receiver " + msg
		}
	}
	for j, want := range sig.Args {
		if msg := p.primSlot(want, f.Locals[in.Args[j+skip]].Type); msg != "" {
			return "argument " + itoa(j+skip) + " " + msg
		}
	}
	// A declared Void result is a placeholder for a primitive whose result is typed from a type argument.
	if in.Op.Yields() && sig.Elem != sem.TyUnit {
		if msg := p.primSlot(sig.Elem, f.Locals[in.Dst].Type); msg != "" {
			return "result " + msg
		}
	}
	return ""
}

func (p *verifier) primSlot(want, got sem.TypeID) string {
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

// A generic type's entity has no type of its own, so its methods are held to their declared receiver.
func (p *verifier) receiverOwner(info *sem.FnInfo) sem.TypeID {
	if info.SelfParam != 0 {
		return p.R.Entity(info.SelfParam).Type
	}
	return p.R.Entity(info.Owner).Type
}

func (p *verifier) receiverMismatch(owner, got sem.TypeID) string {
	owner, got = p.derefAll(owner), p.derefAll(got)
	if msg := p.primSlot(owner, got); msg != "" {
		return msg
	}
	if !p.looseType(owner) && !p.compatibleGeneric(owner, got) {
		return "wants " + p.R.TypeString(owner) + ", got " + p.R.TypeString(got)
	}
	return ""
}
