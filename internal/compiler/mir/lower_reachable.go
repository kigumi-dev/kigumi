package mir

import (
	"cmp"
	"slices"

	"kigumi/internal/compiler/hir"
	"kigumi/internal/compiler/sem"
)

// lowering is Build's state: which top-level body is being lowered, and which body each Func came from.
type lowering struct {
	cur    sem.EntityID
	top    map[*Func]sem.EntityID
	tried  map[sem.EntityID]bool
	checks []pendingCheck
}

func (p *Program) lowerable(id sem.EntityID) bool {
	if !p.hasLowerableBody(id) {
		return false
	}
	if p.R.BodySkipped(id) {
		p.Skipped = append(p.Skipped, id)
		return false
	}
	return true
}

func (p *Program) hasLowerableBody(id sem.EntityID) bool {
	if _, minted := p.R.InstanceOf(id); minted {
		return false
	}
	return hir.Lowerable(p.R, id)
}

// lowerReachable lowers the roots in entity order, then whatever the reachability walk finds from them.
func (p *Program) lowerReachable(all bool) {
	p.lowering = &lowering{top: map[*Func]sem.EntityID{}, tried: map[sem.EntityID]bool{}}
	roots := p.lowerRoots(all)
	for _, id := range roots {
		p.lowerTop(id)
	}
	if !all {
		w := newReachWalk(p, true)
		for _, id := range roots {
			if w.rs.add(id, Reason{}) {
				w.worklist = append(w.worklist, id)
			}
		}
		for _, cf := range p.Comptime {
			w.scan(cf.Func, cf.Owner.Ent, p.R.Entity(cf.Owner.Ent).Pkg)
		}
		w.drain()
	}
	p.runPendingChecks()
	top := p.lowering.top
	slices.SortStableFunc(p.Funcs, func(a, b *Func) int { return cmp.Compare(top[a], top[b]) })
	p.lowering = nil
	if !all {
		p.unlowered = make([]bool, len(p.R.Entities))
		for i := 1; i < len(p.unlowered); i++ {
			p.unlowered[i] = p.ByEnt[sem.EntityID(i)] == nil && p.hasLowerableBody(sem.EntityID(i))
		}
	}
}

// lowerRoots lists, in entity order, the bodies lowered whether or not anything reaches them.
func (p *Program) lowerRoots(all bool) []sem.EntityID {
	r := p.R
	comptime := map[sem.EntityID]bool{}
	for _, site := range r.Comptime {
		comptime[p.topOf(site.Fn)] = true
	}
	main := r.MainFn()
	var roots []sem.EntityID
	for i := 1; i < len(r.Entities); i++ {
		id := sem.EntityID(i)
		if p.hasLowerableBody(id) && (all || comptime[id] || p.alwaysLowered(id, main) && p.keepsRoot(id)) && p.lowerable(id) {
			roots = append(roots, id)
		}
	}
	return roots
}

func (p *Program) keepsRoot(id sem.EntityID) bool {
	return p.rootKeep == nil || p.rootKeep(p.R.Packages[p.R.Entity(id).Pkg].Path)
}

func (p *Program) alwaysLowered(id, main sem.EntityID) bool {
	r := p.R
	e := r.Entity(id)
	if pkg := r.Packages[e.Pkg]; !pkg.Std && pkg.Module == "" {
		return true
	}
	switch e.Kind {
	case sem.EntTest, sem.EntImplicitMain:
		return true
	}
	return id == main || r.Fn(id).Export != ""
}

func (p *Program) topOf(id sem.EntityID) sem.EntityID {
	for id != 0 && p.R.Entity(id).Kind == sem.EntClosure {
		id = p.R.Entity(id).Parent
	}
	return id
}

func (p *Program) lowerTop(id sem.EntityID) {
	l := p.lowering
	if l.tried[id] || !p.lowerable(id) {
		return
	}
	l.tried[id] = true
	saved := l.cur
	l.cur = id
	p.build(hir.Lower(p.R, id))
	l.cur = saved
}

// calleeBody is ent's MIR only if lowering every body in entity order would have built it before the current one,
// so a borrow check decides the same however much of the program is lowered.
func (p *Program) calleeBody(ent sem.EntityID) (*Func, bool) {
	l := p.lowering
	if l != nil {
		if ent >= l.cur || !p.lowerable(ent) {
			return nil, false
		}
		p.lowerTop(ent)
	}
	f, ok := p.ByEnt[ent]
	return f, ok
}
