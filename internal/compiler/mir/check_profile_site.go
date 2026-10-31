package mir

import (
	midir "kigumi/internal/compiler/mir/ir"
	"kigumi/internal/compiler/sem"
	"kigumi/internal/compiler/syntax"
)

// profileReach is a body a profile root reaches, with the last call a user body made on the way there.
type profileReach struct {
	f    *midir.Func
	from *midir.Func
	node syntax.NodeID
}

type blamedBox struct {
	file sem.FileID
	node syntax.NodeID
	typ  sem.TypeID
}

// A std body is walked once per user call that reaches it, so an allocator block around one call excuses only that call.
func (p *Program) reachAt(f *midir.Func, via profileReach) profileReach {
	if f == nil || p.userBody(f) {
		return profileReach{f: f}
	}
	return profileReach{f: f, from: via.from, node: via.node}
}

func (p *Program) via(r profileReach, n syntax.NodeID) profileReach {
	if !p.userBody(r.f) {
		return r
	}
	return profileReach{from: r.f, node: n}
}

func (p *Program) userBody(f *midir.Func) bool {
	return f.Ent != 0 && !p.R.Packages[p.R.Entity(f.Ent).Pkg].Std
}

// An instance is the built form of the shared body the profile judged.
func (p *Program) profileTrusts(ent sem.EntityID) bool {
	if inst, ok := p.R.InstanceOf(ent); ok {
		ent = inst.Gen
	}
	return p.R.ProfileTrusts(ent)
}

// blameBox reports a box in std at the user's call that reached it, the one place the user can change.
func (p *Program) blameBox(out []AllocClaim, seen map[blamedBox]bool, r profileReach, c sem.Code, in midir.Inst) []AllocClaim {
	at, n := r.f, in.Node
	if r.from != nil && !p.userBody(r.f) {
		at, n = r.from, r.node
	}
	k := blamedBox{at.File, n, in.Type}
	if seen[k] || p.R.InAllocatorBlock(at.File, n) {
		return out
	}
	seen[k] = true
	return append(out, p.claimAt(at, c, n, in.Type))
}
