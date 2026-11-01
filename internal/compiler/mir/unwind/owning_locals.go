package unwind

import (
	midir "kigumi/internal/compiler/mir/ir"
	"kigumi/internal/compiler/sem"
)

// OwningLocals marks the locals whose slot carries a reference: not a lent
// receiver, not a lease, and not a borrow of something else.
func OwningLocals(f *midir.Func, r *sem.Result) map[midir.LocalID]bool {
	owned := map[midir.LocalID]bool{}
	for i, l := range f.Locals {
		owned[midir.LocalID(i)] = !l.Borrowed && r.Types.Kind(l.Type) != sem.KRef
	}
	for _, prm := range f.Params {
		owned[prm] = true
	}
	defined := map[midir.LocalID]bool{}
	for _, prm := range f.Params {
		defined[prm] = true
	}
	for _, blk := range f.Blocks {
		for _, in := range blk.Insts {
			if !in.Op.Yields() {
				continue
			}
			defined[in.Dst] = true
			if in.Op == midir.OpBorrow || in.Op == midir.OpLendBox || (in.Op == midir.OpBuiltin && in.Str == "iter.at_ref") {
				owned[in.Dst] = false
			}
		}
	}
	for l := range owned {
		if !defined[l] {
			owned[l] = false
		}
	}
	return owned
}

// AliasParents maps a local to the one whose reference it stands for; the
// two share a reference, so an abandoned frame releases it once.
func AliasParents(f *midir.Func) map[midir.LocalID]midir.LocalID {
	parent := map[midir.LocalID]midir.LocalID{}
	defs := map[midir.LocalID]int{}
	for _, blk := range f.Blocks {
		for _, in := range blk.Insts {
			if !in.Op.Yields() {
				continue
			}
			defs[in.Dst]++
			if (in.Op == midir.OpAlias || in.Op == midir.OpBorrow) && len(in.Args) == 1 {
				parent[in.Dst] = in.Args[0]
			}
		}
	}
	for l, n := range defs {
		if n > 1 {
			delete(parent, l)
		}
	}
	return parent
}
