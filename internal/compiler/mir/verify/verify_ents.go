package verify

import (
	midir "kigumi/internal/compiler/mir/ir"
	"kigumi/internal/compiler/sem"
)

// verifyEntities checks that what an instruction names exists and is of
// the kind the opcode reads: a function to call, a variant to build or
// test, a record type to build, a closure to capture, asm info at the
// asm node. A back end indexes these without checking.
func (p *verifier) verifyEntities(f *midir.Func) *Error {
	kindOf := func(ent sem.EntityID) sem.EntityKind {
		if int(ent) >= len(p.R.Entities) {
			return sem.EntNone
		}
		return p.R.Entity(ent).Kind
	}
	for bid, blk := range f.Blocks {
		bid := midir.BlockID(bid)
		for i, in := range blk.Insts {
			want := ""
			switch in.Op {
			case midir.OpCall, midir.OpFnItem, midir.OpCFnPtr:
				if kindOf(in.Ent) != sem.EntFn {
					want = "a function"
				} else if _, minted := p.R.InstanceOf(in.Ent); minted && p.ByEnt[in.Ent] == nil {
					want = "an instantiation with a body"
				}
			case midir.OpBind:
				// The bind adapter wraps a lowered body; a std primitive
				// binds through its runtime key.
				if kindOf(in.Ent) != sem.EntFn || (p.ByEnt[in.Ent] == nil && !(p.R.Fn(in.Ent).Body == 0 && p.R.Entity(in.Ent).Flags&sem.EfStd != 0)) {
					want = "a function with a body or a std primitive"
				}
			case midir.OpCallValue:
				if in.Ent != 0 && kindOf(in.Ent) != sem.EntFn {
					want = "a function or nothing"
				}
			case midir.OpVariant, midir.OpIsVariant:
				if kindOf(in.Ent) != sem.EntVariant {
					want = "a variant"
				}
			case midir.OpRecord:
				if kindOf(in.Ent) != sem.EntType {
					want = "a record type"
				} else if n := len(p.R.TypeDecl(in.Ent).Fields); n != len(in.Args) {
					return fail("entities", bid, i, "record of %d fields built from %d values", n, len(in.Args))
				}
			case midir.OpIsType:
				if k := kindOf(in.Ent); k != sem.EntType && k != sem.EntAlias {
					want = "a type"
				}
			case midir.OpClosure:
				form, k := in.Form(), kindOf(in.Ent)
				if !(form == midir.FormClosure && k == sem.EntClosure && p.ByEnt[in.Ent] != nil || (form == midir.FormMethod || form == midir.FormWitness || form == midir.FormApplied) && k == sem.EntFn) {
					want = "a lowered closure, or a method for a method value"
				}
			case midir.OpAsm:
				if p.R.AsmOf(f.File, in.Node) == nil {
					return fail("entities", bid, i, "asm without its operand table")
				}
			case midir.OpGlobalGet, midir.OpGlobalSet:
				if kindOf(in.Ent) != sem.EntStatic {
					want = "a static"
				}
			case midir.OpField, midir.OpFieldMove, midir.OpPayload:
				if in.Index < 0 {
					return fail("entities", bid, i, "%s index %d", in.Op, in.Index)
				}
			}
			if want == "" && (in.Op == midir.OpFnItem || in.Op == midir.OpClosure || in.Op == midir.OpBind && in.Async) && in.Async != p.asyncValue(in.Ent) {
				return fail("entities", bid, i, "%s of %s is marked async %v, which its declaration says %v", in.Op, p.R.Entity(in.Ent).Name, in.Async, !in.Async)
			}
			if want != "" {
				name := "entity " + itoa(int(in.Ent)) + " (out of range)"
				if int(in.Ent) < len(p.R.Entities) {
					name = p.DeclName(in.Ent)
				}
				return fail("entities", bid, i, "%s names %s, which is not %s", in.Op, name, want)
			}
		}
	}
	return nil
}

func (p *verifier) asyncValue(ent sem.EntityID) bool {
	return p.R.Entity(ent).Kind == sem.EntFn && p.R.Fn(ent).Declared&sem.EffAsync != 0
}
