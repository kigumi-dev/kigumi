package plan

import (
	"slices"

	midir "kigumi/internal/compiler/mir/ir"
	"kigumi/internal/compiler/sem"
)

// uncountTypes keeps f's runtime types out of reference counting, as they are immortal, and drops the reads of
// opened arguments that nothing uses.
func uncountTypes(r *sem.Result, f *midir.Func) {
	read, opens := make([]bool, len(f.Locals)), make([]bool, len(f.Locals))
	for _, blk := range f.Blocks {
		for _, in := range blk.Insts {
			if isOpen(in) {
				opens[in.Dst] = true
			}
			if in.Op == midir.OpDrop || in.Op == midir.OpRelease {
				continue
			}
			for _, a := range in.Args {
				read[a] = true
			}
			for _, a := range in.Rt {
				read[a] = true
			}
		}
		for _, a := range blk.Term.Args {
			read[a] = true
		}
	}
	gone := func(l midir.LocalID) bool { return r.Types.IsRtType(f.Locals[l].Type) || opens[l] && !read[l] }
	for bid := range f.Blocks {
		blk := &f.Blocks[bid]
		blk.Insts = slices.DeleteFunc(blk.Insts, func(in midir.Inst) bool {
			switch {
			case in.Op == midir.OpDrop || in.Op == midir.OpRelease:
				return gone(in.Args[0])
			case isOpen(in):
				return !read[in.Dst]
			}
			return false
		})
		for i := range blk.Insts {
			if in := &blk.Insts[i]; (in.Op == midir.OpShare || in.Op == midir.OpCopy) && r.Types.IsRtType(f.Locals[in.Dst].Type) {
				in.Op, in.Share, in.Copy = midir.OpAlias, false, false
			}
		}
	}
}

func isOpen(in midir.Inst) bool {
	return in.Op == midir.OpBuiltin && (in.Str == "type.arg" || in.Str == "type.const")
}
