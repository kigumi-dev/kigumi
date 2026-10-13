package mir

import "kigumi/internal/sem"

// verifyTypes checks the operand and result types the contract fixes:
// copies keep their type, conditions are Bool, borrows are references to
// what they borrow, constants and interpolations carry their literal type,
// and a return yields the function's type.
func (p *Program) verifyTypes(f *Func) *Error {
	tt := p.R.Types
	ty := func(l LocalID) sem.TypeID { return f.Locals[l].Type }
	for bid, blk := range f.Blocks {
		bid := BlockID(bid)
		for i, in := range blk.Insts {
			if in.Op.Yields() && yieldsData(in.Op) && ty(in.Dst) == sem.TyNever {
				return fail("types", bid, i, "%s yields a value into a local of type Never", in.Op)
			}
			switch in.Op {
			case OpCopy, OpMove, OpShare:
				if !p.sameType(ty(in.Dst), ty(in.Args[0])) && ty(in.Args[0]) != sem.TyNever {
					return fail("types", bid, i, "%s from %s into %s", in.Op, p.R.TypeString(ty(in.Args[0])), p.R.TypeString(ty(in.Dst)))
				}
			case OpAlias:
				if !p.sameTypeAcrossRef(ty(in.Dst), ty(in.Args[0])) && ty(in.Args[0]) != sem.TyNever {
					return fail("types", bid, i, "%s from %s into %s", in.Op, p.R.TypeString(ty(in.Args[0])), p.R.TypeString(ty(in.Dst)))
				}
			case OpConst:
				if ty(in.Dst) != in.Type {
					return fail("types", bid, i, "constant of %s into %s", p.R.TypeString(in.Type), p.R.TypeString(ty(in.Dst)))
				}
			case OpUnit:
				if t := ty(in.Dst); t != sem.TyUnit && t != sem.TyNever {
					return fail("types", bid, i, "unit into %s", p.R.TypeString(t))
				}
			case OpBinary:
				if msg := p.binaryMismatch(in, ty(in.Args[0]), ty(in.Args[1]), ty(in.Dst)); msg != "" {
					return fail("types", bid, i, "%s", msg)
				}
			case OpUnary:
				if in.Str == "cast" && !p.sameType(in.Type, ty(in.Dst)) {
					return fail("types", bid, i, "cast to %s into %s", p.R.TypeString(in.Type), p.R.TypeString(ty(in.Dst)))
				}
			case OpIsVariant, OpIsType:
				if ty(in.Dst) != sem.TyBool {
					return fail("types", bid, i, "%s yields Bool, not %s", in.Op, p.R.TypeString(ty(in.Dst)))
				}
			case OpInterp:
				if ty(in.Dst) != sem.TyString {
					return fail("types", bid, i, "interpolation yields String, not %s", p.R.TypeString(ty(in.Dst)))
				}
			case OpShell:
				if n := tt.Node(ty(in.Dst)); !p.looseType(ty(in.Dst)) && (n.Kind != sem.KNamed || n.Ent != p.R.LangItem("Plan")) {
					return fail("types", bid, i, "shell plan yields %s, not a Plan", p.R.TypeString(ty(in.Dst)))
				}
			case OpBorrow:
				if ty(in.Dst) != tt.Ref(ty(in.Args[0]), in.Index == 1) {
					return fail("types", bid, i, "borrow of %s yields %s", p.R.TypeString(ty(in.Args[0])), p.R.TypeString(ty(in.Dst)))
				}
			case OpVariant, OpCFnPtr:
				if ty(in.Dst) != in.Type {
					return fail("types", bid, i, "%s of %s into %s", in.Op, p.R.TypeString(in.Type), p.R.TypeString(ty(in.Dst)))
				}
			case OpBox:
				// Type is the dynamic type the box records, the value's own.
				if tt.Kind(ty(in.Dst)) != sem.KIface || in.Type != ty(in.Args[0]) {
					return fail("types", bid, i, "box of %s as %s into %s", p.R.TypeString(ty(in.Args[0])), p.R.TypeString(in.Type), p.R.TypeString(ty(in.Dst)))
				}
			case OpBoxDirect, OpUnboxDirect:
				// Only the unboxed side is named: the carrier may be a
				// reference or a defined type handing the same object on.
				side, carrier := ty(in.Dst), ty(in.Args[0])
				if in.Op == OpBoxDirect {
					side, carrier = carrier, side
				}
				if !p.sameType(in.Type, side) {
					return fail("types", bid, i, "%s of %s holds %s", in.Op, p.R.TypeString(in.Type), p.R.TypeString(side))
				}
				if !p.compatible(in.Type, carrier) {
					return fail("types", bid, i, "%s of %s carried as %s", in.Op, p.R.TypeString(in.Type), p.R.TypeString(carrier))
				}
			case OpFnItem:
				if sig := p.R.Fn(in.Ent).Sig; !p.fnItemFits(sig, ty(in.Dst), in.Async) {
					return fail("types", bid, i, "fn item %s of %s into %s", p.R.Entity(in.Ent).Name, p.R.TypeString(sig), p.R.TypeString(ty(in.Dst)))
				}
			case OpNewCell:
				if ty(in.Dst) != ty(in.Args[0]) {
					return fail("types", bid, i, "cell of %s into %s", p.R.TypeString(ty(in.Args[0])), p.R.TypeString(ty(in.Dst)))
				}
			}
		}
		term := len(blk.Insts)
		switch blk.Term.Op {
		case TermBranch:
			if t := ty(blk.Term.Args[0]); t != sem.TyBool {
				return fail("types", bid, term, "branch on %s", p.R.TypeString(t))
			}
		case TermReturn:
			if t := ty(blk.Term.Args[0]); !p.returnable(f, t) {
				return fail("types", bid, term, "return of %s (kind %d) from a function of %s (kind %d)", p.R.TypeString(t), tt.Kind(t), p.R.TypeString(f.Ret), tt.Kind(f.Ret))
			}
		}
	}
	return nil
}

