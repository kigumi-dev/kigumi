package sem

import "kigumi/internal/compiler/syntax"

// fnFlow is what reaches one type parameter, at one call or over the direct calls through one value: the function
// items passed for it, unless some argument for it was anything else (other).
type fnFlow struct {
	items []EntityID
	other bool
}

func (f *fnFlow) itemsOnly() []EntityID {
	if f == nil || f.other {
		return nil
	}
	return f.items
}

func flowAt(flows map[EntityID]*fnFlow, p EntityID) *fnFlow {
	if flows[p] == nil {
		flows[p] = &fnFlow{}
	}
	return flows[p]
}

// noteFlows adds what argument a, passed for a parameter of type decl, sends into each type parameter decl names: a
// bare parameter takes the function items a is, any other position a value of unknown origin.
func (c *checker) noteFlows(flows map[EntityID]*fnFlow, decl TypeID, a syntax.NodeID) {
	tt := c.r.Types
	if tt.Kind(decl) == KParam {
		f := flowAt(flows, tt.Node(decl).Ent)
		if items := c.argFnItems(a); len(items) > 0 {
			f.items = append(f.items, items...)
		} else {
			f.other = true
		}
		return
	}
	for _, p := range tt.paramsIn(decl, nil) {
		flowAt(flows, p).other = true
	}
}

func (tt *TypeTable) paramsIn(t TypeID, out []EntityID) []EntityID {
	if t == 0 {
		return out
	}
	n := tt.Node(t)
	if n.Kind == KParam {
		return append(out, n.Ent)
	}
	out = tt.paramsIn(n.Elem, out)
	for _, a := range n.Args {
		out = tt.paramsIn(a, out)
	}
	return out
}

// noteRecvFlows marks every type parameter fn's receiver type names as reached by a value of unknown origin: the
// receiver was built elsewhere.
func (c *checker) noteRecvFlows(flows map[EntityID]*fnFlow, fn EntityID) {
	if info := c.r.Fn(fn); info.Recv != RecvNone && info.SelfParam != 0 {
		for _, p := range c.r.Types.paramsIn(c.r.Entities[info.SelfParam].Type, nil) {
			flowAt(flows, p).other = true
		}
	}
}

func (c *checker) noteCallFlows(n syntax.NodeID, fn EntityID, declared []TypeID, i int, a syntax.NodeID) {
	if i >= len(declared) {
		return
	}
	if c.callFlows == nil {
		c.callFlows = map[syntax.NodeID]map[EntityID]*fnFlow{}
	}
	if c.callFlows[n] == nil {
		c.callFlows[n] = map[EntityID]*fnFlow{}
		c.noteRecvFlows(c.callFlows[n], fn)
	}
	c.noteFlows(c.callFlows[n], declared[i], a)
}
