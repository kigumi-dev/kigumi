package mir

import (
	"strings"

	"kigumi/internal/sem"
)

func (p *Program) verifyBuiltins(f *Func) *Error {
	for bid, blk := range f.Blocks {
		bid := BlockID(bid)
		for i, in := range blk.Insts {
			if in.Op != OpBuiltin {
				continue
			}
			if msg := p.builtinMismatch(f, in); msg != "" {
				return fail("builtins", bid, i, "%s", msg)
			}
		}
	}
	return nil
}

func (p *Program) builtinMismatch(f *Func, in Inst) string {
	spec, ok := builtinByName[in.Str]
	if !ok {
		return "no runtime entry \"" + in.Str + "\""
	}
	if len(in.Args) < spec.Min || len(in.Args) > len(spec.Args) {
		return in.Str + " takes " + itoa(spec.Min) + " to " + itoa(len(spec.Args)) + " arguments, got " + itoa(len(in.Args))
	}
	if msg := p.iterMismatch(f, in); msg != "" {
		return msg
	}
	for k, a := range in.Args {
		if msg := p.builtinArgMismatch(in.Str, k, spec.Args[k], f.Locals[a].Type); msg != "" {
			return msg
		}
	}
	if msg := p.builtinResultMismatch(spec, f.Locals[in.Dst].Type); msg != "" && in.Op.Yields() {
		return in.Str + " " + msg
	}
	return ""
}

func (p *Program) builtinArgMismatch(name string, k int, want ClassSet, got sem.TypeID) string {
	got = p.R.UnderlyingRepr(p.derefAll(got))
	switch {
	case p.looseType(got):
	case want == classBit(ClassString) && !p.compatible(sem.TyString, got):
		return name + " takes " + p.R.TypeString(got) + " as argument " + itoa(k) + ", not a String"
	case want == classBit(ClassInt) && !p.R.Types.IsInteger(got) && got != sem.TyUntypedInt:
		return name + " indexes with " + p.R.TypeString(got)
	}
	return ""
}

func (p *Program) builtinResultMismatch(spec *BuiltinSpec, got sem.TypeID) string {
	var want sem.TypeID
	switch spec.Result {
	case classBit(ClassUnit):
		want = sem.TyUnit
	case classBit(ClassBool):
		want = sem.TyBool
	case classBit(ClassInt):
		if t := p.R.UnderlyingRepr(p.derefAll(got)); !p.looseType(t) && !p.R.Types.IsInteger(t) {
			return "yields " + p.R.TypeString(got) + ", not an integer"
		}
		return ""
	default:
		return ""
	}
	if !p.looseType(got) && !p.compatible(want, got) {
		return "yields " + p.R.TypeString(got) + ", not " + p.R.TypeString(want)
	}
	return ""
}

func (p *Program) iterMismatch(f *Func, in Inst) string {
	if !strings.HasPrefix(in.Str, "iter.") || len(in.Args) == 0 {
		return ""
	}
	t := p.R.UnderlyingRepr(p.derefAll(f.Locals[in.Args[0]].Type))
	switch p.R.Types.Kind(t) {
	case sem.KNamed, sem.KParam, sem.KVar, sem.KUntyped, sem.KPoison, sem.KIface:
		return ""
	}
	if t == sem.TyString || t == sem.TyBytes || p.looseType(t) {
		return ""
	}
	return in.Str + " walks " + p.R.TypeString(t)
}
