package sem

import (
	"slices"

	"kigumi/internal/compiler/syntax"
)

// selfIndex is the resultParam index of the receiver, which is not in Params.
const selfIndex = -2

type resultParam struct {
	index int
	path  []EntityID
	// into is the field of the result the param place is stored in; empty when it is the whole result.
	into []EntityID
	// overwritten lists the param's fields every return of it has rewritten, so the caller's own binding does not reach the result.
	overwritten []EntityID
}

func (c *checker) noteResultParam(src EntityID, leaf syntax.NodeID) {
	c.noteResultParamAt(nil, src, leaf)
}

// noteResultParamAt reports whether src is a place of a param of this fn, now recorded as the value of the result's field into.
func (c *checker) noteResultParamAt(into []EntityID, src EntityID, leaf syntax.NodeID) bool {
	if len(c.lambdas) > 0 {
		return false
	}
	root, path := c.r.nestedPath(src)
	e := &c.r.Entities[root]
	if e.Kind != EntParam || e.Parent != c.fn {
		return false
	}
	i := slices.Index(c.r.Fn(c.fn).Params, root)
	if i < 0 {
		if sp := c.r.Fn(c.fn).SelfParam; sp == 0 || root != sp {
			return false
		}
		i = selfIndex
	}
	var over []EntityID
	if len(path) == 0 && len(into) == 0 && c.info.Uses[c.stripParen(leaf)] == root {
		over = c.overwrittenBefore(root, leaf)
	}
	for k, rp := range c.r.flow.resultParams[c.fn] {
		if rp.index == i && slices.Equal(rp.path, path) && slices.Equal(rp.into, into) {
			c.r.flow.resultParams[c.fn][k].overwritten = slices.DeleteFunc(slices.Clone(rp.overwritten), func(f EntityID) bool { return !slices.Contains(over, f) })
			return true
		}
	}
	if c.r.flow.resultParams == nil {
		c.r.flow.resultParams = map[EntityID][]resultParam{}
	}
	c.r.flow.resultParams[c.fn] = append(c.r.flow.resultParams[c.fn], resultParam{index: i, path: path, into: slices.Clone(into), overwritten: over})
	return true
}

func (r *Result) applyResultParams(site methodFieldCallSite) bool {
	info := r.Fn(site.callee)
	retType := site.retType
	t := r.tree(site.file)
	changed := false
	for _, rp := range r.flow.resultParams[site.callee] {
		arg := site.argAt(rp.index)
		if arg == 0 || t.Kind(arg) == syntax.Spread {
			continue
		}
		typ := retType
		if len(rp.path) > 0 {
			typ = r.Entities[rp.path[len(rp.path)-1]].Type
			if len(rp.into) == 0 && r.holdsFnPlace(retType) {
				typ = retType
			}
		} else if len(rp.into) > 0 {
			typ = r.Entities[rp.into[len(rp.into)-1]].Type
			if et, ok := r.componentPathType(retType, rp.into); ok {
				typ = et
			}
		}
		if r.forwardResultArg(site, rp, typ) {
			changed = true
		}
		if len(rp.path) > 0 || len(rp.into) > 0 || rp.index < 0 || rp.index >= len(info.Params) {
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
	dst := site.recv
	for _, f := range rp.into {
		dst = r.nestedFnOwner(fieldFnKey{Ent: dst, Field: f})
	}
	expr := stripParens(t, site.argAt(rp.index))
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
		return len(rp.path) == 0 && r.writeLiteralFields(site, dst, expr, typ, 0, rp.overwritten)
	}
	src := r.fnChainEnt(site.file, expr)
	if src == 0 || !r.concretePlace(src) {
		return false
	}
	for _, f := range rp.path {
		src = r.nestedFnOwner(fieldFnKey{Ent: src, Field: f})
	}
	return r.addFnUnion(fnUnion{dst: dst, src: src, typ: typ, file: site.file, node: site.node, skip: r.skipSetID(rp.overwritten)})
}

func (r *Result) writeLiteralFields(site methodFieldCallSite, owner EntityID, lit syntax.NodeID, typ TypeID, depth int, skip []EntityID) bool {
	if depth >= maxNestedFnDepth {
		return false
	}
	t := r.tree(site.file)
	changed := false
	for entry := range t.View(lit).AsRecordLit().Fields().IDs() {
		field := r.Files[site.file].Uses[entry]
		if t.Kind(entry) == syntax.Spread || field == 0 || !r.holdsFn(r.Entities[field].Type, 0) || slices.Contains(skip, field) {
			continue
		}
		init := stripParens(t, t.View(entry).AsFieldInit().Value().ID())
		ft := r.Entities[field].Type
		if r.fnTypedType(ft) || r.Types.Kind(ft) == KIface {
			if r.applyFieldWriteExpr(site.file, fieldFnKey{Ent: owner, Field: field}, init, site.caller, site.node) {
				changed = true
			}
			continue
		}
		nested := r.nestedFnOwner(fieldFnKey{Ent: owner, Field: field})
		if t.Kind(init) == syntax.RecordLit {
			if r.writeLiteralFields(site, nested, init, ft, depth+1, nil) {
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
	for entry := range t.View(lit).AsRecordLit().Fields().IDs() {
		if t.Kind(entry) != syntax.Spread && r.Files[file].Uses[entry] == field {
			return t.View(entry).AsFieldInit().Value().ID()
		}
	}
	return 0
}

func stripParens(t *syntax.Tree, n syntax.NodeID) syntax.NodeID {
	return t.Unparen(n)
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
