package sem

import "kigumi/internal/syntax"

type methodClosureKey struct {
	method, creator EntityID
	recv            TypeID
	scoped          bool
}

type methodValueState struct {
	shared     map[methodClosureKey][]EntityID
	deferrable map[uint32]string
	pending    []pendingInfer
	nodes      []syntax.NodeID
	closures   []EntityID
	depth      int
	truncates  []truncateValue
}

type pendingInfer struct {
	v     uint32
	depth int
}

func (c *checker) sharedMethodClosure(key methodClosureKey, sig TypeID, name string) EntityID {
	sig = c.vars.resolve(sig)
	if c.r.Types.ContainsVar(sig) {
		return 0
	}
	for _, id := range c.methodValues.shared[key] {
		info := c.r.closure(id)
		if c.vars.resolve(info.Sig) != sig {
			continue
		}
		if local := &c.r.Entities[info.Captures[0].Local]; local.Name != name {
			local.Name = "receiver"
		}
		return id
	}
	return 0
}

func (c *checker) receiverName(base syntax.NodeID) string {
	if b := c.t.Unparen(base); c.t.Kind(b) == syntax.Ident {
		return c.t.View(b).Tok().Text()
	}
	return "receiver"
}

func (c *checker) capturedName(n syntax.NodeID, info *ClosureInfo, local EntityID) string {
	if call, ok := c.info.Calls[n]; ok && call.Kind == CallMethodValue && info.IsMethodValue() {
		switch c.t.Kind(n) {
		case syntax.MemberExpr, syntax.OptMemberExpr:
			return c.receiverName(c.t.View(n).Base().ID())
		}
	}
	return c.r.Entities[local].Name
}

// Method values of one method whose type arguments are still open share one closure type so a later unification binds both.
func (c *checker) adoptMethodClosure(n syntax.NodeID, want, got TypeID) bool {
	tt := c.r.Types
	w, g := tt.Node(want), tt.Node(got)
	if w.Kind != KClosure || g.Kind != KClosure || w.Ent == g.Ent {
		return false
	}
	wi, gi := c.r.closure(w.Ent), c.r.closure(g.Ent)
	if !wi.IsMethodValue() || wi.Method != gi.Method || wi.Creator != gi.Creator || wi.Scoped != gi.Scoped {
		return false
	}
	wl, gl := &c.r.Entities[wi.Captures[0].Local], &c.r.Entities[gi.Captures[0].Local]
	if c.vars.resolve(wl.Type) != c.vars.resolve(gl.Type) {
		return false
	}
	path := c.methodValuePath(n, got)
	if path == nil {
		return false
	}
	mark := c.vars.mark()
	if !c.vars.unify(wi.Sig, gi.Sig) {
		c.vars.rollback(mark)
		return false
	}
	for _, x := range path {
		c.setType(x, want)
	}
	if wl.Name != gl.Name {
		wl.Name = "receiver"
	}
	return true
}

func (c *checker) methodValuePath(n syntax.NodeID, got TypeID) []syntax.NodeID {
	var path []syntax.NodeID
	for x := n; x != 0 && len(path) < 64; {
		if c.vars.resolve(c.info.Types[x]) != got {
			return nil
		}
		path = append(path, x)
		if call, ok := c.info.Calls[x]; ok && call.Kind == CallMethodValue {
			return path
		}
		switch c.t.Kind(x) {
		case syntax.Paren:
			x = c.t.View(x).AsParen().Inner().ID()
		case syntax.Block:
			tail := c.t.View(x).AsBlock().Tail()
			if !tail.Valid() {
				return nil
			}
			x = tail.ID()
		default:
			return nil
		}
	}
	return nil
}
