package mir

import "kigumi/internal/sem"

// profileClaims reports a box in a body the allocation profile accepted on the promise that the crossing stays unboxed.
func (p *Program) profileClaims() []AllocClaim {
	roots, flag := p.R.AllocProfileRoots()
	if len(roots) == 0 {
		return nil
	}
	code := "box-without-default-allocator"
	if flag == "--profile recoverable-only" {
		code = "box-in-recoverable-only"
	}
	c, ok := sem.CodeByName(code)
	if !ok {
		return nil
	}
	var out []AllocClaim
	for _, f := range p.reachedFrom(roots) {
		if f.Ent == 0 || !p.R.ProfileTrusts(f.Ent) {
			continue
		}
		for _, blk := range f.Blocks {
			for _, in := range blk.Insts {
				if in.Op == OpBoxDirect && !in.Fixed && !p.R.InAllocatorBlock(f.File, in.Node) {
					out = append(out, p.claimAt(f, c, in.Node, in.Type))
				}
			}
		}
	}
	return out
}

// reachedFrom walks the bodies a call, a function value or an entry can run from roots.
func (p *Program) reachedFrom(roots []sem.EntityID) []*Func {
	seen := map[*Func]bool{}
	var out, work []*Func
	add := func(f *Func) {
		if f != nil && !seen[f] {
			seen[f] = true
			out, work = append(out, f), append(work, f)
		}
	}
	for _, r := range roots {
		add(p.ByEnt[r])
	}
	for len(work) > 0 {
		f := work[len(work)-1]
		work = work[:len(work)-1]
		add(f.Entry)
		add(f.Wraps)
		for _, blk := range f.Blocks {
			for _, in := range blk.Insts {
				switch in.Op {
				case OpCall, OpFnItem, OpBind, OpClosure, OpFutureNew:
					add(p.ByEnt[in.Ent])
				}
			}
		}
	}
	return out
}
