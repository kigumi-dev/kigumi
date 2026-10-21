package mir

import "kigumi/internal/sem"

// Only locals read before a definition in their own block need a bit; the rest are defined wherever they are read.
// The sets at block entry keep only locals live there, so they stay small however long the function is.
type defFlow struct {
	f         *Func
	live      *edgeLive
	stamp     []int32
	cur       int32
	must, may wordSet
}

func (d *defFlow) define(in *Inst) {
	if i := d.live.idx[in.Dst]; i >= 0 {
		d.stamp[i] = d.cur
	}
}

// verifyDefs checks definition before use. A temporary must be assigned
// on every path to each of its uses; a named local must be assigned on at
// least one, because the checker narrows the reads of a pattern binding
// behind `&&` to the matched path, which the CFG cannot see.
func verifyDefs(f *Func) *Error {
	f.weakDefs = nil
	if f.live == nil {
		defer func() { f.live = nil }()
	}
	live := f.edgeLive()
	d := &defFlow{f: f, live: live, stamp: make([]int32, len(live.locals))}
	must := make([]wordSet, len(f.Blocks))
	may := make([]wordSet, len(f.Blocks))
	seen := make([]bool, len(f.Blocks))
	pending := make([]bool, len(f.Blocks))
	pending[0] = true
	order := reversePostorder(f)
	for again := true; again; {
		again = false
		for _, bid := range order {
			if !pending[bid] {
				continue
			}
			pending[bid] = false
			if diverges(f, &f.Blocks[bid]) {
				continue
			}
			outMust, outMay := must[bid].union(live.defs[bid]), may[bid].union(live.defs[bid])
			for _, succ := range f.Blocks[bid].Term.Targets {
				m, y := outMust.inter(live.in[succ]), outMay.inter(live.in[succ])
				if !seen[succ] {
					seen[succ] = true
					must[succ], may[succ] = m, y
					pending[succ], again = true, true
					continue
				}
				nm, ny := must[succ].inter(m), may[succ].union(y)
				if !nm.equal(must[succ]) || !ny.equal(may[succ]) {
					must[succ], may[succ] = nm, ny
					pending[succ], again = true, true
				}
			}
		}
	}
	seen[0] = true
	for bid := range f.Blocks {
		if !seen[bid] {
			continue
		}
		if err := d.check(BlockID(bid), must[bid], may[bid]); err != nil {
			return err
		}
	}
	return nil
}

// diverges reports a block that never reaches its terminator: a panic or
// a call that yields Never ends the function, so the builder's jump after
// it is dead and the block defines nothing for its successors.
func diverges(f *Func, blk *Block) bool {
	for _, in := range blk.Insts {
		if in.Op == OpPanic || (in.Op == OpBuiltin && in.Str == "panic") {
			return true
		}
		if in.Op.Yields() && in.Op != OpUnit && f.Locals[in.Dst].Type == sem.TyNever {
			return true
		}
	}
	return false
}

func (d *defFlow) defined(l LocalID) bool {
	i := d.live.idx[l]
	if i < 0 {
		return true
	}
	added := d.stamp[i] == d.cur
	must, may := added || d.must.has(i), added || d.may.has(i)
	if d.f.Locals[l].Ent != 0 {
		if may && !must {
			if d.f.weakDefs == nil {
				d.f.weakDefs = make([]bool, len(d.f.Locals))
			}
			d.f.weakDefs[l] = true
		}
		return may
	}
	return must
}

func (d *defFlow) check(bid BlockID, inMust, inMay wordSet) *Error {
	f := d.f
	d.cur++
	d.must, d.may = inMust, inMay
	blk := &f.Blocks[bid]
	for i := range blk.Insts {
		in := &blk.Insts[i]
		for _, a := range in.Args {
			// Every backend starts a named local null, so releasing one no path assigned is a no-op.
			if in.Op == OpDrop && f.Locals[a].Ent != 0 {
				continue
			}
			if !d.defined(a) {
				return fail("defs", bid, i, "%s reads %%%s before any definition", in.Op, f.Locals[a].Name)
			}
		}
		if in.Op.Yields() {
			d.define(in)
		}
	}
	for _, a := range blk.Term.Args {
		if !d.defined(a) {
			return fail("defs", bid, len(blk.Insts), "terminator reads %%%s before any definition", f.Locals[a].Name)
		}
	}
	return nil
}
