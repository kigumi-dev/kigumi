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
	next:
		for _, tgt := range r.knownFieldFn[late.src] {
			for _, e := range info.Edges {
				if e.Kind == late.kind && e.Target == tgt && e.Param == late.param && e.Node == late.node && e.File == late.file {
					continue next
				}
			}
			info.Edges = append(info.Edges, EffectEdge{Kind: late.kind, Target: tgt, Param: late.param, File: late.file, Node: late.node, Scoped: late.scoped})
		}
	}
}

type lateBindSite struct {
	caller EntityID
	file   FileID
	node   syntax.NodeID
	callee EntityID
	args   []syntax.NodeID
	seen   int
	scoped bool
}

func (c *checker) stageLateBindSite(n syntax.NodeID, callee EntityID, args []syntax.NodeID, seen int) {
	c.r.lateBindSites = append(c.r.lateBindSites, lateBindSite{caller: c.fn, file: c.f, node: n, callee: callee,
		args: slices.Clone(args), seen: seen, scoped: c.allocDepth > 0})
}

// Binding late keeps a call site's edges independent of declaration order.
func (r *Result) bindLateFieldCalls() {
	for _, site := range r.lateBindSites {
		keys := r.Fn(site.callee).CalledFieldParams
		if site.seen >= len(keys) {
			continue
		}
		var edges []EffectEdge
		c := r.newChecker(site.file, site.caller, 0, 0, &edges)
		if site.scoped {
			c.allocDepth = 1
		}
		params := r.Fn(site.callee).Params
		for _, key := range keys[site.seen:] {
			i := slices.Index(params, key.Param)
			if i < 0 || i >= len(site.args) || c.t.Kind(site.args[i]) == syntax.Spread {
				continue
			}
			c.knownFieldArgEdge(site.node, site.args[i], fieldFnKey{Ent: key.Param, Field: key.Field})
		}
		info := r.Fn(site.caller)
		info.Edges = append(info.Edges, edges...)
	}
}
