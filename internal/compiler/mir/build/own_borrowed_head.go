package build

import (
	"slices"

	midir "kigumi/internal/compiler/mir/ir"
)

// ownBorrowedSource releases a for-loop's borrowed-head source when it is a temporary: a projection
// retains on read while the `&` alias has nothing to release; a bare local or parameter retains nothing.
func (b *builder) ownBorrowedSource(iter midir.LocalID) {
	insts := b.block().Insts
	if len(insts) == 0 {
		return
	}
	last := insts[len(insts)-1]
	if last.Op != midir.OpBorrow || last.Dst != iter || len(last.Args) != 1 {
		return
	}
	if src := last.Args[0]; b.fn.Locals[src].Ent == 0 {
		b.own(src)
		b.ownProjectedRoots(src)
	}
}

// ownProjectedRoots keeps the temporary a head borrows a field path of alive for the loop, so its drop
// cannot run, and empty what the loop iterates, before the last iteration.
func (b *builder) ownProjectedRoots(l midir.LocalID) {
	for {
		def, ok := b.definingInst(l)
		if !ok || def.Op != midir.OpField || b.fn.Locals[def.Args[0]].Ent != 0 {
			return
		}
		l = def.Args[0]
		if root, ok := b.definingInst(l); ok && !projection(root.Op) {
			if !b.owned(l) {
				b.own(l)
			}
			return
		}
	}
}

func (b *builder) definingInst(l midir.LocalID) (midir.Inst, bool) {
	for _, blk := range b.fn.Blocks {
		for _, in := range blk.Insts {
			if in.Dst == l && in.Op.Yields() {
				return in, true
			}
		}
	}
	return midir.Inst{}, false
}

// projection: the instruction reads a value something else keeps (a field, an element, a cell's
// content, a borrow), so the loop has no temporary of its own to keep alive.
func projection(op midir.Opcode) bool {
	switch op {
	case midir.OpField, midir.OpIndex, midir.OpPayload, midir.OpUnbox, midir.OpCellGet, midir.OpBorrow, midir.OpAlias, midir.OpGlobalGet:
		return true
	}
	return false
}

func (b *builder) owned(l midir.LocalID) bool {
	for _, s := range b.scopes {
		if slices.Contains(s.owned, l) {
			return true
		}
	}
	return false
}
