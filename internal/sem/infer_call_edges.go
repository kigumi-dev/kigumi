package sem

import (
	"slices"

	"kigumi/internal/syntax"
)

// Each fn-typed arg contributes its own effects; a named edge only counts
// once the callee body is known to call that parameter.
func (c *checker) callbackEdges(n syntax.NodeID, fn EntityID, sig TypeID, args []syntax.NodeID) {
	tt := c.r.Types
	params := tt.Node(sig).Args
	info := c.r.Fn(fn)
	offloaded := c.r.isOffloadingCallback(fn)
	for i, a := range args {
		if i >= len(params) {
			continue
		}
		pt := c.vars.resolve(params[i])
		if tt.Kind(pt) == KIface {
			c.ifaceArgEdge(n, a, info, i)
			continue
		}
		if tt.Kind(pt) != KClosure && c.r.fnShapeOf(pt) == 0 {
			continue
		}
		var param EntityID
		if info.Body != 0 && i < len(info.Params) {
			param = info.Params[i]
		}
		at := c.vars.resolve(c.info.Types[a])
		switch {
		case c.info.Uses[a] != 0 && c.r.Entities[c.info.Uses[a]].Kind == EntFn:
			tgt := c.info.Uses[a]
			c.edge(EffectEdge{Kind: EdgeCallback, Target: tgt, Param: param, Node: a, Offloaded: offloaded})
			c.fnBoundArgEdge(n, tgt, param)
		case tt.Kind(at) == KClosure:
			tgt := tt.Node(at).Ent
			c.edge(EffectEdge{Kind: EdgeCallback, Target: tgt, Param: param, Node: a, Offloaded: offloaded})
			c.fnBoundArgEdge(n, tgt, param)
		default:
			if shape := c.r.fnShapeOf(at); shape != 0 {
				c.callbackEdge(n, a, Effects(tt.Node(shape).Flags), param, offloaded)
			}
		}
	}
}

// fnBoundArgEdge is ifaceArgEdge's counterpart for a Fn-bound parameter,
// keyed on the call node n rather than the argument's own node like EdgeCallback.
func (c *checker) fnBoundArgEdge(n syntax.NodeID, tgt, param EntityID) {
	if param == 0 {
		return
	}
	c.edge(EffectEdge{Kind: EdgeFnBoundArg, Target: tgt, Param: param, Node: n})
}

// ifaceArgEdge records which parameter an interface-typed argument binds,
// keyed at the call node n to match it back to that call's own EdgeCall.
func (c *checker) ifaceArgEdge(n, a syntax.NodeID, info *FnInfo, i int) {
	if info.Body == 0 || i >= len(info.Params) {
		return
	}
	c.edge(EffectEdge{Kind: EdgeIfaceArg, Target: c.info.Uses[a], Param: info.Params[i], Node: n})
}

// When the value is a parameter of the enclosing function, that function
// becomes polymorphic in it instead of being blamed for its effects.
func (c *checker) callbackEdge(n, expr syntax.NodeID, mods Effects, param EntityID, offloaded bool) {
	if p := c.polyParam(expr); p != 0 {
		c.deferToParam(p)
		// Param, when set, is the slot p was forwarded into; root-bridge
		// reachability needs that to tell forwarding from invoking.
		c.edge(EffectEdge{Kind: EdgeParamCall, Target: p, Param: param, Node: expr})
		c.fnBoundArgEdge(n, p, param)
		return
	}
	c.edge(EffectEdge{Kind: EdgeCallback, Mods: mods, Param: param, Node: expr, Offloaded: offloaded})
}

// A non-fn-typed p (from a record-typed parameter's field fact) has
// no matching argument slot in callbackEdges, so it's skipped here.
func (c *checker) deferToParam(p EntityID) {
	if c.r.fnShapeOf(c.r.Entities[p].Type) == 0 {
		return
	}
	info := c.r.Fn(c.r.Entities[p].Parent)
	if !slices.Contains(info.CalledParams, p) {
		info.CalledParams = append(info.CalledParams, p)
	}
}

// polyParam excludes closure parameters, which keep the declared-type rule.
func (c *checker) polyParam(expr syntax.NodeID) EntityID {
	for c.t.Kind(expr) == syntax.CallExpr || c.t.Kind(expr) == syntax.WsCallExpr {
		expr = syntax.NodeID(c.t.Nodes[expr].Lhs)
	}
	p := c.info.Uses[expr]
	if alias, ok := c.paramAlias[p]; ok {
		p = alias
	}
	if p == 0 || c.r.Entities[p].Kind != EntParam || c.r.Entities[c.r.Entities[p].Parent].Kind != EntFn {
		return 0
	}
	return p
}

func (c *checker) abandonCall(sig TypeID, args []syntax.NodeID) TypeID {
	c.synthArgs(args)
	c.vars.poison(sig)
	return TyPoison
}

func (c *checker) variadicForm(args []syntax.NodeID, sig TypeID) VariadicForm {
	if c.r.Types.Node(sig).Flags&fnVariadic == 0 {
		return VariadicNone
	}
	if len(args) > 0 && c.t.Kind(args[len(args)-1]) == syntax.Spread {
		return VariadicSpread
	}
	return VariadicElements
}
