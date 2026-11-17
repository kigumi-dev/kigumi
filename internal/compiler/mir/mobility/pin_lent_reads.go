package mobility

import (
	"kigumi/internal/compiler/mir"
	midir "kigumi/internal/compiler/mir/ir"
	"kigumi/internal/compiler/sem"
)

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
						pinHolder(p, rs, g.Ent)
					}
				}
			}
		}
	}
}

func pinHolder(p *mir.Program, rs sem.ReadSite, site sem.EntityID) {
	r := p.R
	if r.ReadReleased(rs, site) || r.IsSync(r.Entity(rs.Local).Type) {
		return
	}
	h := r.FrameFn(r.HolderOf(rs))
	if body := p.ByEnt[h]; body == nil || !body.Async {
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
