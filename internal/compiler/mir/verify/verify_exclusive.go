package verify

import (
	"slices"

	mirflow "kigumi/internal/compiler/mir/flow"
	midir "kigumi/internal/compiler/mir/ir"
)

// liveMutBorrow is one `&mut` borrow a path may still use: the local it
// borrows and the reference local that carries it.
type liveMutBorrow struct {
	base, ref midir.LocalID
}

// exclusiveFlow carries the live `&mut` borrows of one function across its
// blocks, together with the liveness that ends each of them.
type exclusiveFlow struct {
	f      *midir.Func
	live   []map[int]mirflow.Bitset // per block, live before each borrow and at exit
	used   []bool                   // a local is read somewhere in the body
	refIdx []int                    // compact index of each borrow reference local, -1 otherwise
	refs   int
}

// verifyExclusive rejects a local borrowed mutably twice while the first
// borrow is still live. Exclusivity is the point of `&mut`: the callee handed
// the second reference may write the referent while the holder of the first
// still reads it, so the two must not overlap.
//
// The IR names a place by a local, so a field or element borrow is a borrow of
// the temporary that holds the projection: `&mut p.a` and `&mut p.b` borrow
// two different locals and both may be live. A borrow lives from its OpBorrow
// until its reference is no longer live; a reference nothing reads stays live
// to the end of its block, since nothing there proves the borrow over.
func (p *verifier) verifyExclusive(f *midir.Func) *Error {
	if !hasMutBorrow(f) {
		return nil
	}
	x := newExclusiveFlow(f)
	in := make([][]liveMutBorrow, len(f.Blocks))
	reached := make([]bool, len(f.Blocks))
	reached[0] = true
	for changed := true; changed; {
		changed = false
		for _, bid := range mirflow.ReversePostorder(f) {
			if !reached[bid] {
				continue
			}
			out, _ := x.transfer(bid, in[bid], false)
			if diverges(f, &f.Blocks[bid]) {
				continue
			}
			for _, succ := range f.Blocks[bid].Term.Targets {
				if !reached[succ] {
					reached[succ] = true
					in[succ] = out
					changed = true
					continue
				}
				merged := mergeLiveMutBorrows(in[succ], out)
				if len(merged) != len(in[succ]) {
					in[succ] = merged
					changed = true
				}
			}
		}
	}
	// An unreachable block still has to be well formed: it enters with no
	// borrow outstanding, so a second `&mut` of one local there is as wrong
	// as on a reachable path.
	for bid := range f.Blocks {
		active := in[bid]
		if !reached[bid] {
			active = nil
		}
		if _, err := x.transfer(midir.BlockID(bid), active, true); err != nil {
			return err
		}
	}
	return nil
}

func hasMutBorrow(f *midir.Func) bool {
	for _, blk := range f.Blocks {
		for _, in := range blk.Insts {
			if in.Op == midir.OpBorrow && in.Index == 1 && len(in.Args) == 1 {
				return true
			}
		}
	}
	return false
}

func newExclusiveFlow(f *midir.Func) *exclusiveFlow {
	x := &exclusiveFlow{f: f, live: make([]map[int]mirflow.Bitset, len(f.Blocks)), used: make([]bool, len(f.Locals)), refIdx: make([]int, len(f.Locals))}
	for i := range x.refIdx {
		x.refIdx[i] = -1
	}
	for _, blk := range f.Blocks {
		for _, in := range blk.Insts {
			if isMutBorrow(&in) {
				x.refIdx[in.Dst] = x.refs
				x.refs++
			}
		}
	}
	x.liveness()
	for _, blk := range f.Blocks {
		for _, in := range blk.Insts {
			for _, a := range in.Args {
				x.used[a] = true
			}
		}
		for _, a := range blk.Term.Args {
			x.used[a] = true
		}
	}
	return x
}