// returnable accepts the function's type, a diverging value, and, for a
// script's implicit main, Unit standing for the `Ok(())` of falling off
// its statement list. Every other return of a fallible-Unit function must
// carry an actual Ok, not a bare Unit (see ImplicitOk).
func (p *Program) returnable(f *Func, t sem.TypeID) bool {
	if p.sameType(t, f.Ret) || t == sem.TyNever || f.Ret == 0 {
		return true
	}
	if f.ImplicitOk && t == sem.TyUnit {
		if ok, _, isResult := p.R.Types.IsResult(f.Ret); isResult && ok == sem.TyUnit {
			return true
		}
	}
	return false
}

// sameType is type identity, with a closure counted as its signature (every
// back end treats a closure as a function value of that type) and, failing
// that, two fn types compared ignoring `pure`/`noalloc`: closure erasure and a
// plain fn item both carry those qualifiers only for the checker's effects
// pass (an EdgeContract), never in the value's runtime representation.
func (p *Program) sameType(a, b sem.TypeID) bool {
	a, b = p.fnType(a), p.fnType(b)
	if a == b || p.sameFnRepr(a, b) {
		return true
	}
	// A defined type and its underlying chain share one representation, so
	// OpCopy/OpMove/OpShare cross that boundary at zero cost.
	return p.R.UnderlyingRepr(a) == p.R.UnderlyingRepr(b)
}

// sameTypeAcrossRef is sameType after peeling one reference layer off each
// side: a wrap/unwrap aliases its receiver's own bits even through a borrow.
func (p *Program) sameTypeAcrossRef(a, b sem.TypeID) bool {
	return p.sameType(p.derefOnce(a), p.derefOnce(b))
}

func (p *Program) derefOnce(t sem.TypeID) sem.TypeID {
	if p.R.Types.Kind(t) == sem.KRef {
		return p.R.Types.Node(t).Elem
	}
	return t
}

func (p *Program) fnType(t sem.TypeID) sem.TypeID {
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
	if na.Flags&structural != nb.Flags&structural || na.Elem != nb.Elem || len(na.Args) != len(nb.Args) {
		return false
	}
	for i := range na.Args {
		if na.Args[i] != nb.Args[i] {
			return false
		}
	}
	return true
}
