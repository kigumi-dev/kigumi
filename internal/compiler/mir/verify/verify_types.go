package verify

import (
	midir "kigumi/internal/compiler/mir/ir"
	"kigumi/internal/compiler/sem"
)

// verifyTypes checks the operand and result types the contract fixes:
// copies keep their type, conditions are Bool, borrows are references to
// what they borrow, constants and interpolations carry their literal type,
// and a return yields the function's type.
func (p *verifier) verifyTypes(f *midir.Func) *Error {
	tt := p.R.Types
	ty := func(l midir.LocalID) sem.TypeID { return f.Locals[l].Type }
	for bid, blk := range f.Blocks {
		bid := midir.BlockID(bid)
		for i, in := range blk.Insts {
			if in.Op.Yields() && yieldsData(in.Op) && ty(in.Dst) == sem.TyNever {
				return fail("types", bid, i, "%s yields a value into a local of type Never", in.Op)
			}
			switch in.Op {
			case midir.OpCopy, midir.OpMove, midir.OpShare:
				if !p.SameType(ty(in.Dst), ty(in.Args[0])) && ty(in.Args[0]) != sem.TyNever {
					return fail("types", bid, i, "%s from %s into %s", in.Op, p.R.TypeString(ty(in.Args[0])), p.R.TypeString(ty(in.Dst)))
				}
			case midir.OpAlias:
				if !p.sameTypeAcrossRef(ty(in.Dst), ty(in.Args[0])) && ty(in.Args[0]) != sem.TyNever {
					return fail("types", bid, i, "%s from %s into %s", in.Op, p.R.TypeString(ty(in.Args[0])), p.R.TypeString(ty(in.Dst)))
				}
			case midir.OpConst:
				if ty(in.Dst) != in.Type {
					return fail("types", bid, i, "constant of %s into %s", p.R.TypeString(in.Type), p.R.TypeString(ty(in.Dst)))
				}
			case midir.OpUnit:
				if t := ty(in.Dst); t != sem.TyUnit && t != sem.TyNever {
					return fail("types", bid, i, "unit into %s", p.R.TypeString(t))
				}
			case midir.OpBinary:
				if msg := p.binaryMismatch(in, ty(in.Args[0]), ty(in.Args[1]), ty(in.Dst)); msg != "" {
					return fail("types", bid, i, "%s", msg)
				}
			case midir.OpUnary:
				if in.Str == "cast" && !p.SameType(in.Type, ty(in.Dst)) {
					return fail("types", bid, i, "cast to %s into %s", p.R.TypeString(in.Type), p.R.TypeString(ty(in.Dst)))
				}
			case midir.OpIsVariant, midir.OpIsType:
				if ty(in.Dst) != sem.TyBool {
					return fail("types", bid, i, "%s yields Bool, not %s", in.Op, p.R.TypeString(ty(in.Dst)))
				}
			case midir.OpInterp:
				if ty(in.Dst) != sem.TyString {
					return fail("types", bid, i, "interpolation yields String, not %s", p.R.TypeString(ty(in.Dst)))
				}
			case midir.OpShell:
				if n := tt.Node(ty(in.Dst)); !p.looseType(ty(in.Dst)) && (n.Kind != sem.KNamed || n.Ent != p.R.LangItem("Plan")) {
					return fail("types", bid, i, "shell plan yields %s, not a Plan", p.R.TypeString(ty(in.Dst)))
				}
			case midir.OpBorrow:
				if tt.WithRefLifetime(ty(in.Dst), 0) != tt.Ref(ty(in.Args[0]), in.Index == 1) {
					return fail("types", bid, i, "borrow of %s yields %s", p.R.TypeString(ty(in.Args[0])), p.R.TypeString(ty(in.Dst)))
				}
			case midir.OpVariant, midir.OpCFnPtr:
				if ty(in.Dst) != in.Type {
					return fail("types", bid, i, "%s of %s into %s", in.Op, p.R.TypeString(in.Type), p.R.TypeString(ty(in.Dst)))
				}
			case midir.OpBox:
				// Type is the dynamic type the box records, the value's own.
				if tt.Kind(ty(in.Dst)) != sem.KIface || in.Type != ty(in.Args[0]) {
					return fail("types", bid, i, "box of %s as %s into %s", p.R.TypeString(ty(in.Args[0])), p.R.TypeString(in.Type), p.R.TypeString(ty(in.Dst)))
				}
			case midir.OpLendBox:
				if r := tt.Node(ty(in.Dst)); r.Kind != sem.KRef || tt.Kind(r.Elem) != sem.KIface || in.Type != tt.Node(ty(in.Args[0])).Elem {
					return fail("types", bid, i, "lendbox of %s as %s into %s", p.R.TypeString(ty(in.Args[0])), p.R.TypeString(in.Type), p.R.TypeString(ty(in.Dst)))
				}
			case midir.OpBoxDirect, midir.OpUnboxDirect:
				// Only the unboxed side is named: the carrier may be a
				// reference or a defined type handing the same object on.
				side, carrier := ty(in.Dst), ty(in.Args[0])
				if in.Op == midir.OpBoxDirect {
					side, carrier = carrier, side
				}
				if !p.SameType(in.Type, side) {
					return fail("types", bid, i, "%s of %s holds %s", in.Op, p.R.TypeString(in.Type), p.R.TypeString(side))
				}
				if !p.compatible(in.Type, carrier) {
					return fail("types", bid, i, "%s of %s carried as %s", in.Op, p.R.TypeString(in.Type), p.R.TypeString(carrier))
				}
			case midir.OpFnItem:
				if sig := p.R.Fn(in.Ent).Sig; !p.fnItemFits(sig, ty(in.Dst), in.Async) {
					return fail("types", bid, i, "fn item %s of %s into %s", p.R.Entity(in.Ent).Name, p.R.TypeString(sig), p.R.TypeString(ty(in.Dst)))
				}
			case midir.OpNewCell:
				if ty(in.Dst) != ty(in.Args[0]) {
					return fail("types", bid, i, "cell of %s into %s", p.R.TypeString(ty(in.Args[0])), p.R.TypeString(ty(in.Dst)))
				}
			}
		}
		term := len(blk.Insts)
		switch blk.Term.Op {
		case midir.TermBranch:
			if t := ty(blk.Term.Args[0]); t != sem.TyBool {
				return fail("types", bid, term, "branch on %s", p.R.TypeString(t))
			}
		case midir.TermReturn:
			if t := ty(blk.Term.Args[0]); !p.returnable(f, t) {
				return fail("types", bid, term, "return of %s (kind %d) from a function of %s (kind %d)", p.R.TypeString(t), tt.Kind(t), p.R.TypeString(f.Ret), tt.Kind(f.Ret))
			}
		}
	}
	return nil
}

// returnable accepts the function's type, a diverging value, and, for a
// script's implicit main, Void standing for the `Ok(())` of falling off
// its statement list. Every other return of a fallible-Void function must
// carry an actual Ok, not a bare Void (see ImplicitOk).
func (p *verifier) returnable(f *midir.Func, t sem.TypeID) bool {
	if p.SameType(t, f.Ret) || t == sem.TyNever || f.Ret == 0 {
		return true
	}
	if f.ImplicitOk && t == sem.TyUnit {
		if ok, _, isResult := p.R.Types.IsResult(f.Ret); isResult && ok == sem.TyUnit {
			return true
		}
	}
	return false
}

// sameTypeAcrossRef is sameType after peeling one reference layer off each
// side: a wrap/unwrap aliases its receiver's own bits even through a borrow.
func (p *verifier) sameTypeAcrossRef(a, b sem.TypeID) bool {
	return p.SameType(p.derefOnce(a), p.derefOnce(b))
}

func (p *verifier) derefOnce(t sem.TypeID) sem.TypeID {
	if p.R.Types.Kind(t) == sem.KRef {
		return p.R.Types.Node(t).Elem
	}
	return t
}