// liveness is the backward liveness of every local: a borrow ends when its
// reference local is dead, unlike the defs rule's sets, which track only the
// locals that cross a block edge without a definition.
func (x *exclusiveFlow) liveness() {
	f := x.f
	words := (x.refs + 63) / 64
	entry := make([]mirflow.Bitset, len(f.Blocks))
	for i := range entry {
		entry[i] = make(mirflow.Bitset, words)
	}
	live := make(mirflow.Bitset, words)
	for changed := true; changed; {
		changed = false
		for _, bid := range mirflow.BackwardOrder(f) {
			blk := &f.Blocks[bid]
			clear(live)
			for _, t := range blk.Term.Targets {
				for w, v := range entry[t] {
					live[w] |= v
				}
			}
			for _, a := range blk.Term.Args {
				x.mark(live, a)
			}
			for i := len(blk.Insts) - 1; i >= 0; i-- {
				in := &blk.Insts[i]
				if in.Op.Yields() {
					x.unmark(live, in.Dst)
				}
				for _, a := range in.Args {
					x.mark(live, a)
				}
			}
			if !slices.Equal(entry[bid], live) {
				copy(entry[bid], live)
				changed = true
			}
		}
	}
	for bid := range f.Blocks {
		blk := &f.Blocks[bid]
		at := make(mirflow.Bitset, words)
		for _, t := range blk.Term.Targets {
			for w, v := range entry[t] {
				at[w] |= v
			}
		}
		for _, a := range blk.Term.Args {
			x.mark(at, a)
		}
		before := map[int]mirflow.Bitset{len(blk.Insts): at}
		live := at
		for i := len(blk.Insts) - 1; i >= 0; i-- {
			next := make(mirflow.Bitset, words)
			copy(next, live)
			in := &blk.Insts[i]
			if in.Op.Yields() {
				x.unmark(next, in.Dst)
			}
			for _, a := range in.Args {
				x.mark(next, a)
			}
			if isMutBorrow(in) {
				before[i] = next
			}
			live = next
		}
		x.live[bid] = before
	}
}

// transfer walks one block, ending the borrows whose reference is dead and
// rejecting a mutable borrow of a local another live borrow already covers.
func (x *exclusiveFlow) transfer(bid midir.BlockID, active []liveMutBorrow, check bool) ([]liveMutBorrow, *Error) {
	blk := &x.f.Blocks[bid]
	cur := active
	for i := range blk.Insts {
		in := &blk.Insts[i]
		if !isMutBorrow(in) {
			continue
		}
		cur = x.prune(cur, bid, i, false)
		base, ref := in.Args[0], in.Dst
		for _, b := range cur {
			if b.base == base {
				if check {
					return nil, fail("exclusive", bid, i, "borrows %%%s mutably while %%%s still holds a mutable borrow of it", x.f.Locals[base].Name, x.f.Locals[b.ref].Name)
				}
				break
			}
		}
		cur = append(cur, liveMutBorrow{base: base, ref: ref})
	}
	// A borrow the block ends without a later read ends here, so it does not
	// follow the edge: only a reference live past the block keeps it alive.
	return x.prune(cur, bid, len(blk.Insts), true), nil
}

// prune keeps the borrows whose reference can still be read at this point. A
// reference no instruction in the body reads is kept until its block ends, as
// nothing inside the block proves the borrow over.
func (x *exclusiveFlow) prune(active []liveMutBorrow, bid midir.BlockID, i int, exit bool) []liveMutBorrow {
	live, ok := x.live[bid][i]
	if !ok {
		return active
	}
	kept := make([]liveMutBorrow, 0, len(active))
	for _, b := range active {
		if idx := x.refIdx[b.ref]; idx >= 0 && !live.Has(midir.LocalID(idx)) && (exit || x.used[b.ref]) {
			continue
		}
		kept = append(kept, b)
	}
	return kept
}

func (x *exclusiveFlow) mark(live mirflow.Bitset, l midir.LocalID) {
	if i := x.refIdx[l]; i >= 0 {
		live.Set(midir.LocalID(i))
	}
}

func (x *exclusiveFlow) unmark(live mirflow.Bitset, l midir.LocalID) {
	if i := x.refIdx[l]; i >= 0 {
		live.Clear(midir.LocalID(i))
	}
}

func isMutBorrow(in *midir.Inst) bool {
	return in.Op == midir.OpBorrow && in.Index == 1 && len(in.Args) == 1
}

func mergeLiveMutBorrows(dst, src []liveMutBorrow) []liveMutBorrow {
	for _, b := range src {
		dup := false
		for _, have := range dst {
			if have == b {
				dup = true
				break
			}
		}
		if !dup {
			dst = append(dst, b)
		}
	}
	return dst
}
