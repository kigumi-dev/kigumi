package mir

import "kigumi/internal/compiler/sem"

type joinArm struct {
	blk  BlockID
	idx  int
	lent bool
}

// noteJoinArm keeps a reference join's count uniform: once a counted branch meets a lent one, every lent branch takes its own count.
func (b *builder) noteJoinArm(dst LocalID) {
	if b.fn.Locals[dst].Ent != 0 || b.r.Types.Kind(b.fn.Locals[dst].Type) != sem.KRef {
		return
	}
	if b.joins == nil {
		b.joins = map[LocalID][]joinArm{}
	}
	arms := append(b.joins[dst], joinArm{b.cur, len(b.block().Insts) - 1, b.unowned[dst]})
	b.joins[dst] = arms
	lent, counted := false, false
	for _, a := range arms {
		lent = lent || a.lent
		counted = counted || !a.lent
	}
	if lent && counted {
		for i, a := range arms {
			if a.lent {
				b.fn.Blocks[a.blk].Insts[a.idx].Share = true
				arms[i].lent = false
			}
		}
	}
	b.unowned[dst] = lent && !counted
}
