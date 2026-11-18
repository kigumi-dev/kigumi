package plan

import (
	"fmt"
	"maps"
	"slices"

	"kigumi/internal/compiler/mir/instance"
	midir "kigumi/internal/compiler/mir/ir"
	"kigumi/internal/compiler/sem"
)

// addEnvParam gives generic function f its environment, a borrowed parameter after the declared ones: a runtime
// type is immortal, so a caller may pass its own environment any number of times.
func (ps *envPass) addEnvParam(f *midir.Func) {
	r := ps.p.R
	env, _ := instance.EnvType(r, f.Ent)
	l := newTypeLocal(r, f, "$env")
	f.Locals[l].Borrowed = true
	at := len(f.Params) - hiddenCount(r, f.Ent)
	f.Params = slices.Insert(f.Params, at, l)
	f.HasEnv, f.Env, f.EnvType = true, l, ps.p.Canon.Of(r, env)
}

// checkDropsCharged holds sem to each drop reading its instantiation here, by an environment or a body made per
// instantiation: sem must have charged the records of its type for that.
func (ps *envPass) checkDropsCharged(bodies []*midir.Func) error {
	r := ps.p.R
	for _, f := range bodies {
		if f.Ent == 0 || !r.IsDropFn(f.Ent) || r.GenericOf(f.Ent) != f.Ent {
			continue
		}
		if (f.HasEnv || instance.Bound(r, &ps.p.Desc, f.Ent)) && !r.DropReadsInstance(r.Fn(f.Ent).Owner) {
			return fmt.Errorf("internal error: %s reads an instantiation sem did not charge its records for", f.Name)
		}
	}
	return nil
}

// hiddenCount is the number of witness and const parameters fn takes after its declared ones.
func hiddenCount(r *sem.Result, fn sem.EntityID) int {
	info := r.Fn(fn)
	return len(info.Witnesses) + len(info.ConstParams)
}

func newTypeLocal(r *sem.Result, f *midir.Func, name string) midir.LocalID {
	f.Locals = append(f.Locals, midir.Local{Name: name, Type: r.Types.RtType()})
	return midir.LocalID(len(f.Locals) - 1)
}

// mintClosure copies lambda f to capture, after its own captures, the environment of the function it is written
// in when it reads that, and the arguments other type tests opened that it reads.
func (ps *envPass) mintClosure(f *midir.Func) {
	r := ps.p.R
	m := mintedClosure{env: ps.takesEnv(f)}
	var types []sem.TypeID
	if m.env {
		types = append(types, r.Types.RtType())
	}
	for _, p := range slices.Sorted(maps.Keys(ps.free[f])) {
		if ps.inScope(f, p) {
			continue
		}
		m.opened = append(m.opened, p)
		types = append(types, openedType(r, p))
	}
	c := f.Ent
	cv, extra := r.InstantiateClosureCapturing(c, types, "$rt")
	m.ent = cv
	g := ps.p.cloneFor(f, cv, ps.p.FuncName(c), map[sem.EntityID]sem.TypeID{c: r.Types.Closure(cv)}, map[sem.EntityID]sem.EntityID{c: cv})
	at := len(r.Closure(c).Captures)
	var params []midir.LocalID
	opened := map[sem.EntityID]midir.LocalID{}
	for i, ent := range extra {
		isEnv := m.env && i == 0
		var p sem.EntityID
		if !isEnv {
			p = m.opened[i-boolInt(m.env)]
		}
		g.Locals = append(g.Locals, midir.Local{Name: "$rt", Type: types[i], Ent: ent})
		l := midir.LocalID(len(g.Locals) - 1)
		params = append(params, l)
		if isEnv {
			env, _ := instance.EnvType(r, instance.Enclosing(r, r.ClosureOrigin(c)))
			g.HasEnv, g.Env, g.EnvType = true, l, ps.p.Canon.Of(r, env)
		} else {
			opened[p] = l
		}
	}
	g.Params = slices.Insert(g.Params, at, params...)
	ps.captured[g] = opened
	ps.p.Funcs = append(ps.p.Funcs, g)
	ps.p.ByEnt[cv] = g
	ps.minted[c] = m
}

// openedType is the type of the local an opened parameter's argument travels in: a runtime type, or a const
// parameter's value.
func openedType(r *sem.Result, p sem.EntityID) sem.TypeID {
	if info := r.TypeParam(p); info.IsConst {
		return info.ConstType
	}
	return r.Types.RtType()
}

func boolInt(b bool) int {
	if b {
		return 1
	}
	return 0
}
