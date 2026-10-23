package sem

import (
	"kigumi/internal/syntax"
	"kigumi/internal/token"
)

// killSite is a top-level statement that replaces what a local's place (top is a field of it, 0 for the whole local) held,
// so a use after the statement no longer sees the fns written before it.
type killSite struct {
	top  EntityID
	stmt token.Span
}

func (c *checker) topLevelStmt(stmt syntax.NodeID) bool {
	if c.fn == 0 || len(c.lambdas) > 0 || c.r.Entities[c.fn].Kind != EntFn {
		return false
	}
	return c.isBodyStmt(stmt)
}

type bodyStmtSet struct {
	fn    EntityID
	stmts map[syntax.NodeID]struct{}
}

func (c *checker) isBodyStmt(stmt syntax.NodeID) bool {
	body := c.r.Fn(c.fn).Body
	if body == 0 {
		return false
	}
	if c.r.flow.bodyStmts.fn != c.fn || c.r.flow.bodyStmts.stmts == nil {
		ids := c.t.List(body).AppendIDs(nil)
		c.r.flow.bodyStmts = bodyStmtSet{fn: c.fn, stmts: make(map[syntax.NodeID]struct{}, len(ids))}
		for _, id := range ids {
			c.r.flow.bodyStmts.stmts[id] = struct{}{}
		}
	}
	_, ok := c.r.flow.bodyStmts.stmts[stmt]
	return ok
}

func (c *checker) killableLocal(ent EntityID) bool {
	e := &c.r.Entities[ent]
	return c.r.canonEnt(ent) == ent && e.Kind == EntLocal && e.Parent == c.fn && e.Flags&EfMut != 0
}

func (c *checker) addKill(root, top EntityID, stmt syntax.NodeID) {
	if c.r.flow.kills == nil {
		c.r.flow.kills = map[EntityID][]killSite{}
	}
	c.r.flow.kills[root] = append(c.r.flow.kills[root], killSite{top: top, stmt: c.t.Span(stmt)})
}

func (c *checker) noteAssignKill(stmt, lhs, rhs syntax.NodeID) {
	a := c.t.View(stmt).AsAssign()
	if a.Op().Kind().IsCompoundAssign() || !c.topLevelStmt(stmt) {
		return
	}
	p, ok := c.buildPlace(lhs)
	if !ok || p.Deref || !c.killableLocal(p.Root) || c.mentions(rhs, c.r.Entities[p.Root].Name) {
		return
	}
	switch {
	case p.Index && len(p.Fields) == 0:
		c.noteElementOverwrite(stmt, lhs, rhs, p.Root)
	case !p.Index && len(p.Fields) == 0:
		c.addKill(p.Root, 0, stmt)
	case !p.Index && len(p.Fields) == 1:
		c.addKill(p.Root, p.Fields[0], stmt)
	}
}

// Only `Array.of(x)` held in a local no statement has named since it was bound is known to hold x at index 0.
func (c *checker) noteSoleElement(stmt, init syntax.NodeID, local EntityID) {
	elems, ok := c.r.containerLitElems(c.f, init)
	if t := c.r.Types.Node(c.vars.resolve(c.r.Entities[local].Type)); !ok || len(elems) != 1 || t.Kind != KNamed || t.Ent != c.r.Types.arrayEnt || c.r.Types.arrayEnt == 0 || !c.topLevelStmt(stmt) {
		return
	}
	if c.r.flow.soleElem == nil {
		c.r.flow.soleElem = map[EntityID]token.Span{}
	}
	c.r.flow.soleElem[local] = c.t.Span(stmt)
}

func (c *checker) untouchedSince(local EntityID, until syntax.NodeID) bool {
	decl, ok := c.r.flow.soleElem[local]
	if !ok || decl.End > c.t.Span(until).Start {
		return false
	}
	name := c.r.Entities[local].Name
	for _, s := range c.t.List(c.r.Fn(c.fn).Body).AppendIDs(nil) {
		if sp := c.t.Span(s); sp.Start >= decl.End && sp.End <= c.t.Span(until).Start && c.mentions(s, name) {
			return false
		}
	}
	return true
}

func (c *checker) mentions(n syntax.NodeID, name string) bool {
	if c.t.Kind(n) == syntax.Ident && c.t.View(n).Tok().Text() == name {
		return true
	}
	for ch := range c.t.View(n).Children() {
		if c.mentions(ch.ID(), name) {
			return true
		}
	}
	return false
}

func (c *checker) zeroIndex(n syntax.NodeID) bool {
	return c.t.Kind(n) == syntax.IntLit && c.t.View(n).Tok().Text() == "0"
}

func (c *checker) noteElementOverwrite(stmt, lhs, rhs syntax.NodeID, root EntityID) {
	args := c.t.View(c.t.Unparen(lhs)).AsBracket().Args()
	if args.Len() != 1 || !c.zeroIndex(args.ItemID(0)) || !c.untouchedSince(root, stmt) {
		return
	}
	top := c.r.containerField(fieldElement)
	if _, elems, ok := c.r.containerShape(c.vars.resolve(c.r.Entities[root].Type)); ok && c.r.holdsFnPlace(elems[0]) {
		c.flowComponent(c.r.nestedFnOwner(fieldFnKey{Ent: root, Field: top}), elems[0], rhs)
	}
	c.addKill(root, top, stmt)
}

// fnKilledAt reports that every write of f into key comes before a statement that replaced the place and the use follows it.
func (r *Result) fnKilledAt(key fieldFnKey, f EntityID, file FileID, use syntax.NodeID) bool {
	root := r.rootOf(key.Ent)
	kills := r.flow.kills[root]
	sites := r.flow.sites[fieldSiteKey{Key: key, Fn: f}]
	if len(kills) == 0 || len(sites) == 0 || r.Entities[root].Kind != EntLocal {
		return false
	}
	top := key.Field
	if key.Ent != root {
		var ok bool
		if top, ok = r.nestedTopField(key.Ent, root); !ok {
			return false
		}
	}
	fn := &r.Entities[r.Entities[root].Parent]
	if fn.Kind != EntFn || fn.File != file {
		return false
	}
	scan, tree := r.flowScanOf(file, fn.Node), r.Files[file]
	useSpan, fnSpan := tree.span(use), tree.span(fn.Node)
	for _, w := range sites {
		if w.File != file || !within(fnSpan, tree.span(w.Node)) || !r.killedWrite(kills, top, tree.span(w.Node), useSpan, scan) {
			return false
		}
	}
	return true
}

func (r *Result) killedWrite(kills []killSite, top EntityID, w, use token.Span, scan *flowScan) bool {
	for _, lam := range scan.lambdas {
		if within(lam, w) {
			return false
		}
	}
	for _, k := range kills {
		if (k.top == 0 || k.top == top) && w.Start < k.stmt.Start && k.stmt.End <= use.Start {
			return true
		}
	}
	return false
}
