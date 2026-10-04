package sem

import (
	"fmt"
	"slices"

	"kigumi/internal/syntax"
)

// synthContext's bindings shadow any outer ones for the block's extent.
func (c *checker) synthContext(n syntax.NodeID, want TypeID) TypeID {
	node := c.t.Nodes[n]
	items := c.t.Children(syntax.NodeID(node.Lhs))
	frame := ctxFrame{bind: map[EntityID]ctxBindSource{}, depth: len(c.lambdas)}
	locals := make([]EntityID, len(items))
	for i, item := range items {
		pt := c.vars.resolve(c.synth(item))
		if pt == TyPoison {
			continue
		}
		bindings, ok := c.r.providesOf(pt)
		if !ok {
			c.errAt(item, cCtxWrongProviderType, pt)
			continue
		}
		local := c.declareLocal(item, fmt.Sprintf("$ctx%d", i), pt, true)
		locals[i] = local
		for _, b := range bindings {
			if _, dup := frame.bind[b.Key]; dup {
				c.errAt(item, cCtxDuplicateBinding, b.Key)
				continue
			}
			frame.bind[b.Key] = ctxBindSource{Local: local, Field: b.Field}
		}
	}
	if c.info.CtxProviders == nil {
		c.info.CtxProviders = map[syntax.NodeID][]EntityID{}
	}
	c.info.CtxProviders[n] = locals
	c.ctxStack = append(c.ctxStack, frame)
	t := c.check(syntax.NodeID(node.Rhs), want)
	c.ctxStack = c.ctxStack[:len(c.ctxStack)-1]
	return t
}

func (c *checker) ctxLookup(key EntityID) (ctxBindSource, bool) {
	for i := len(c.ctxStack) - 1; i >= 0; i-- {
		if s, ok := c.ctxStack[i].bind[key]; ok {
			return s, true
		}
	}
	return ctxBindSource{}, false
}

func (c *checker) contextKeyValue(n syntax.NodeID, key EntityID) TypeID {
	typ := c.r.Entities[key].Type
	if src, ok := c.ctxLookup(key); ok {
		c.noteCapture(n, src.Local, c.r.local(src.Local).Scope)
		c.setCtxUse(n, CtxBind{Key: key, Local: src.Local, Field: src.Field})
		c.placeOf(n)
		return typ
	}
	if len(c.lambdas) > 0 {
		c.errAt(n, cCtxUnboundKey, key)
		c.setCtxUse(n, CtxBind{Key: key})
		return typ
	}
	info := c.r.Fn(c.fn)
	c.addCtxDirect(info, key, n)
	if info.CtxDirectReal == nil {
		info.CtxDirectReal = map[EntityID]bool{}
	}
	info.CtxDirectReal[key] = true
	// Minted now, not deferred to the post-fixpoint Params append, so a
	// direct use can become a real mutable place immediately (buildPlace).
	c.setCtxUse(n, CtxBind{Key: key, Param: c.r.ctxParamFor(c.fn, key)})
	c.placeOf(n)
	return typ
}

func (c *checker) addCtxDirect(info *FnInfo, key EntityID, n syntax.NodeID) {
	if !slices.Contains(info.CtxDirect, key) {
		info.CtxDirect = append(info.CtxDirect, key)
	}
	info.CtxDirectRefs = append(info.CtxDirectRefs, ctxNodeRef{File: c.f, Node: n})
}

// ctxPlace mirrors buildPlace (place.go) for a resolved context-key use.
func (c *checker) ctxPlace(n syntax.NodeID) (Place, bool) {
	b, ok := c.info.CtxUse[n]
	if !ok {
		return Place{}, false
	}
	if b.Local != 0 {
		return c.r.fieldPlace(c.r.identPlace(b.Local), b.Field), true
	}
	if b.Param != 0 {
		return c.r.identPlace(b.Param), true
	}
	return Place{}, false
}

func (c *checker) setCtxUse(n syntax.NodeID, b CtxBind) {
	if c.info.CtxUse == nil {
		c.info.CtxUse = map[syntax.NodeID]CtxBind{}
	}
	c.info.CtxUse[n] = b
}

// recordCtxCallEdge defers a call's context requirement to the fixpoint
// pass: the callee's own CtxRequires may not be final yet.
func (c *checker) recordCtxCallEdge(callee EntityID, node syntax.NodeID) {
	if callee == 0 || c.r.poisoned(callee) || c.r.Entities[callee].Kind != EntFn {
		return
	}
	info := c.r.Fn(c.fn)
	info.ctxCalls = append(info.ctxCalls, ctxCallEdge{
		ref:     ctxNodeRef{File: c.f, Node: node},
		callee:  callee,
		stack:   slices.Clone(c.ctxStack),
		lambdas: lambdaEnts(c.lambdas),
	})
}

// lambdaEnts lists the entities of every open lambda, outermost first.
func lambdaEnts(l []*lambdaCtx) []EntityID {
	if len(l) == 0 {
		return nil
	}
	out := make([]EntityID, len(l))
	for i, ctx := range l {
		out[i] = ctx.ent
	}
	return out
}

// recordCtxValueRef defers a callable-as-value reference's validity to the
// fixpoint pass; every such erasure site must call this, not just bare fn names.
func (c *checker) recordCtxValueRef(n syntax.NodeID, fn EntityID) {
	if fn == 0 || c.r.Fn(fn).Body == 0 {
		return
	}
	c.r.ctxValueRefs = append(c.r.ctxValueRefs, ctxValueRef{ref: ctxNodeRef{File: c.f, Node: n}, target: fn})
}

// printCtxRequirement joins print's std/stdio dependency into CtxDirect as a
// phantom: it never reads the value, so an unbound key never errors.
func (c *checker) printCtxRequirement(n syntax.NodeID, name string) {
	stdioName := "stdout"
	if name == "eprint" {
		stdioName = "stderr"
	}
	key := c.r.stdCtxKey("std/stdio", stdioName)
	if key == 0 {
		return
	}
	if _, ok := c.ctxLookup(key); ok {
		return
	}
	c.addCtxDirect(c.r.Fn(c.fn), key, n)
	c.setCtxUse(n, CtxBind{Key: key})
}

// stdCtxKey returns 0 silently when the key isn't loaded or isn't a context key.
func (r *Result) stdCtxKey(pkgPath, name string) EntityID {
	pkg, ok := r.pathIndex[pkgPath]
	if !ok {
		return 0
	}
	b, ok := r.Scopes[r.Packages[pkg].Scope].Names[name]
	if !ok || b.Ent == 0 || r.Entities[b.Ent].Kind != EntContextKey {
		return 0
	}
	return b.Ent
}
