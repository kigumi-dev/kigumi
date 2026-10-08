package mir

import "kigumi/internal/sem"

type spec struct {
	ent     sem.EntityID
	gen     sem.EntityID
	args    []sem.TypeID
	wit     []sem.WitnessArg
	fn      *Func
	parent  *spec
	root    *spec
	brought int
	capped  bool
	sites   []*callSite
	dead    bool
}

type callSite struct {
	f        *Func
	blk, idx int
	gone     bool
}

type specializer struct {
	p        *Program
	byKey    map[string]*spec
	byEnt    map[sem.EntityID]*spec
	byFunc   map[*Func]*spec
	all      []*spec
	work     []*spec
	names    map[string]bool
	noSpec   map[sem.EntityID]bool
	reported map[string]bool
}

// Specialize runs after plan-rc and fold-comptime and before plan-direct, so comptime executes the shared body.
func Specialize(p *Program) error {
	s := &specializer{p: p, byKey: map[string]*spec{}, byEnt: map[sem.EntityID]*spec{}, byFunc: map[*Func]*spec{}, names: map[string]bool{}, noSpec: map[sem.EntityID]bool{}, reported: map[string]bool{}}
	for _, f := range p.Funcs {
		s.names[f.Name] = true
	}
	for _, f := range append([]*Func(nil), p.Funcs...) {
		s.scan(f, nil)
	}
	for len(s.work) > 0 {
		sp := s.work[0]
		s.work = s.work[1:]
		s.devirtualize(sp)
		s.scan(sp.fn, sp)
	}
	s.settle()
	s.prune()
	return nil
}

func (s *specializer) scan(f *Func, ctx *spec) {
	for bid := range f.Blocks {
		insts := f.Blocks[bid].Insts
		for i := range insts {
			if insts[i].Op != OpCall {
				continue
			}
			// A copy of a body scanned before it was copied already names the instantiation.
			if sp := s.byEnt[insts[i].Ent]; sp != nil {
				sp.sites = append(sp.sites, &callSite{f: f, blk: bid, idx: i})
				continue
			}
			args, wit, ok := s.instance(f, ctx, insts[i])
			if !ok {
				continue
			}
			if sp := s.get(f, ctx, insts[i], args, wit); sp != nil {
				sp.sites = append(sp.sites, &callSite{f: f, blk: bid, idx: i})
				insts[i].Ent = sp.ent
			}
		}
	}
}

func (s *specializer) get(f *Func, ctx *spec, in Inst, args []sem.TypeID, wit []sem.WitnessArg) *spec {
	gen := in.Ent
	key := s.key(gen, args, wit)
	if sp, ok := s.byKey[key]; ok {
		return sp
	}
	if s.runaway(ctx, gen, args) {
		if !s.reported[key] {
			s.reported[key] = true
			s.warn(f, in.Node, "spec-runaway", s.p.R.Entity(gen).Name, s.argsText(args))
		}
		return nil
	}
	g := s.p.ByEnt[gen]
	if s.closureTyped(g) || s.overBudget(ctx) {
		return nil
	}
	ent := s.p.R.Instantiate(gen, args, key)
	sp := &spec{ent: ent, gen: gen, args: args, wit: wit, parent: ctx}
	charge(sp, ctx)
	sp.fn = s.p.cloneFor(g, ent, s.name(gen, args), s.p.R.InstanceSubst(gen, args))
	s.p.Funcs = append(s.p.Funcs, sp.fn)
	s.p.ByEnt[ent] = sp.fn
	s.byKey[key], s.byEnt[ent], s.byFunc[sp.fn] = sp, sp, sp
	s.all = append(s.all, sp)
	s.work = append(s.work, sp)
	return sp
}
