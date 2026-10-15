package mir

import "slices"

// CopyStore is a `t = share copy s` whose only use is the `setindex` after it in the same block.
type CopyStore struct {
	Block      BlockID
	Share, Set int
}

// CopyStores finds the copies a boxed array slot takes in place: it keeps its box and no box is built for t.
func (p *Program) CopyStores(f *Func) []CopyStore {
	if !f.DirectPlanned() {
		return nil
	}
	uses, defs := map[LocalID]int{}, map[LocalID]int{}
	for _, blk := range f.Blocks {
		for _, in := range blk.Insts {
			for _, a := range in.Args {
				uses[a]++
			}
			if in.Op.Yields() {
				defs[in.Dst]++
			}
		}
		for _, a := range blk.Term.Args {
			uses[a]++
		}
	}
	var out []CopyStore
	for bid, blk := range f.Blocks {
		for i, in := range blk.Insts {
			if in.Op != OpShare || !in.Copy || in.IfWritable || f.IsDirect(in.Args[0]) || !p.R.InPlaceCopy(f.Locals[in.Args[0]].Type) {
				continue
			}
			if uses[in.Dst] != 1 || defs[in.Dst] != 1 || f.Locals[in.Dst].Cell {
				continue
			}
			if j := p.storeAfter(f, blk.Insts, i); j > 0 {
				out = append(out, CopyStore{Block: BlockID(bid), Share: i, Set: j})
			}
		}
	}
	return out
}

func (p *Program) storeAfter(f *Func, insts []Inst, i int) int {
	src, dst := insts[i].Args[0], insts[i].Dst
	for j := i + 1; j < len(insts); j++ {
		in := insts[j]
		if in.Op == OpSetIndex && in.Args[2] == dst {
			if f.IsDirect(in.Args[0]) || in.Args[0] == dst || in.Args[0] == src {
				return 0
			}
			return j
		}
		if slices.Contains(in.Args, src) || slices.Contains(in.Args, dst) || in.Op.Yields() && in.Dst == src {
			return 0
		}
	}
	return 0
}

func (p *Program) copiesInPlace(f *Func, in Inst) bool {
	if in.Op != OpShare || !in.Copy || !f.DirectPlanned() || !p.R.InPlaceCopy(f.Locals[in.Args[0]].Type) {
		return false
	}
	for _, cs := range p.CopyStores(f) {
		if f.Blocks[cs.Block].Insts[cs.Share].Dst == in.Dst {
			return true
		}
	}
	return false
}
