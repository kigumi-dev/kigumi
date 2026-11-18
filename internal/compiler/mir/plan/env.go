package plan

import (
	"fmt"
	"slices"

	"kigumi/internal/compiler/mir/instance"
	midir "kigumi/internal/compiler/mir/ir"
	"kigumi/internal/compiler/sem"
)

// envPass gives each generic body whose runtime types depend on its type arguments one hidden value, the interned
// runtime type of the instantiation it runs for, and composes those types from it or from what a type test opened.
type envPass struct {
	p *planner
	// free are, per body, the parameters its runtime types depend on that the body does not open itself.
	free map[*midir.Func]map[sem.EntityID]bool
	// minted maps a closure that captures runtime types to its copy taking them as extra captures; captured are
	// the opened arguments such a copy holds.
	minted   map[sem.EntityID]mintedClosure
	captured map[*midir.Func]map[sem.EntityID]midir.LocalID
	// witnesses are, per body, the instantiations the calls in it bind their witness values to.
	witnesses map[*midir.Func]map[midir.LocalID]sem.TypeID
}

type mintedClosure struct {
	ent    sem.EntityID
	env    bool
	opened []sem.EntityID
}

func (p *planner) planEnvs() error {
	ps := &envPass{p: p, free: map[*midir.Func]map[sem.EntityID]bool{}, minted: map[sem.EntityID]mintedClosure{}, captured: map[*midir.Func]map[sem.EntityID]midir.LocalID{}, witnesses: map[*midir.Func]map[midir.LocalID]sem.TypeID{}}
	bodies := ps.bodies()
	ps.solve(bodies)
	for _, f := range bodies {
		if ps.takesEnv(f) && ps.p.R.Entity(f.Ent).Kind == sem.EntFn {
			ps.addEnvParam(f)
		}
	}
	if err := ps.checkDropsCharged(bodies); err != nil {
		return err
	}
	for _, f := range bodies {
		if ps.p.R.Entity(f.Ent).Kind != sem.EntClosure || len(ps.free[f]) == 0 {
			continue
		}
		if !ps.p.R.CapturesRuntimeTypes(f.Ent) {
			return fmt.Errorf("internal error: %s captures a runtime type sem did not charge it for", f.Name)
		}
		ps.mintClosure(f)
	}
	for _, f := range append(ps.bodies(), p.comptimeFuncs()...) {
		ps.rewrite(f)
	}
	ps.retypeClosures()
	p.Funcs = slices.DeleteFunc(p.Funcs, func(f *midir.Func) bool {
		_, copied := ps.minted[f.Ent]
		return copied
	})
	for _, f := range append(ps.bodies(), p.comptimeFuncs()...) {
		if f.Async && len(f.Slot) != len(f.Locals) {
			planFrame(f, p)
		}
	}
	return nil
}

// retypeClosures types the values of each lambda that now captures runtime types as its copy, wherever a type in
// a body holds the lambda's: a local, an instruction's type, the result.
func (ps *envPass) retypeClosures() {
	if len(ps.minted) == 0 {
		return
	}
	r := ps.p.R
	sub := map[sem.EntityID]sem.TypeID{}
	for c, m := range ps.minted {
		sub[c] = r.Types.Closure(m.ent)
	}
	retype := func(t *sem.TypeID) { *t = r.Types.Subst(*t, sub) }
	for _, f := range append(ps.bodies(), ps.p.comptimeFuncs()...) {
		if _, own := ps.minted[f.Ent]; own {
			continue
		}
		retype(&f.Ret)
		for i := range f.Locals {
			retype(&f.Locals[i].Type)
		}
		for b := range f.Blocks {
			for i := range f.Blocks[b].Insts {
				retype(&f.Blocks[b].Insts[i].Type)
			}
		}
	}
}

func (ps *envPass) bodies() []*midir.Func {
	return append([]*midir.Func(nil), ps.p.Funcs...)
}

// scope are the parameters f's environment carries: a generic function's own, or for a closure those of the
// function it is written in. A specialized copy has none.
func (ps *envPass) scope(f *midir.Func) []sem.EntityID {
	r := ps.p.R
	if f.Ent == 0 {
		return nil
	}
	switch r.Entity(f.Ent).Kind {
	case sem.EntFn:
		return instance.Scope(r, f.Ent)
	case sem.EntClosure:
		if r.ClosureSubst(f.Ent) != nil {
			return nil
		}
		return instance.Scope(r, instance.Enclosing(r, r.ClosureOrigin(f.Ent)))
	}
	return nil
}

func (ps *envPass) inScope(f *midir.Func, p sem.EntityID) bool {
	for _, q := range ps.scope(f) {
		if q == p {
			return true
		}
	}
	return false
}

// takesEnv reports whether f reads its environment.
func (ps *envPass) takesEnv(f *midir.Func) bool {
	for p := range ps.free[f] {
		if ps.inScope(f, p) {
			return true
		}
	}
	return false
}

func (ps *envPass) calleeTakesEnv(ent sem.EntityID) bool {
	f := ps.p.ByEnt[ent]
	return f != nil && ps.p.R.Entity(ent).Kind == sem.EntFn && ps.takesEnv(f)
}

// solve finds every body's free parameters: those of the runtime types it composes, of the environments of the
// calls and function values it makes, and of the lambdas it builds, less those its own type tests open.
func (ps *envPass) solve(bodies []*midir.Func) {
	for changed := true; changed; {
		changed = false
		for _, f := range bodies {
			opened := openedIn(f)
			free := ps.free[f]
			for _, t := range ps.siteTypes(f) {
				for _, p := range instance.Params(ps.p.R, t) {
					if _, own := opened[p]; !own && !free[p] {
						if free == nil {
							free = map[sem.EntityID]bool{}
							ps.free[f] = free
						}
						free[p], changed = true, true
					}
				}
			}
		}
	}
}

// openedIn maps the parameters f's type tests open to the locals that hold them.
func openedIn(f *midir.Func) map[sem.EntityID]midir.LocalID {
	out := map[sem.EntityID]midir.LocalID{}
	for _, blk := range f.Blocks {
		for _, in := range blk.Insts {
			if in.Op == midir.OpBuiltin && (in.Str == "type.arg" || in.Str == "type.const") && len(in.Holes) == 1 {
				out[in.Holes[0]] = in.Dst
			}
		}
	}
	return out
}
