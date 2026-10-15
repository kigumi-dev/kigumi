package sem

import (
	"slices"

	"kigumi/internal/syntax"
)

type resultParam struct {
	index int
	path  []EntityID
}

func (c *checker) noteResultParam(src EntityID) {
	if len(c.lambdas) > 0 {
		return
	}
	root, path := c.r.nestedPath(src)
	e := &c.r.Entities[root]
	if e.Kind != EntParam || e.Parent != c.fn {
		return
	}
	i := slices.Index(c.r.Fn(c.fn).Params, root)
	if i < 0 {
		return
	}
	for _, rp := range c.r.flow.resultParams[c.fn] {
		if rp.index == i && slices.Equal(rp.path, path) {
			return
		}
	}
	if c.r.flow.resultParams == nil {
		c.r.flow.resultParams = map[EntityID][]resultParam{}
	}
	c.r.flow.resultParams[c.fn] = append(c.r.flow.resultParams[c.fn], resultParam{index: i, path: path})
}

func (r *Result) applyResultParams(site methodFieldCallSite) bool {
	info := r.Fn(site.callee)
	retType := r.Types.Node(info.Sig).Elem
	t := r.tree(site.file)
	changed := false
	for _, rp := range r.flow.resultParams[site.callee] {
		if rp.index >= len(site.args) || t.Kind(site.args[rp.index]) == syntax.Spread {
			continue
		}
		typ := retType
		if len(rp.path) > 0 {
			typ = r.Entities[rp.path[len(rp.path)-1]].Type
		}
		if r.forwardResultArg(site, rp, typ) {
			changed = true
		}
		if len(rp.path) > 0 || rp.index >= len(info.Params) {
			continue
		}
		for _, f := range r.recordFieldsOf(typ) {
			if r.applyFieldWriteSummary(site, f, site.callee, info.Params[rp.index]) {
				changed = true
			}
		}
	}
	return changed
}

func (r *Result) forwardResultArg(site methodFieldCallSite, rp resultParam, typ TypeID) bool {
	t := r.tree(site.file)
	expr := stripParens(t, site.args[rp.index])
	for _, f := range rp.path {
		if t.Kind(expr) != syntax.RecordLit {
			break
		}
		expr = stripParens(t, r.literalInit(site.file, expr, f))
		rp.path = rp.path[1:]
	}
	if expr == 0 {
		return false
	}
	if t.Kind(expr) == syntax.RecordLit {
		return len(rp.path) == 0 && r.writeLiteralFields(site, site.recv, expr, typ, 0)
	}
	src := r.fnChainEnt(site.file, expr)
	if src == 0 || !r.concretePlace(src) {
		return false
	}
	for _, f := range rp.path {
		src = r.nestedFnOwner(fieldFnKey{Ent: src, Field: f})
	}
	return r.addFnUnion(fnUnion{dst: site.recv, src: src, typ: typ, file: site.file, node: site.node})
}

func (r *Result) writeLiteralFields(site methodFieldCallSite, owner EntityID, lit syntax.NodeID, typ TypeID, depth int) bool {
	if depth >= maxNestedFnDepth {
		return false
	}
	t := r.tree(site.file)
	changed := false
	for _, entry := range t.Children(syntax.NodeID(t.Nodes[lit].Rhs)) {
		field := r.Files[site.file].Uses[entry]
		if t.Kind(entry) == syntax.Spread || field == 0 || !r.holdsFn(r.Entities[field].Type, 0) {
			continue
		}
		init := stripParens(t, syntax.NodeID(t.Nodes[entry].Lhs))
		ft := r.Entities[field].Type
		if r.fnTypedType(ft) || r.Types.Kind(ft) == KIface {
			if r.applyFieldWriteExpr(site.file, fieldFnKey{Ent: owner, Field: field}, init, site.caller, site.node) {
				changed = true
			}
			continue
		}
		nested := r.nestedFnOwner(fieldFnKey{Ent: owner, Field: field})
		if t.Kind(init) == syntax.RecordLit {
			if r.writeLiteralFields(site, nested, init, ft, depth+1) {
				changed = true
			}
		} else if src := r.fnChainEnt(site.file, init); src != 0 && r.concretePlace(src) {
			if r.addFnUnion(fnUnion{dst: nested, src: src, typ: ft, file: site.file, node: site.node}) {
				changed = true
			}
		}
	}
	return changed
}

func (r *Result) literalInit(file FileID, lit syntax.NodeID, field EntityID) syntax.NodeID {
	t := r.tree(file)
	for _, entry := range t.Children(syntax.NodeID(t.Nodes[lit].Rhs)) {
		if t.Kind(entry) != syntax.Spread && r.Files[file].Uses[entry] == field {
			return syntax.NodeID(t.Nodes[entry].Lhs)
		}
	}
	return 0
}

func stripParens(t *syntax.Tree, n syntax.NodeID) syntax.NodeID {
	for t.Kind(n) == syntax.Paren {
		n = syntax.NodeID(t.Nodes[n].Lhs)
	}
	return n
}

// The passes that apply result sites repeat, so a union is queued once per copy site.
func (r *Result) addFnUnion(u fnUnion) bool {
	k := u
	k.typ = 0
	if r.flow.unionSeen[k] {
		return false
	}
	if r.flow.unionSeen == nil {
		r.flow.unionSeen = map[fnUnion]bool{}
	}
	r.flow.unionSeen[k] = true
	r.fnUnions = append(r.fnUnions, u)
	return true
}
