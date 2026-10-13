package mir

import (
	"strings"

	"kigumi/internal/sem"
)

var builtinArgs = map[string][2]int{
	"print": {1, 1}, "eprint": {1, 1}, "panic": {0, 1}, "message": {1, 1},
	"host": {0, 0}, "comptime": {0, 0}, "alloc.push": {1, 1}, "fixed.fill": {2, 2},
	"iter.len": {1, 1}, "iter.at": {2, 2}, "iter.at_ref": {2, 2}, "iter.at_move": {2, 2},
	"direct.eq": {2, 2}, "direct.hash": {1, 1},
}

func (p *Program) verifyBuiltins(f *Func) *Error {
	for bid, blk := range f.Blocks {
		bid := BlockID(bid)
		for i, in := range blk.Insts {
			if in.Op != OpBuiltin {
				continue
			}
			n, ok := builtinArgs[in.Str]
			switch {
			case !ok:
				return fail("builtins", bid, i, "no runtime entry %q", in.Str)
			case len(in.Args) < n[0] || len(in.Args) > n[1]:
				return fail("builtins", bid, i, "%s takes %d to %d arguments, got %d", in.Str, n[0], n[1], len(in.Args))
			}
			if msg := p.iterMismatch(f, in); msg != "" {
				return fail("builtins", bid, i, "%s", msg)
			}
			if idx := builtinIndex(in); idx >= 0 {
				if t := f.Locals[in.Args[idx]].Type; !p.R.Types.IsInteger(t) && !p.looseType(t) && t != sem.TyUntypedInt {
					return fail("builtins", bid, i, "%s indexes with %s", in.Str, p.R.TypeString(t))
				}
			}
		}
	}
	return nil
}

func builtinIndex(in Inst) int {
	switch in.Str {
	case "fixed.fill", "iter.at", "iter.at_ref", "iter.at_move":
		return 1
	}
	return -1
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
