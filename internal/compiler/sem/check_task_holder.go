package sem

import (
	"slices"

	"kigumi/internal/compiler/syntax"
)

// taskHolding is how a value of a type can hold a `Task` handle: hit for one in its own structure, ifaces for each
// interface it may hold, which hides one only when a type converted to it does.
type taskHolding struct {
	hit    bool
	ifaces []TypeID
}

func (h taskHolding) any() bool { return h.hit || len(h.ifaces) > 0 }

func (r *Result) taskHoldingOf(t TypeID) taskHolding {
	if h, ok := r.taskHoldings[t]; ok {
		return h
	}
	var h taskHolding
	r.walkTaskHolding(t, map[TypeID]bool{}, &h)
	h.ifaces = slices.Clip(h.ifaces)
	if r.taskHoldings == nil {
		r.taskHoldings = map[TypeID]taskHolding{}
	}
	r.taskHoldings[t] = h
	return h
}

// walkTaskHolding leaves closures out: what a closure captures is checked where it is built.
func (r *Result) walkTaskHolding(t TypeID, seen map[TypeID]bool, h *taskHolding) {
	if h.hit || t == 0 || t == TyPoison || seen[t] {
		return
	}
	seen[t] = true
	n := r.Types.Node(t)
	switch n.Kind {
	case KFn, KClosure:
		return
	case KIface:
		h.ifaces = append(h.ifaces, t)
	case KNamed:
		if r.Entities[n.Ent].Kind == EntType && r.isStdTaskEnt(n.Ent, "Task") {
			h.hit = true
			return
		}
		if r.Entities[n.Ent].Kind == EntType {
			r.walkNamedTaskHolding(n.Ent, n.Args, seen, h)
		}
	}
	r.walkTaskHolding(n.Elem, seen, h)
	for _, a := range n.Args {
		r.walkTaskHolding(a, seen, h)
	}
}

func (r *Result) walkNamedTaskHolding(ent EntityID, args []TypeID, seen map[TypeID]bool, h *taskHolding) {
	decl := r.typeDecl(ent)
	subst := instSubst(decl.Params, args)
	switch decl.Form {
	case FormDefined:
		r.walkTaskHolding(r.Types.Subst(decl.Underlying, subst), seen, h)
	case FormRecord, FormResource:
		for _, fld := range decl.Fields {
			r.walkTaskHolding(r.Types.Subst(r.field(fld).Type, subst), seen, h)
		}
	case FormAdt:
		for _, v := range decl.Variants {
			for _, pt := range r.variant(v).Payload {
				r.walkTaskHolding(r.Types.Subst(pt, subst), seen, h)
			}
		}
	}
}

// TypeHoldsTask reports whether a value of t can hold a `Task` handle; through an interface it needs a type converted to
// that interface to hold one, which is only known once every body is checked.
func (r *Result) TypeHoldsTask(t TypeID) bool {
	if v, ok := r.taskHolds[t]; ok {
		return v
	}
	h := r.taskHoldingOf(t)
	_, _, hidden := r.hiddenIn(h.ifaces, r.implHoldsTask)
	v := h.hit || hidden
	if r.seal.witnesses != nil {
		if r.taskHolds == nil {
			r.taskHolds = map[TypeID]bool{}
		}
		r.taskHolds[t] = v
	}
	return v
}

// implHoldsTask counts a binding that left a type parameter open as one that may hold a handle.
func (r *Result) implHoldsTask(impl taskImpl) (bool, []TypeID) {
	h := r.taskHoldingOf(impl.t)
	return h.hit || impl.open, h.ifaces
}

// hiddenIn finds an interface of ifaces and a type converted to it for which hit holds, through the interfaces those
// types may hold in turn.
func (r *Result) hiddenIn(ifaces []TypeID, hit func(taskImpl) (bool, []TypeID)) (TypeID, taskImpl, bool) {
	seen := map[EntityID]bool{}
	for len(ifaces) > 0 {
		iface := ifaces[0]
		ifaces = ifaces[1:]
		ent := r.Types.Node(iface).Ent
		if seen[ent] {
			continue
		}
		seen[ent] = true
		for _, impl := range r.ifaceImplementers(ent) {
			ok, more := hit(impl)
			if ok {
				return iface, impl, true
			}
			ifaces = append(ifaces, more...)
		}
	}
	return 0, taskImpl{}, false
}

type hiddenTaskSite struct {
	f      FileID
	n      syntax.NodeID
	code   Code
	args   []any
	ifaces []TypeID
	// futures, when set, are the interfaces through which alone the call's result may hold a Future.
	futures []TypeID
}

// rejectTaskHolder reports code at n when a value of t holds a `Task` handle in its own structure, and leaves one only an
// interface may hide to reportHiddenTasks; it says whether it reported now.
func (r *Result) rejectTaskHolder(f FileID, n syntax.NodeID, t TypeID, code Code, args ...any) bool {
	h := r.taskHoldingOf(t)
	switch {
	case h.hit:
		r.errAt(f, n, code, args...)
		return true
	case len(h.ifaces) > 0:
		r.deferHiddenTask(f, n, h.ifaces, code, args...)
	}
	return false
}

func (r *Result) deferHiddenTask(f FileID, n syntax.NodeID, ifaces []TypeID, code Code, args ...any) {
	r.taskHidden = append(r.taskHidden, hiddenTaskSite{f: f, n: n, code: code, args: args, ifaces: ifaces})
}

// reportHiddenTasks names the converted type that hides the handle in a note at the conversion.
func (r *Result) reportHiddenTasks() {
	for _, s := range r.taskHidden {
		if len(s.futures) > 0 {
			if _, _, ok := r.hiddenIn(s.futures, r.implHoldsFuture); !ok {
				continue
			}
		}
		at, node, note := s.f, s.n, ""
		if len(s.ifaces) > 0 {
			iface, impl, ok := r.hiddenIn(s.ifaces, r.implHoldsTask)
			if !ok {
				continue
			}
			at, node, note = impl.conv.f, impl.conv.n, r.conversionNote(iface, impl)
		}
		d, ok := r.diagAt(s.f, s.n, s.code, s.args...)
		if !ok {
			continue
		}
		if note != "" {
			d = d.WithNote(r.loc(at, r.Files[at].span(node)), note)
		}
		r.emitDiag(s.f, d)
	}
	r.taskHidden = nil
}

func (r *Result) conversionNote(iface TypeID, impl taskImpl) string {
	to, from, t := "`"+r.TypeString(iface)+"`", "`"+r.TypeString(impl.conv.from)+"`", "`"+r.TypeString(impl.t)+"`"
	switch {
	case !r.taskHoldingOf(impl.t).hit:
		return from + " is converted to " + to + " here, and not every binding of its type parameters is known; one may be a `Task`"
	case impl.bound:
		return from + " is converted to " + to + " here; an instantiation makes it " + t + ", which holds a `Task` handle"
	}
	return t + " holds a `Task` handle and is converted to " + to + " here"
}

func (r *Result) implHoldsFuture(impl taskImpl) (bool, []TypeID) {
	return r.holdsFuture(impl.t, false) || impl.open, r.taskHoldingOf(impl.t).ifaces
}
