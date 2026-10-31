package vm

import (
	"kigumi/internal/compiler/mir"
	midir "kigumi/internal/compiler/mir/ir"
	"kigumi/internal/compiler/sem"
)

// callPlanned runs an OpCall through its cached plan; a call with no plan resolves in callEntity each time.
func (m *Machine) callPlanned(in *midir.Inst, ic *instCode, args []*obj) *obj {
	p := m.planFor(in, ic)
	switch p.kind {
	case planHost:
		r := p.host(m, p.ent, args)
		m.chargeMemory(r)
		if p.lowered {
			m.releaseOwned(p.ent, args)
		}
		return r
	case planStd:
		r := m.std(p.key, args)
		m.chargeMemory(r)
		return r
	}
	return m.callEntity(in, args)
}

// callEntity runs a named callee: a lowered body, a std primitive, or an
// interface requirement dispatched on the receiver's dynamic type.
func (m *Machine) callEntity(in *midir.Inst, args []*obj) *obj {
	ent := in.Ent
	e := m.r.Entity(ent)
	if e.Kind == sem.EntFn {
		info := m.r.Fn(ent)
		if info.Abi != "" || info.Naked {
			panic(&Unsupported{What: "foreign call to " + e.Name})
		}
		if m.r.Entity(e.Parent).Kind == sem.EntInterface {
			return m.callMethod(args[0], e.Name, args[1:], info.Recv)
		}
	}
	f, lowered := m.lowered(ent)
	if e.Kind == sem.EntFn && e.Flags&sem.EfStd != 0 {
		key := m.stdKey(ent)
		if m.opts.Sandbox && hostEffect(key) {
			m.abort("`" + key + "` is not available at compile time")
		}
		if h, ok := m.hostFor(in.Prov, key); ok && m.hostWins(lowered) {
			r := h(m, ent, args)
			m.chargeMemory(r)
			if lowered {
				m.releaseOwned(ent, args)
			}
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

// releaseOwned drops what the lowered body would have consumed.
func (m *Machine) releaseOwned(fn sem.EntityID, args []*obj) {
	for i, a := range args {
		if !m.p.ParamLent(fn, i) {
			m.release(a)
		}
	}
}

// stdKey names a bodiless std function the way the runtime's table does.
func (m *Machine) stdKey(fn sem.EntityID) string { return mir.StdKey(m.r, fn, true) }

// closure builds a function value: a lowered body with its captures (or
// bound witnesses) in the environment, or a std primitive by key. A witness
// bound to a std function (rt_std_bind) follows callEntity's priority: the
// host accelerator wins over a real body when both exist.
func (m *Machine) closure(in *midir.Inst, ic *instCode, caps []*obj) *obj {
	c := newObj(kClosure)
	cl := c.clo()
	c.async = in.Async
	for _, x := range caps {
		retain(x)
	}
	cl.env = mkArray(caps)
	p := m.closurePlanFor(in, ic, len(caps))
	if cl.req = p.req; cl.req != nil {
		return c
	}
	if p.deny != "" {
		m.abort(p.deny)
	}
	if p.useLowered {
		cl.fn = p.f
		c.bind = in.Op == midir.OpBind
	} else {
		cl.stdKey, c.ent, cl.prov, cl.lent, cl.dyn = p.key, in.Ent, in.Prov, p.lent, p.dyn
	}
	return c
}

// callValue is rt_callv: the caller hands its arguments over.
func (m *Machine) callValue(fv *obj, args []*obj) *obj {
	fv = deref(fv)
	if fv.k != kClosure {
		m.abort("value is not callable")
	}
	if fv.creq() != nil {
		return m.callRequirementValue(fv, args)
	}
	if fv.async {
		return m.asyncValueCall(fv, args)
	}
	if fv.cstd() != "" {
		return m.callStdValue(fv, args)
	}
	r := m.invoke(fv.cfn(), fv.cenv(), fv.bind, args)
	m.resultTick(r, args)
	return r
}

// asyncValueCall is rt_callv on an async fn/method value: it builds the lazy Future a direct call does, with a closure of
// its own (fv is lent); the args, dedicated to this call, move into the Future.
func (m *Machine) asyncValueCall(fv *obj, args []*obj) *obj {
	raw := newObj(kClosure)
	rc, src := raw.clo(), fv.clo()
	rc.fn, rc.stdKey, rc.lent, rc.prov, rc.dyn = src.fn, src.stdKey, src.lent, src.prov, src.dyn
	raw.ent, raw.bind = fv.ent, fv.bind
	rc.env = retain(src.env)
	m.takeReferents(fv, args)
	return mkOpaque("Future", &future{fn: raw, args: args})
}

// takeReferents swaps each `&mut` cell the caller lends and takes back at once for its referent, and cuts an existential
// view's link to such a cell, as a direct call lends the place.
func (m *Machine) takeReferents(fv *obj, args []*obj) {
	if fv.cfn() == nil {
		return
	}
	off := 0
	if env := fv.cenv(); env != nil && !fv.bind {
		off = len(env.fields)
	}
	refs := m.p.ReferentParams(fv.cfn())
	for i, a := range args {
		switch {
		case off+i >= len(refs) || !refs[off+i] || a == nil:
		case a.k == kCell:
			args[i] = retain(a.inner)
			m.release(a)
		case a.k == kBox && a.b:
			a.fields = nil
		}
	}
}
