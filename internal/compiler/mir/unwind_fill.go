package mir

import (
	"cmp"
	"math/bits"
	"slices"
)

// unwindFill walks the blocks backward keeping the live owned locals as a descending list, so a panic point costs one pass over that list instead of over every word of the live bitset.
type unwindFill struct {
	f        *Func
	plan     *UnwindPlan
	owned    map[LocalID]bool
	parent   map[LocalID]LocalID
	aliased  []bool
	held     func(in *Inst, i int) bool
	words    int
	interned map[uint64][]int32
	cur      []LocalID
	scratch  []LocalID
}

func (u *unwindFill) block(bid int, liveIn []bitset) {
	blk := &u.f.Blocks[bid]
	live := make(bitset, u.words)
	for _, t := range blk.Term.Targets {
		for w := range live {
			live[w] |= liveIn[t][w]
		}
	}
	for _, a := range blk.Term.Args {
		if u.owned[a] {
			live.set(a)
		}
	}
	u.cur = u.cur[:0]
	for w := len(live) - 1; w >= 0; w-- {
		for x := live[w]; x != 0; {
			b := bits.Len64(x) - 1
			x &^= 1 << b
			u.cur = append(u.cur, LocalID(w*64+b))
		}
	}
	for i := len(blk.Insts) - 1; i >= 0; i-- {
		in := &blk.Insts[i]
		u.visit(bid, i, in, live)
		if shapes[in.Op].result && live.has(in.Dst) {
			live.clear(in.Dst)
			u.remove(in.Dst)
		}
		for _, a := range in.Args {
			if u.owned[a] && !live.has(a) {
				live.set(a)
				u.insert(a)
			}
		}
	}
}

func descending(e, t LocalID) int { return cmp.Compare(t, e) }

func (u *unwindFill) insert(l LocalID) {
	if i, found := slices.BinarySearchFunc(u.cur, l, descending); !found {
		u.cur = slices.Insert(u.cur, i, l)
	}
}

func (u *unwindFill) remove(l LocalID) {
	if i, found := slices.BinarySearchFunc(u.cur, l, descending); found {
		u.cur = slices.Delete(u.cur, i, i+1)
	}
}

func (u *unwindFill) visit(bid, i int, in *Inst, live bitset) {
	if cannotPanic(in.Op) {
		return
	}
	u.plan.At[bid][i] = 0
	src, lv := u.cur, live
	if u.held != nil {
		src, lv = u.withHeld(in, live)
	}
	set := u.scratch[:0]
	result := shapes[in.Op].result
	for _, id := range src {
		if result && in.Dst == id || u.aliased[id] && sharesOwner(id, u.parent, u.owned, lv, in) {
			continue
		}
		set = append(set, id)
	}
	u.scratch = set
	if len(set) == 0 {
		return
	}
	var h uint64 = 14695981039346656037
	for _, id := range set {
		h = (h ^ uint64(id)) * 1099511628211
	}
	for _, idx := range u.interned[h] {
		if slices.Equal(u.plan.Sets[idx], set) {
			u.plan.At[bid][i] = idx
			return
		}
	}
	idx := int32(len(u.plan.Sets))
	u.plan.Sets = append(u.plan.Sets, slices.Clone(set))
	u.interned[h] = append(u.interned[h], idx)
	u.plan.At[bid][i] = idx
}

// withHeld adds the boxes in's call site still releases, which the walk counted as handed over.
func (u *unwindFill) withHeld(in *Inst, live bitset) ([]LocalID, bitset) {
	src, lv := u.cur, live
	for i, a := range in.Args {
		if u.owned[a] && !lv.has(a) && u.held(in, i) {
			if &lv[0] == &live[0] {
				lv = slices.Clone(live)
				src = slices.Clone(u.cur)
			}
			lv.set(a)
			j, _ := slices.BinarySearchFunc(src, a, descending)
			src = slices.Insert(src, j, a)
		}
	}
	return src, lv
}
