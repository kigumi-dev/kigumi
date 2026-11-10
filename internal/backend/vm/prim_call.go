package vm

import "kigumi/internal/compiler/sem"

// callPrim answers a call with the host function h, or the bodiless primitive key when h is nil;
// SecretSan and the two-run trace see every primitive's arguments here and nowhere else.
func (m *Machine) callPrim(ent sem.EntityID, key string, h hostFn, args []*obj) *obj {
	if m.opts.SecretSan || m.trace != nil {
		m.primSinks(key, h != nil, args)
		defer m.watchBranching(primArgTable[key], args)()
	}
	var r *obj
	if h != nil {
		r = h(m, ent, args)
	} else {
		r = m.std(key, args)
	}
	m.chargeMemory(r)
	if !m.opts.SecretSan {
		return r
	}
	if m.secretPrimOf(ent) == primDeclassify {
		return m.untaint(r)
	}
	m.joinWritable(ent, args)
	return m.stdResultTaint(key, args, r)
}

func (m *Machine) primSinks(key string, host bool, args []*obj) {
	if host && hostEffect(key) {
		m.checkHostIO(args, key)
	}
	m.argSinks(key, primArgTable[key], args)
}

// argSinks applies the index, address and size sinks and records the trace events of a call's integer arguments.
func (m *Machine) argSinks(op string, roles []primArg, args []*obj) {
	for _, a := range roles {
		if a.at >= len(args) {
			continue
		}
		switch a.role {
		case roleIndex:
			m.traceIndexAt(op, args[a.of], args[a.at])
			m.checkIndex(args[a.at])
		case roleAddress:
			m.traceIndexAt(op, nil, args[a.at])
			m.checkAddress(args[a.at])
		case roleSize:
			m.checkAllocSize(args[a.at], op)
		}
	}
}

// joinWritable joins every argument into each argument the callee may write, as a primitive may store any input
// there: their whole labels into its content, their shapes into its shape (a builder grows by what it appends).
func (m *Machine) joinWritable(ent sem.EntityID, args []*obj) {
	if ent == 0 || m.r.Entity(ent).Kind != sem.EntFn {
		return
	}
	t := label{elem: m.joinTaint(args), shape: m.anyShape(args)}
	if !t.tainted() {
		return
	}
	info := m.r.Fn(ent)
	params := info.Params
	if info.Recv != sem.RecvNone {
		params = append([]sem.EntityID{info.SelfParam}, params...)
	}
	for i, p := range params {
		if i < len(args) && m.writableParam(info, p) {
			m.mergeLabel(derefRaw(args[i]), t)
		}
	}
}

// writableParam is a `mut self` or `&mut` parameter, or a handle to contents the runtime keeps outside any layout
// (UnsafeCell, Shared), which a primitive may write through a shared receiver.
func (m *Machine) writableParam(info *sem.FnInfo, p sem.EntityID) bool {
	if p == info.SelfParam && info.Recv == sem.RecvMut {
		return true
	}
	t := m.r.Entity(p).Type
	if m.r.Types.RefMut(t) {
		return true
	}
	if m.r.Types.Kind(t) == sem.KRef {
		t = m.r.Types.Node(t).Elem
	}
	if m.r.Types.Kind(t) != sem.KNamed {
		return false
	}
	d := m.r.TypeDecl(m.r.Types.Node(t).Ent)
	return d != nil && d.Form == sem.FormOpaque && len(d.Params) > 0 && !m.r.HoldsElements(t)
}

// watchBranching marks the roleBranches callbacks of a primitive call until it returns, so callValueLent treats
// what they return as a branch condition; the returned func ends the watch.
func (m *Machine) watchBranching(roles []primArg, args []*obj) func() {
	n := len(m.branching)
	for _, a := range roles {
		if a.role == roleBranches && a.at < len(args) {
			m.branching = append(m.branching, deref(args[a.at]))
		}
	}
	return func() { m.branching = m.branching[:n] }
}

// checkCallbackBranch treats r as a branch condition when fv is a callback whose result its primitive branches on:
// a branch sink, and a decision in the two-run trace.
func (m *Machine) checkCallbackBranch(fv, r *obj) {
	for i := len(m.branching) - 1; i >= 0; i-- {
		if m.branching[i] == deref(fv) {
			m.checkBranch(r)
			m.trace.decision(r)
			return
		}
	}
}
