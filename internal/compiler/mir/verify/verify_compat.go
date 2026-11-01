package verify

import "kigumi/internal/compiler/sem"

func (p *verifier) looseType(t sem.TypeID) bool {
	if t == 0 || t == sem.TyNever {
		return true
	}
	switch p.R.Types.Kind(t) {
	case sem.KVar, sem.KUntyped, sem.KPoison, sem.KConst:
		return true
	}
	return false
}

func (p *verifier) compatible(want, got sem.TypeID) bool { return p.compat(want, got, false) }

func (p *verifier) compatibleGeneric(want, got sem.TypeID) bool { return p.compat(want, got, true) }

func (p *verifier) compat(want, got sem.TypeID, open bool) bool {
	return p.compatAt(want, got, open, false)
}

func (p *verifier) compatAt(want, got sem.TypeID, open, nested bool) bool {
	tt := p.R.Types
	want, got = p.FnType(p.DerefAll(want)), p.FnType(p.DerefAll(got))
	if want == got || p.looseType(want) || p.looseType(got) || p.SameType(want, got) {
		return true
	}
	nw, ng := tt.Node(want), tt.Node(got)
	if open && (nw.Kind == sem.KParam || nested && ng.Kind == sem.KParam) {
		return true
	}
	if nw.Kind != ng.Kind {
		return false
	}
	switch nw.Kind {
	case sem.KParam:
		return nw.Ent == ng.Ent
	case sem.KNamed, sem.KIface:
		if nw.Ent != ng.Ent || len(nw.Args) != len(ng.Args) {
			return false
		}
		for i := range nw.Args {
			if !p.compatAt(nw.Args[i], ng.Args[i], open, true) {
				return false
			}
		}
		return true
	case sem.KFn:
		if len(nw.Args) != len(ng.Args) || !p.compatAt(nw.Elem, ng.Elem, open, true) {
			return false
		}
		for i := range nw.Args {
			if !p.compatAt(nw.Args[i], ng.Args[i], open, true) {
				return false
			}
		}
		return true
	case sem.KPtr:
		return p.compatAt(nw.Elem, ng.Elem, open, true)
	}
	return false
}

func (p *verifier) fnItemFits(sig, got sem.TypeID, async bool) bool {
	tt := p.R.Types
	if !async || p.looseType(got) {
		return p.compatibleGeneric(sig, got)
	}
	ns, ng := tt.Node(sig), tt.Node(p.FnType(p.DerefAll(got)))
	if ns.Kind != sem.KFn || ng.Kind != sem.KFn || len(ns.Args) != len(ng.Args) {
		return false
	}
	for i := range ns.Args {
		if !p.compatibleGeneric(ns.Args[i], ng.Args[i]) {
			return false
		}
	}
	return true
}

func (p *verifier) fits(want, got sem.TypeID) bool {
	if p.looseType(want) || p.looseType(got) {
		return true
	}
	tt := p.R.Types
	if (tt.Kind(want) == sem.KRef) != (tt.Kind(got) == sem.KRef) {
		return false
	}
	return p.compatible(want, got)
}
