package sem

import (
	"slices"

	"kigumi/internal/syntax"
)

// A summary applied after the site was checked may still grow the field's known fns.
type lateFieldEdge struct {
	kind   EdgeKind
	src    fieldFnKey
	param  EntityID
	caller EntityID
	file   FileID
	node   syntax.NodeID
	scoped bool
}

func (c *checker) stageLateFieldEdge(kind EdgeKind, src fieldFnKey, param EntityID, n syntax.NodeID) {
	if c.fn == 0 {
		return
	}
	c.r.lateFieldEdges = append(c.r.lateFieldEdges, lateFieldEdge{kind: kind, src: src, param: param, caller: c.fn, file: c.f, node: n, scoped: c.allocDepth > 0})
}

func (r *Result) closeLateFieldEdges() {
	for _, late := range r.lateFieldEdges {
		info := r.Fn(late.caller)
		r.bindWrittenPlace(info, late)
	next:
		for _, tgt := range r.knownFieldFn[late.src] {
			r.noteEdgeSite(late.src, late.file, late.node, tgt)
			for _, e := range info.Edges {
				if e.Kind == late.kind && e.Target == tgt && e.Param == late.param && e.Node == late.node && e.File == late.file {
					continue next
				}
			}
			info.Edges = append(info.Edges, EffectEdge{Kind: late.kind, Target: tgt, Param: late.param, File: late.file, Node: late.node, Scoped: late.scoped})
		}
	}
}

func (r *Result) bindWrittenPlace(info *FnInfo, late lateFieldEdge) {
	ent, ok := r.fieldParamEnts[late.src]
	if late.kind != EdgeFieldBind || !ok || len(r.fieldBindOwners[ent]) == 0 {
		return
	}
	for _, e := range info.Edges {
		if e.Kind == EdgeFieldBind && e.Target == ent && e.Param == late.param && e.Node == late.node && e.File == late.file {
			return
		}
	}
	info.Edges = append(info.Edges, EffectEdge{Kind: EdgeFieldBind, Target: ent, Param: late.param, File: late.file, Node: late.node, Scoped: late.scoped})
}

type lateBindSite struct {
	caller     EntityID
	file       FileID
	node       syntax.NodeID
	callee     EntityID
	args       []syntax.NodeID
	seen       int
	seenNested int
	scoped     bool
}

func (c *checker) stageLateBindSite(n syntax.NodeID, callee EntityID, args []syntax.NodeID, seen int) {
	c.r.lateBindSites = append(c.r.lateBindSites, lateBindSite{caller: c.fn, file: c.f, node: n, callee: callee,
		args: slices.Clone(args), seen: seen, seenNested: len(c.r.nestedCalled[callee]), scoped: c.allocDepth > 0})
}

// Binding late keeps a call site's edges independent of declaration order; a nested key a callee gains by forwarding reaches its callers on a later pass.
func (r *Result) bindLateFieldCalls() {
	for changed := true; changed; {
		changed = false
		for i := range r.lateBindSites {
			if r.bindLateSite(&r.lateBindSites[i]) {
				changed = true
			}
		}
	}
}

func (r *Result) bindLateSite(site *lateBindSite) bool {
	keys := r.Fn(site.callee).CalledFieldParams
	nested := r.nestedCalled[site.callee]
	if site.seen >= len(keys) && site.seenNested >= len(nested) {
		return false
	}
	var edges []EffectEdge
	c := r.newChecker(site.file, site.caller, 0, 0, &edges)
	if site.scoped {
		c.allocDepth = 1
	}
	params := r.Fn(site.callee).Params
	for _, key := range keys[min(site.seen, len(keys)):] {
		i := slices.Index(params, key.Param)
		if i < 0 || i >= len(site.args) || c.t.Kind(site.args[i]) == syntax.Spread {
			continue
		}
		c.knownFieldArgEdge(site.node, site.args[i], fieldFnKey{Ent: key.Param, Field: key.Field})
	}
	c.nestedArgEdges(site.node, site.callee, site.args, site.seenNested)
	site.seen, site.seenNested = len(keys), len(nested)
	info := r.Fn(site.caller)
	info.Edges = append(info.Edges, edges...)
	return true
}
