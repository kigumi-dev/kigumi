package vm

import (
	"kigumi/internal/mir"
	"kigumi/internal/sem"
)

// callEntity runs a named callee: a lowered body, a std primitive, or an
// interface requirement dispatched on the receiver's dynamic type.
func (m *Machine) callEntity(in *mir.Inst, args []*obj) *obj {
	ent := in.Ent
	e := m.r.Entity(ent)
	if e.Kind == sem.EntFn {
		info := m.r.Fn(ent)
		if info.Abi != "" || info.Naked {
			panic(&Unsupported{What: "foreign call to " + e.Name})
		}
		if m.r.Entity(e.Parent).Kind == sem.EntInterface {
			return m.callMethod(args[0], e.Name, args[1:], info.Recv == sem.RecvMut)
		}
	}
	f, lowered := m.p.ByEnt[ent]
	if e.Kind == sem.EntFn && e.Flags&sem.EfStd != 0 {
		key := m.stdKey(ent)
		if m.opts.Sandbox && hostEffect(key) {
			m.abort("`" + key + "` is not available at compile time")
		}
		if h, ok := m.hostFor(in.Prov, key); ok && m.hostWins(lowered) {
			r := h(m, ent, args)
			m.chargeMemory(r)
			return r
		}
	}
	if lowered {
		return m.call(f, args)
	}
	r := m.std(m.stdKey(ent), args)
	m.chargeMemory(r)
	return r
}

// stdKey names a bodiless std function the way the runtime's table does.
func (m *Machine) stdKey(fn sem.EntityID) string { return mir.StdKey(m.r, fn, true) }

// closure builds a function value: a lowered body with its captures (or
// bound witnesses) in the environment, or a std primitive by key. A witness
// bound to a std function (rt_std_bind) follows callEntity's priority: the
// host accelerator wins over a real body when both exist.
func (m *Machine) closure(in *mir.Inst, caps []*obj) *obj {
	c := newObj(kClosure)
	c.async = in.Async
	for _, x := range caps {
		retain(x)
	}
	c.env = mkArray(caps)
	if e := m.r.Entity(in.Ent); in.Op == mir.OpClosure && e.Kind == sem.EntFn && m.r.Entity(e.Parent).Kind == sem.EntInterface {
		c.req = &reqCall{name: e.Name, witness: in.Str == "witness"}
		return c
	}
	f, lowered := m.p.ByEnt[in.Ent]
	useLowered := lowered
	if e := m.r.Entity(in.Ent); e.Kind == sem.EntFn && e.Flags&sem.EfStd != 0 {
		key := m.stdKey(in.Ent)
		if m.opts.Sandbox && hostEffect(key) {
			m.abort("`" + key + "` is not available at compile time")
		}
		if _, ok := m.hostFor(in.Prov, key); ok && m.hostWins(lowered) {
			useLowered = false
		}
	}
	if useLowered {
		c.fn = f
		c.bind = in.Op == mir.OpBind
	} else {
		c.stdKey, c.ent, c.prov = m.stdKey(in.Ent), in.Ent, in.Prov
		c.lent = mir.LentMask(m.r, in.Ent)
		c.dyn = m.curFn.Locals[in.Dst].Type
	}
	return c
}

// callValue is rt_callv: the caller hands its arguments over.
func (m *Machine) callValue(fv *obj, args []*obj) *obj {
	fv = deref(fv)
	if fv.k != kClosure {
		m.abort("value is not callable")
	}
	if fv.req != nil {
		return m.callRequirementValue(fv, args)
	}
	if fv.async {
		return m.asyncValueCall(fv, args)
	}
	if fv.stdKey != "" {
		all := append(append([]*obj{}, fv.env.fields...), args...)
		// A primitive whose meaning depends on its declaration (a numeric
		// conversion) is dispatched as if called directly.
		site, siteFn := m.site, m.siteFn
		m.site, m.siteFn = m.valueSite(fv, len(fv.env.fields))
		var r *obj
		if h, ok := m.hostFor(fv.prov, fv.stdKey); ok {
			r = h(m, fv.ent, all)
		} else {
			r = m.std(fv.stdKey, all)
		}
		m.chargeMemory(r)
		m.site, m.siteFn = site, siteFn
		// A bound primitive lends its receiver and borrowed parameters
		// (rt_std_bind).
		for i, a := range args {
			if fv.lent>>i&1 == 0 {
				m.release(a)
			}
		}
		return r
	}
	return m.invoke(fv.fn, fv.env, fv.bind, args)
}

// asyncValueCall is rt_callv on an async fn/method value: it builds the
// same lazy Future a direct call does, instead of
// running the body. fv is lent (its own last use is released elsewhere),
// so the Future keeps an independent closure of its own; args arrive
// already dedicated to this call and move into the Future unchanged.
func (m *Machine) asyncValueCall(fv *obj, args []*obj) *obj {
	raw := newObj(kClosure)
	raw.fn, raw.stdKey, raw.ent, raw.bind, raw.lent, raw.prov, raw.dyn = fv.fn, fv.stdKey, fv.ent, fv.bind, fv.lent, fv.prov, fv.dyn
	raw.env = retain(fv.env)
	return mkOpaque("Future", &future{fn: raw, args: args})
}

// invoke is the adapter convention: the callee takes a retained reference
// to every argument, the environment supplies the captured leading
// parameters (or the bound trailing ones), and the argument array is
// released afterwards.
func (m *Machine) invoke(f *mir.Func, env *obj, bind bool, args []*obj) *obj {
	var params []*obj
	nenv := 0
	if env != nil {
		nenv = len(env.fields)
	}
	if bind {
		for _, a := range args {
			params = append(params, retain(a))
		}
		for i := 0; i < nenv; i++ {
			params = append(params, retain(env.fields[i]))
		}
	} else {
		for i := 0; i < nenv; i++ {
			params = append(params, retain(env.fields[i]))
		}
		for _, a := range args {
			params = append(params, retain(a))
		}
	}
	r := m.call(f, params)
	m.releaseLentEnv(f, params, len(args), bind)
	// Reverse, as a direct call's arguments drop and rt_core.c's invoke releases them.
	for i := len(args) - 1; i >= 0; i-- {
		m.release(args[i])
	}
	return r
}

// releaseLentEnv drops the references invoke took for captured parameters the
// callee only borrows, so the callee has nothing to release them.
func (m *Machine) releaseLentEnv(f *mir.Func, params []*obj, nargs int, bind bool) {
	kind := m.r.Entity(f.Ent).Kind
	if len(params) == nargs || kind != sem.EntFn && kind != sem.EntClosure {
		return
	}
	first, end := 0, len(params)-nargs
	if bind {
		first, end = nargs, len(params)
	}
	for i := first; i < end; i++ {
		if kind == sem.EntClosure && m.r.Types.Kind(f.Locals[f.Params[i]].Type) == sem.KRef || kind == sem.EntFn && m.p.ParamLent(f.Ent, i) {
			m.release(params[i])
		}
	}
}
