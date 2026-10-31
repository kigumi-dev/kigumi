package mir

import (
	midir "kigumi/internal/compiler/mir/ir"
)

// verifyBoxedEntry checks that a boxed caller of a direct signature has an
// entry to call, and that the entry takes and returns everything boxed.
func (p *verifier) verifyBoxedEntry(f *midir.Func) *Error {
	if f.Wraps == nil {
		if f.DSig != nil && f.Ent != 0 && f.Entry == nil {
			return &Error{Rule: "direct", Msg: "a direct signature with no boxed entry"}
		}
		return nil
	}
	w := f.Wraps
	switch {
	case f.DSig != nil:
		return &Error{Rule: "direct", Msg: "a boxed entry with a direct signature"}
	case w.Entry != f:
		return &Error{Rule: "direct", Msg: "a boxed entry its function does not name"}
	case len(f.Params) != len(w.Params) || !p.sameType(f.Ret, w.Ret):
		return &Error{Rule: "direct", Msg: "a boxed entry whose shape differs from " + w.Name}
	}
	for i, prm := range f.Params {
		if f.IsDirect(prm) || !p.sameType(f.Locals[prm].Type, w.Locals[w.Params[i]].Type) {
			return &Error{Rule: "direct", Msg: "boxed entry parameter " + itoa(i) + " is not the boxed " + p.R.TypeString(w.Locals[w.Params[i]].Type)}
		}
	}
	for bid, blk := range f.Blocks {
		if blk.Term.Op == midir.TermReturn && f.IsDirect(blk.Term.Args[0]) {
			return fail("direct", midir.BlockID(bid), len(blk.Insts), "a boxed entry returns its result unboxed")
		}
	}
	return nil
}
