package mir

import "kigumi/internal/compiler/sem"

func (p *verifier) indexMismatch(f *Func, in Inst) string {
	tt := p.R.Types
	base := p.R.UnderlyingRepr(p.derefAll(f.Locals[in.Args[0]].Type))
	if p.looseType(base) {
		return ""
	}
	var elem sem.TypeID
	textual := base == sem.TyString || base == sem.TyBytes
	switch n := tt.Node(base); {
	case textual:
	case n.Kind == sem.KNamed && n.Ent == tt.ArrayEnt() && len(n.Args) == 1:
		elem = n.Args[0]
	case p.R.IsFixedArray(base):
		elem, _, _ = p.R.FixedElem(base)
	default:
		return "indexes " + p.R.TypeString(f.Locals[in.Args[0]].Type) + ", which is no array, string or bytes"
	}
	if textual && in.Op == OpSetIndex {
		return "writes into " + p.R.TypeString(base) + ", which has no slots to set"
	}
	idx := p.derefAll(f.Locals[in.Args[1]].Type)
	if !p.looseType(idx) && idx != sem.TyUsize && !(in.Op == OpIndex && p.rangeOfUsize(idx)) {
		return "indexes by " + p.R.TypeString(idx) + ", not a usize"
	}
	if in.Op == OpSetIndex && elem != 0 && !p.compatible(elem, f.Locals[in.Args[2]].Type) {
		return "stores " + p.R.TypeString(f.Locals[in.Args[2]].Type) + " in an array of " + p.R.TypeString(elem)
	}
	return ""
}

func (p *verifier) rangeOfUsize(t sem.TypeID) bool {
	n := p.R.Types.Node(t)
	rng := p.R.LangItem("Range")
	return n.Kind == sem.KNamed && rng != 0 && n.Ent == rng && len(n.Args) == 1 && n.Args[0] == sem.TyUsize
}
