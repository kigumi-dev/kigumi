package mir

import (
	midir "kigumi/internal/compiler/mir/ir"
	"kigumi/internal/compiler/sem"
)

func (p *verifier) verifyDirectSig(f *midir.Func) *Error {
	if f.Direct == nil {
		return nil
	}
	s := f.DSig
	if s == nil {
		if p.EnvABI(f) {
			return &Error{Rule: "direct", Msg: "a closure body with an unboxed environment has no direct signature"}
		}
		return nil
	}
	if len(s.Params) != len(f.Params) {
		return &Error{Rule: "direct", Msg: "direct signature of " + itoa(len(s.Params)) + " parameters for a function of " + itoa(len(f.Params))}
	}
	env := p.EnvABI(f)
	if f.Async || (!env && (f.Ent == 0 || !p.R.DirectCarried(f.Ent))) {
		return &Error{Rule: "direct", Msg: "direct signature for a function that passes everything boxed"}
	}
	caps := 0
	if env {
		caps = len(p.envTypes(f))
	}
	for i, t := range s.Params {
		prm := f.Params[i]
		have := f.Locals[prm].Type
		unboxable := p.R.DirectAgg(have) && (env || p.R.DirectParamSlot(f.Ent, f.Locals[prm].Ent))
		switch {
		case t != 0 && (!unboxable || !p.sameType(t, have)):
			return &Error{Rule: "direct", Msg: "direct signature takes " + p.R.TypeString(t) + " as parameter " + itoa(i) + " of another type, " + p.R.TypeString(have)}
		case t == 0 && i < caps && unboxable:
			return &Error{Rule: "direct", Msg: "direct signature leaves capture " + itoa(i) + " boxed"}
		}
	}
	if want := p.expectedResult(f); s.Ret != want && (want == 0 || !p.sameType(s.Ret, want)) {
		return &Error{Rule: "direct", Msg: "direct signature returns " + p.R.TypeString(s.Ret) + " for a function of " + p.R.TypeString(f.Ret)}
	}
	return nil
}

func (p *verifier) expectedResult(f *midir.Func) sem.TypeID {
	switch {
	case f.Async:
		return 0
	case p.EnvABI(f):
		if p.R.DirectAgg(f.Ret) {
			return f.Ret
		}
		return 0
	case f.Ent == 0:
		return 0
	}
	return p.R.DirectResultType(f.Ent, f.Ret)
}

// verifyCrossLocals checks that a crossing's own local is unnamed, set once and
// read right after, as plan-direct makes it; the flag decides who releases it.
func (p *verifier) verifyCrossLocals(f *midir.Func) *Error {
	any := false
	for _, l := range f.Locals {
		any = any || l.Cross
	}
	if !any {
		return nil
	}
	defs := make([]int, len(f.Locals))
	for bid, blk := range f.Blocks {
		for i, in := range blk.Insts {
			if in.Op.Yields() && f.Locals[in.Dst].Cross {
				defs[in.Dst]++
			}
			for _, a := range in.Args {
				if !f.Locals[a].Cross {
					continue
				}
				if adjacentDef(f, blk, i, a) < 0 {
					return fail("direct", midir.BlockID(bid), i, "%s reads the crossing local %%%s other than just after it is set", in.Op, f.Locals[a].Name)
				}
			}
		}
		for _, a := range blk.Term.Args {
			if f.Locals[a].Cross && adjacentDef(f, blk, len(blk.Insts), a) < 0 {
				return fail("direct", midir.BlockID(bid), len(blk.Insts), "terminator reads the crossing local %%%s other than just after it is set", f.Locals[a].Name)
			}
		}
	}
	for l, n := range defs {
		if n > 1 || (f.Locals[l].Cross && f.Locals[l].Ent != 0) {
			return &Error{Rule: "direct", Msg: "the crossing local %" + f.Locals[l].Name + " is named or set more than once"}
		}
	}
	return nil
}

func adjacentDef(f *midir.Func, blk midir.Block, use int, l midir.LocalID) int {
	for j := use - 1; j >= 0 && blk.Insts[j].Op.Yields() && f.Locals[blk.Insts[j].Dst].Cross; j-- {
		if blk.Insts[j].Dst == l {
			return j
		}
	}
	return -1
}
