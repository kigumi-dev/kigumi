package sem

import (
	"fmt"
	"slices"

	"kigumi/internal/syntax"
)

func (r *Result) checkContextRequirements() {
	nodes := r.ctxNodes()
	// sets[id][key] is the nearest node inside id that pulls key in, for
	// an unbound-at-root diagnostic to point at.
	sets := map[EntityID]map[EntityID]ctxNodeRef{}
	// real[id][key] is false for a key reaching id only through print's
	// phantom dependency (infer_context.go), never a real CtxUse.
	real := map[EntityID]map[EntityID]bool{}
	for _, id := range nodes {
		info := r.Fn(id)
		refOf := map[EntityID]ctxNodeRef{}
		for _, ref := range info.CtxDirectRefs {
			key := r.Files[ref.File].CtxUse[ref.Node].Key
			if _, has := refOf[key]; !has {
				refOf[key] = ref
			}
		}
		set, rset := map[EntityID]ctxNodeRef{}, map[EntityID]bool{}
		for _, k := range info.CtxDirect {
			set[k] = refOf[k]
			rset[k] = info.CtxDirectReal[k]
		}
		sets[id], real[id] = set, rset
	}
	for changed := true; changed; {
		changed = false
		for _, id := range nodes {
			set, rset := sets[id], real[id]
			for _, e := range r.Fn(id).ctxCalls {
				if len(e.lambdas) > 0 {
					continue
				}
				for k := range sets[e.callee] {
					if _, _, found := ctxStackFind(e.stack, k); found {
						continue
					}
					if _, has := set[k]; !has {
						set[k] = e.ref
						changed = true
					}
					if real[e.callee][k] && !rset[k] {
						rset[k] = true
						changed = true
					}
				}
			}
		}
	}
	for _, id := range nodes {
		keys := make([]EntityID, 0, len(sets[id]))
		for k := range sets[id] {
			keys = append(keys, k)
		}
		slices.Sort(keys)
		info := r.Fn(id)
		info.CtxRequires, info.CtxReal = keys, real[id]
	}
	for _, id := range nodes {
		r.resolveCtxFn(id)
	}
	r.settleCtxExclusive()
	r.checkCtxRoots(nodes, sets, real)
	r.checkCtxVisibility(nodes)
	r.checkCtxValueRefs()
}

// checkCtxVisibility reports a public function requiring a narrower-
// visibility key: outside callers could never bind it.
func (r *Result) checkCtxVisibility(nodes []EntityID) {
	for _, id := range nodes {
		e := &r.Entities[id]
		if e.Kind != EntFn || e.Vis.Level == VisPrivate {
			continue
		}
		for _, key := range r.Fn(id).CtxRequires {
			kv := r.Entities[key].Vis
			if !r.covers(kv, e.Vis) {
				r.errAt(e.File, e.Node, cCtxVisibilityLeak, e.Name, r.visText(kv), key)
			}
		}
	}
}

func (r *Result) ctxNodes() []EntityID {
	var out []EntityID
	for id := 1; id < len(r.Entities); id++ {
		e := &r.Entities[id]
		if e.Flags&EfPoison != 0 {
			continue
		}
		switch e.Kind {
		case EntFn:
			if r.Fn(EntityID(id)).Body != 0 && r.Entities[e.Parent].Kind != EntInterface {
				out = append(out, EntityID(id))
			}
		case EntImplicitMain, EntTest:
			out = append(out, EntityID(id))
		}
	}
	return out
}

// ctxParamFor returns fn's hidden parameter entity that reads key.
func (r *Result) ctxParamFor(fn, key EntityID) EntityID {
	info := r.Fn(fn)
	if info.CtxParam == nil {
		info.CtxParam = map[EntityID]EntityID{}
	}
	if id, ok := info.CtxParam[key]; ok {
		return id
	}
	e := &r.Entities[fn]
	pe := Entity{Kind: EntParam, Name: fmt.Sprintf("$ctx.%s", r.Entities[key].Name), Pkg: e.Pkg, File: e.File, Node: e.Node,
		Parent: fn, Type: r.Entities[key].Type, Flags: EfMut}
	id := r.newEntity(pe)
	r.Entities[id].Detail = r.addLocal(LocalInfo{Scope: r.declScope(fn)})
	info.CtxParam[key] = id
	return id
}

func (r *Result) resolveCtxFn(fn EntityID) {
	info := r.Fn(fn)
	for _, key := range info.CtxRequires {
		if info.CtxReal[key] {
			r.ctxParamFor(fn, key)
		}
	}
	for _, ref := range info.CtxDirectRefs {
		if !info.CtxReal[r.Files[ref.File].CtxUse[ref.Node].Key] {
			continue
		}
		fi := &r.Files[ref.File]
		b := fi.CtxUse[ref.Node]
		b.Param = r.ctxParamFor(fn, b.Key)
		fi.CtxUse[ref.Node] = b
	}
	for _, e := range info.ctxCalls {
		calleeInfo := r.Fn(e.callee)
		var binds []CtxBind
		for _, key := range calleeInfo.CtxRequires {
			if !calleeInfo.CtxReal[key] {
				continue
			}
			if src, depth, ok := ctxStackFind(e.stack, key); ok {
				r.ctxCaptureChain(e.lambdas[depth:], src.Local)
				binds = append(binds, CtxBind{Key: key, Local: src.Local, Field: src.Field})
				continue
			}
			if len(e.lambdas) > 0 {
				r.errAt(e.ref.File, e.ref.Node, cCtxUnboundKey, key)
				binds = append(binds, CtxBind{Key: key})
				continue
			}
			binds = append(binds, CtxBind{Key: key, Param: r.ctxParamFor(fn, key)})
		}
		if len(binds) == 0 {
			continue
		}
		fi := &r.Files[e.ref.File]
		if fi.CtxCall == nil {
			fi.CtxCall = map[syntax.NodeID][]CtxBind{}
		}
		fi.CtxCall[e.ref.Node] = binds
	}
}
