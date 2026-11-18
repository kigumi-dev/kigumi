package plan

import (
	"kigumi/internal/compiler/mir/instance"
	midir "kigumi/internal/compiler/mir/ir"
	"kigumi/internal/compiler/sem"
)

// descSite is a body that the descriptor a box or record builds reaches: a method-table slot, or the drop hook.
type descSite struct {
	fn sem.EntityID
	t  sem.TypeID
}

type descRoot struct {
	descSite
	sp *spec
}

type descSiteKey struct {
	f        *midir.Func
	blk, idx int
	fn       sem.EntityID
}

// comptimeFuncs are the comptime blocks' bodies, which run the instantiations they name like any other body.
func (p *planner) comptimeFuncs() []*midir.Func {
	out := make([]*midir.Func, 0, len(p.Comptime))
	for _, cf := range p.Comptime {
		out = append(out, cf.Func)
	}
	return out
}

func (s *specializer) drainDescriptors() {
	s.drain()
	for s.rootDescriptors() {
		s.drain()
	}
}

// rootDescriptors instantiates, for every box and record built at a concrete type, the method-table and drop
// bodies that cannot run without that instantiation; it reports whether it left new work.
func (s *specializer) rootDescriptors() bool {
	for _, f := range append(s.p.comptimeFuncs(), s.p.Funcs...) {
		for bid := range f.Blocks {
			for i := range f.Blocks[bid].Insts {
				for _, d := range s.descSites(f, f.Blocks[bid].Insts[i]) {
					s.rootDescriptor(f, bid, i, d)
				}
			}
		}
	}
	return len(s.work) > 0
}

func (s *specializer) descSites(f *midir.Func, in midir.Inst) []descSite {
	r := s.p.R
	var t sem.TypeID
	var fns []sem.EntityID
	switch in.Op {
	case midir.OpBox, midir.OpLendBox:
		iface := f.Locals[in.Dst].Type
		if in.Op == midir.OpLendBox {
			iface = r.Types.Node(iface).Elem
		}
		t, fns = in.Type, r.Witnesses(in.Type, iface, r.Entity(f.Ent).Pkg)
	case midir.OpRecord:
		t, fns = f.Locals[in.Dst].Type, []sem.EntityID{r.TypeDecl(in.Ent).Drop}
	default:
		return nil
	}
	if !instance.Concrete(r, t) {
		return nil
	}
	var out []descSite
	for _, fn := range fns {
		if fn != 0 && s.p.ByEnt[fn] != nil && len(s.shapeParams()[fn]) > 0 {
			out = append(out, descSite{fn, t})
		}
	}
	return out
}

func (s *specializer) rootDescriptor(f *midir.Func, bid, idx int, d descSite) {
	key := descSiteKey{f, bid, idx, d.fn}
	if s.descSeen[key] {
		return
	}
	s.descSeen[key] = true
	args, ok := s.receiverShapeArgs(d.fn, d.t)
	if !ok {
		return
	}
	in := midir.Inst{Op: midir.OpCall, Ent: d.fn, Node: f.Blocks[bid].Insts[idx].Node}
	sp := s.get(f, s.byFunc[f], in, s.p.R.EraseLifetimes(args), nil)
	if sp == nil {
		return
	}
	sp.sites = append(sp.sites, &callSite{f: f, blk: bid, idx: idx, desc: true})
	s.descRoots = append(s.descRoots, descRoot{d, sp})
}

// publishDesc hands the backends the descriptor bodies that stayed live.
func (s *specializer) publishDesc() {
	live := map[*spec]bool{}
	for _, sp := range s.live() {
		live[sp] = true
	}
	for _, d := range s.descRoots {
		if live[d.sp] {
			s.p.Desc.Put(s.p.R, d.fn, d.t, d.sp.fn)
		}
	}
}
