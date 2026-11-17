package sem

import "kigumi/internal/compiler/syntax"

func (w *leaseWalk) taintCallback(n syntax.NodeID) {
	t := w.c.t
	if typ := w.c.vars.resolve(w.c.info.Types[n]); w.c.r.Types.Kind(typ) == KClosure {
		ent := w.c.r.Types.Node(typ).Ent
		info := w.c.r.closure(ent)
		if info.IsMethodValue() || w.c.r.Entities[ent].File != w.c.f {
			if name, ok := w.retainingCallee(n); ok {
				w.c.errAt(n, cLeasePassed, w.fn, name)
			}
			return
		}
		changed := false
		for _, param := range info.Params {
			typ := w.c.r.Entities[param].Type
			if !w.tainted[param] && w.c.r.mayContainPtr(typ, map[TypeID]bool{}) {
				w.tainted[param], changed = true, true
			}
		}
		if changed && t.Kind(n) != syntax.Lambda {
			w.walk(t.View(w.c.r.Entities[ent].Node).AsLambda().Body().ID())
		}
		return
	}
	if w.callable(n) {
		if name, ok := w.retainingCallee(n); ok {
			w.c.errAt(n, cLeasePassed, w.fn, name)
		}
		return
	}
	for child := range t.View(n).Children() {
		w.taintCallback(child.ID())
	}
}

func (w *leaseWalk) callable(n syntax.NodeID) bool {
	t := w.c.vars.resolve(w.c.info.Types[n])
	k := w.c.r.Types.Kind(t)
	return k == KFn || k == KClosure
}
