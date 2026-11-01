package plan

import (
	"kigumi/internal/compiler/mir"
	midir "kigumi/internal/compiler/mir/ir"
	"kigumi/internal/compiler/sem"
)

type spec struct {
	ent     sem.EntityID
	gen     sem.EntityID
	args    []sem.TypeID
	wit     []sem.WitnessArg
	fn      *midir.Func
	sub     map[sem.EntityID]sem.TypeID
	clones  []funcPair
	parent  *spec
	root    *spec
	brought int
	capped  bool
	key     string
	refused bool
	sites   []*callSite
	dead    bool
	scalar  bool
	held    *heldRoot
}

type callSite struct {
	f        *midir.Func
	blk, idx int
	gone     bool
}

type specializer struct {
	p        *planner
	byKey    map[string]*spec
	byEnt    map[sem.EntityID]*spec
	byFunc   map[*midir.Func]*spec
	all      []*spec
	work     []*spec
	names    map[string]*nameHolder
	noSpec   map[sem.EntityID]bool
	reported map[string]bool
	scanned  map[*midir.Func]bool
	rooted   map[*midir.Func]*heldRoot
	scalars  bool
	bonus    map[string]int
	lent     map[lendParam]bool
	edits    []siteEdit
	// Rebuilt per dry run; a revert drops its caller's entry.
	reboxFlows map[*midir.Func]*reboxFlow
}

type siteEdit struct {
	f        *midir.Func
	blk, idx int
	was      sem.EntityID
}

// Specialize runs after plan-rc and before plan-direct, and comptime folds after plan-direct.
func Specialize(p *mir.Program) error { return (&planner{Program: p}).specialize() }

func (p *planner) specialize() error {
	mark := len(p.Diags)
	newSpecializer(p, nil).intrinsifyOwned()
	bonus := map[string]int{}
	for attempt := 0; ; attempt++ {
		s := newSpecializer(p, bonus)
		s.run()
		if attempt == maxReclaims || !s.reclaim(bonus) {
			break
		}
		s.rollback(mark)
	}
	p.inlineProtoCalls()
	p.dropStaleComptime()
	return nil
}

func newSpecializer(p *planner, bonus map[string]int) *specializer {
	s := &specializer{p: p, byKey: map[string]*spec{}, byEnt: map[sem.EntityID]*spec{}, byFunc: map[*midir.Func]*spec{}, names: map[string]*nameHolder{}, noSpec: map[sem.EntityID]bool{}, reported: map[string]bool{}, scanned: map[*midir.Func]bool{}, bonus: bonus}
	for _, f := range p.Funcs {
		s.names[f.Name] = nil
	}
	return s
}

func (s *specializer) run() {
	s.markRooted()
	for _, f := range append([]*midir.Func(nil), s.p.Funcs...) {
		s.scan(f, nil)
	}
	s.drain()
	s.scalarPhase()
	s.settle()
	s.prune()
}

func (s *specializer) scan(f *midir.Func, ctx *spec) {
	for bid := range f.Blocks {
		insts := f.Blocks[bid].Insts
		for i := range insts {
			if insts[i].Op != midir.OpCall && insts[i].Op != midir.OpFnItem {
				continue
			}
			// A copy of a body scanned before it was copied already names the instantiation.
			if sp := s.byEnt[insts[i].Ent]; sp != nil {
				s.hold(sp, s.heldIn(f, ctx))
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
				s.edits = append(s.edits, siteEdit{f, bid, i, insts[i].Ent})
				insts[i].Ent = sp.ent
			}
		}
	}
	s.scanned[f] = true
}

func (s *specializer) get(f *midir.Func, ctx *spec, in midir.Inst, args []sem.TypeID, wit []sem.WitnessArg) *spec {
	gen := in.Ent
	key := s.key(gen, args, wit)
	held := s.heldIn(f, ctx)
	if sp, ok := s.byKey[key]; ok {
		s.hold(sp, held)
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
	if s.noSpec[gen] || held != nil && s.overHeldLimit(f, in, held, gen, args) || held == nil && s.overBudget(ctx) {
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
	if s.leftGeneric(append([]*midir.Func{fn}, funcsOf(clones)...)) {
		s.noSpec[gen] = true
		return nil
	}
	sp := &spec{key: key, ent: ent, gen: gen, args: args, wit: wit, parent: ctx, fn: fn, sub: sub, clones: clones, scalar: s.scalars, held: held}
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

func (sp *spec) funcs() []*midir.Func { return append([]*midir.Func{sp.fn}, funcsOf(sp.clones)...) }

func funcsOf(ps []funcPair) []*midir.Func {
	out := make([]*midir.Func, len(ps))
	for i, p := range ps {
		out[i] = p.fn
	}
	return out
}
