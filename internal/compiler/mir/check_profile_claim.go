package mir

import (
	midir "kigumi/internal/compiler/mir/ir"
	"kigumi/internal/compiler/sem"
)

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
	seen := map[blamedBox]bool{}
	reached := p.reachedFrom(roots)
	for _, r := range reached {
		f := r.f
		if f.Ent == 0 || !p.profileTrusts(f.Ent) {
			continue
		}
		for _, blk := range f.Blocks {
			for _, in := range blk.Insts {
				if in.Op == midir.OpBoxDirect && !in.Fixed && !p.R.InAllocatorBlock(f.File, in.Node) {
					out = p.blameBox(out, seen, r, c, in)
				}
			}
		}
	}
	return p.scalarBoxClaims(out, reached, c)
}

// reachedFrom walks the bodies a call, a function value or an entry can run from roots.
func (p *Program) reachedFrom(roots []sem.EntityID) []profileReach {
	seen := map[profileReach]bool{}
	var out, work []profileReach
	add := func(f *midir.Func, via profileReach) {
		if r := p.reachAt(f, via); f != nil && !seen[r] {
			seen[r] = true
			out, work = append(out, r), append(work, r)
		}
	}
	for _, r := range roots {
		add(p.ByEnt[r], profileReach{})
	}
	for len(work) > 0 {
		r := work[len(work)-1]
		work = work[:len(work)-1]
		add(r.f.Entry, r)
		add(r.f.Wraps, r)
		for _, blk := range r.f.Blocks {
			for _, in := range blk.Insts {
				switch in.Op {
				case midir.OpCall, midir.OpFnItem, midir.OpBind, midir.OpClosure, midir.OpFutureNew:
					add(p.ByEnt[in.Ent], p.via(r, in.Node))
				}
			}
		}
	}
	return out
}
