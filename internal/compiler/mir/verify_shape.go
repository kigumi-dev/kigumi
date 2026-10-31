package mir

import (
	"math/big"
	"strconv"

	midir "kigumi/internal/compiler/mir/ir"
)

// verifyShape checks each instruction against its opcode's contract.
func verifyShape(f *midir.Func) *Error {
	for bid, blk := range f.Blocks {
		bid := midir.BlockID(bid)
		for i, in := range blk.Insts {
			s, ok := in.Op.Shape()
			if !ok {
				return fail("shape", bid, i, "unknown opcode %d", in.Op)
			}
			if len(in.Args) < s.Min || (s.Max >= 0 && len(in.Args) > s.Max) {
				return fail("shape", bid, i, "%s takes %s, got %d", in.Op, arity(s), len(in.Args))
			}
			if s.Entity && in.Ent == 0 {
				return fail("shape", bid, i, "%s needs an entity", in.Op)
			}
			if s.Type && in.Type == 0 {
				return fail("shape", bid, i, "%s needs a type", in.Op)
			}
			if s.Text && in.Str == "" {
				return fail("shape", bid, i, "%s needs its text", in.Op)
			}
			if in.Op == midir.OpAwait && in.Str != "" && in.Str != "settle" {
				return fail("shape", bid, i, "await carries the text %q, which is not a mode", in.Str)
			}
			if !s.Text && in.Str != "" && in.Op != midir.OpClosure && in.Op != midir.OpNop && in.Op != midir.OpConst && in.Op != midir.OpAwait {
				return fail("shape", bid, i, "%s carries the text %q it never reads", in.Op, in.Str)
			}
			if in.Op == midir.OpConst && in.Str != "" && !numberText(in) {
				return fail("shape", bid, i, "const carries the text %q, which is not its number", in.Str)
			}
			if msg := flagMisplaced(in); msg != "" {
				return fail("shape", bid, i, "%s %s", in.Op, msg)
			}
			switch in.Op {
			case midir.OpInterp:
				if n := countStr(in.Strs, ""); n != len(in.Args) {
					return fail("shape", bid, i, "interp has %d holes for %d arguments", n, len(in.Args))
				}
			case midir.OpShell:
				if n := countStr(in.Strs, "$"); n != len(in.Args) {
					return fail("shape", bid, i, "shell has %d holes for %d arguments", n, len(in.Args))
				}
			case midir.OpBorrow:
				if in.Index != 0 && in.Index != 1 {
					return fail("shape", bid, i, "borrow index %d is neither shared nor mut", in.Index)
				}
			case midir.OpUnary:
				if in.Str == "cast" && in.Type == 0 {
					return fail("shape", bid, i, "cast needs its target type")
				}
			}
		}
	}
	return nil
}

func flagMisplaced(in midir.Inst) string {
	switch {
	case in.Op == midir.OpNop:
		return ""
	case in.Fixed && in.Op != midir.OpBoxDirect:
		return "is marked fixed, which only a box of a constant is"
	case in.Copy && in.Op != midir.OpShare:
		return "is marked as a copy, which only a share is"
	case in.Share && in.Op != midir.OpCopy:
		return "is marked shared, which only a copy is"
	case in.Async && in.Op != midir.OpFnItem && in.Op != midir.OpClosure && in.Op != midir.OpBind:
		return "is marked async, which only a function item, bind or closure is"
	case in.EndScope && in.Op != midir.OpDrop:
		return "is marked as a scope exit, which only a drop is"
	case in.Scoped && in.Op != midir.OpNewCell && in.Op != midir.OpDrop:
		return "is marked scoped, which only a cell or its drop is"
	}
	return ""
}

func numberText(in midir.Inst) bool {
	v, ok := new(big.Int).SetString(in.Str, 10)
	return ok && (in.Lit.Int == nil || in.Lit.Int.Cmp(v) == 0)
}

func arity(s midir.Shape) string {
	switch {
	case s.Max < 0:
		return "at least " + itoa(s.Min) + " arguments"
	case s.Min == s.Max:
		return itoa(s.Min) + " arguments"
	}
	return itoa(s.Min) + " to " + itoa(s.Max) + " arguments"
}

func itoa(n int) string { return strconv.Itoa(n) }

func countStr(ss []string, want string) int {
	n := 0
	for _, s := range ss {
		if s == want {
			n++
		}
	}
	return n
}
