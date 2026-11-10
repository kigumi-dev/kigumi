package mobility

import (
	"kigumi/internal/compiler/mir"
	midir "kigumi/internal/compiler/mir/ir"
	"kigumi/internal/compiler/sem"
)

// PinLentReads keeps an async fn on its lane when a closure handed on reads a non-Sync local of its frame in place: the
// closure may run in a task on that lane; a lambda's own locals are gone before the fn can suspend.
func PinLentReads(p *mir.Program) {
	r := p.R
	for _, g := range p.AllFuncs() {
		for _, blk := range g.Blocks {
			for _, in := range blk.Insts {
				if staysInFrame(in.Op) {
					continue
				}
				for k, a := range in.Args {
					if in.Op == midir.OpCallValue && k == 0 {
						continue
					}
					for _, rs := range r.ReadSites(g.Locals[a].Type) {
						pinHolder(p, rs)
					}
				}
			}
		}
	}
}

func pinHolder(p *mir.Program, rs sem.ReadSite) {
	r := p.R
	h := r.HolderOf(rs)
	if body := p.ByEnt[h]; body == nil || !body.Async || r.Entity(h).Kind != sem.EntFn || r.IsSync(r.Entity(rs.Local).Type) {
		return
	}
	if info := r.Fn(h); info.Mobility == sem.MobilityMigratable {
		info.Mobility, info.BoundBy, info.BoundRead = sem.MobilityExecutorBound, r.Entity(rs.Local).Name, rs
	}
}

// staysInFrame: these move, copy, lend, or end a value inside the frame; any other use hands it on.
func staysInFrame(op midir.Opcode) bool {
	switch op {
	case midir.OpAlias, midir.OpShare, midir.OpCopy, midir.OpMove, midir.OpBorrow, midir.OpDrop, midir.OpRelease:
		return true
	}
	return false
}
