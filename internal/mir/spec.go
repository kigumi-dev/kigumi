package mir

import "kigumi/internal/sem"

type spec struct {
	ent     sem.EntityID
	gen     sem.EntityID
	args    []sem.TypeID
	wit     []sem.WitnessArg
	fn      *Func
	sub     map[sem.EntityID]sem.TypeID
	clones  []funcPair
	parent  *spec
	root    *spec
	brought int
	capped  bool
	sites   []*callSite
	dead    bool
	scalar  bool
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
	names    map[string]*nameHolder
	noSpec   map[sem.EntityID]bool
	reported map[string]bool
	scanned  map[*Func]bool
	scalars  bool
}

// Specialize runs after plan-rc and fold-comptime and before plan-direct, so comptime executes the shared body.
func Specialize(p *Program) error {
	s := &specializer{p: p, byKey: map[string]*spec{}, byEnt: map[sem.EntityID]*spec{}, byFunc: map[*Func]*spec{}, names: map[string]*nameHolder{}, noSpec: map[sem.EntityID]bool{}, reported: map[string]bool{}, scanned: map[*Func]bool{}}
	for _, f := range p.Funcs {
		s.names[f.Name] = nil
	}
	s.intrinsifyOwned()
	for _, f := range append([]*Func(nil), p.Funcs...) {
		s.scan(f, nil)
	}
	s.drain()
	s.scalarPhase()
	s.settle()
	s.prune()
	p.inlineProtoCalls()
	return nil
}

func (s *specializer) scan(f *Func, ctx *spec) {
	for bid := range f.Blocks {
		insts := f.Blocks[bid].Insts
		for i := range insts {
			if insts[i].Op != OpCall && insts[i].Op != OpFnItem {
				continue
			}
			// A copy of a body scanned before it was copied already names the instantiation.
			if sp := s.byEnt[insts[i].Ent]; sp != nil {
				if !s.scanned[f] {
					sp.sites = append(sp.sites, &callSite{f: f, blk: bid, idx: i})
				}
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
	s.scanned[f] = true
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
	if s.noSpec[gen] || s.overBudget(ctx) {
		return nil
	}
	sub := s.p.R.InstanceSubst(gen, args)
	clones, ents, ok := s.closureClones(g, sub)
	if !ok {
		s.noSpec[gen] = true
		return nil
	}
	ent := s.p.R.Instantiate(gen, args, key)
	name, holder := s.name(gen, args, wit)
	fn := s.p.cloneFor(g, ent, name, sub, ents)
	holder.fn = fn
	if kind := s.p.R.ProtoKind(gen); kind != "" {
		if t, ok := s.p.protoSubject(fn); ok && s.p.R.ProtocolDirect(t, kind) {
			s.p.intrinsify(fn, kind, t)
		}
	}
	if s.leftGeneric(append([]*Func{fn}, funcsOf(clones)...)) {
		s.noSpec[gen] = true
		return nil
	}
	sp := &spec{ent: ent, gen: gen, args: args, wit: wit, parent: ctx, fn: fn, sub: sub, clones: clones, scalar: s.scalars}
	charge(sp, ctx)
	s.p.Funcs = append(s.p.Funcs, fn)
	s.p.ByEnt[ent] = fn
	s.byKey[key], s.byEnt[ent], s.byFunc[fn] = sp, sp, sp
	for _, c := range clones {
		var h *nameHolder
		c.fn.Name, h = s.instanceName(c.gen.Name, args, wit)
		h.fn = c.fn
		s.p.Funcs = append(s.p.Funcs, c.fn)
		s.p.ByEnt[c.fn.Ent] = c.fn
		s.byFunc[c.fn] = sp
	}
	s.all = append(s.all, sp)
	s.work = append(s.work, sp)
	return sp
}

func (sp *spec) funcs() []*Func { return append([]*Func{sp.fn}, funcsOf(sp.clones)...) }

func funcsOf(ps []funcPair) []*Func {
	out := make([]*Func, len(ps))
	for i, p := range ps {
		out[i] = p.fn
	}
	return out
}
