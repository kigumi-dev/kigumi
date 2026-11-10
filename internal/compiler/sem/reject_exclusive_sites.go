package sem

import (
	"slices"

	"kigumi/internal/compiler/syntax"
)

type exclusiveSite struct {
	base        syntax.NodeID
	args        []syntax.NodeID
	root, field EntityID
	roots       []EntityID
	lent        map[EntityID]EntityID
	// fnArgs: the strict fn-shaped argument rule applies to `mut self` calls only.
	fnArgs bool
}

type outsideMutCall struct {
	base        syntax.NodeID
	root, field EntityID
	roots       []EntityID
	lent        map[EntityID]EntityID
	// byRef: the access is a `&mut` argument, not a `mut self` receiver.
	byRef bool
	// async: the call's future holds the receiver until it completes.
	async bool
	// enclosing is the innermost closure whose body contains base, 0 at declaration scope.
	enclosing EntityID
	// args of a `mut self` call run while it holds the receiver.
	args []syntax.NodeID
}

// mutWriteSite is a write of a captured local: the node that writes, the closure whose body holds it (0 when the write
// sits at declaration scope), and whether the write is itself a `mut self` call on the shared cell.
type mutWriteSite struct {
	node     syntax.NodeID
	closure  EntityID
	selfCall bool
	// method is the callee of a `mut self` call site.
	method EntityID
}

func (c *checker) noteMutCapture(n syntax.NodeID, root EntityID, selfCall bool) {
	if c.mutCaptures == nil {
		c.mutCaptures = map[EntityID][]mutWriteSite{}
	}
	c.mutCaptures[root] = append(c.mutCaptures[root], mutWriteSite{node: n, closure: c.enclosingClosure(), selfCall: selfCall})
}

// enclosingClosure names the innermost lambda being checked, or 0 at declaration scope.
func (c *checker) enclosingClosure() EntityID {
	if len(c.lambdas) == 0 {
		return 0
	}
	return c.lambdas[len(c.lambdas)-1].ent
}

func (c *checker) rootsHeldByClosure(roots []EntityID) bool {
	for _, root := range roots {
		for _, h := range c.holders(root) {
			if h.mode != capCopy {
				return true
			}
		}
	}
	for local := range c.captureSites {
		if c.borrowsRoot(local, &heldResult{roots: roots}) {
			return true
		}
	}
	return false
}

func (c *checker) sharesLoop(a, b syntax.NodeID) bool {
	pa, pb := c.t.Span(a).Start, c.t.Span(b).Start
	for _, l := range c.loopSpans {
		if l.Start <= pa && pa < l.End && l.Start <= pb && pb < l.End {
			return true
		}
	}
	return false
}

func (c *checker) settleExclusive() {
	c.recheckErasures()
	c.exclusiveArgs = c.exclusiveArgExprs()
	for _, call := range c.outsideCalls {
		if !call.byRef && !call.async {
			// ReplacesSelf is only final once every body is checked.
			c.r.ctxDeferred = append(c.r.ctxDeferred, func() { c.settleOutsideCall(call) })
			continue
		}
		c.settleOrDefer(call.roots, func() { c.settleOutsideCall(call) })
	}
	for _, s := range c.exclusiveSites {
		c.settleOrDefer(s.roots, func() { c.settleExclusiveSite(s) })
	}
	// exclusiveArgs stays: a deferred settle (context fixpoint) can still run after this.
	c.outsideCalls, c.exclusiveSites = nil, nil
}

// exclusiveArgExprs lists every argument of an exclusive call once, unwrapped so a span test sees the value itself.
func (c *checker) exclusiveArgExprs() []syntax.NodeID {
	var out []syntax.NodeID
	for _, s := range c.exclusiveSites {
		for _, a := range s.args {
			if c.t.Kind(a) == syntax.NamedArg {
				a = c.t.View(a).AsNamedArg().Value().ID()
			}
			out = append(out, c.t.Unparen(a))
		}
	}
	return out
}

func (c *checker) settleOrDefer(roots []EntityID, settle func()) {
	if c.deferredToContext(roots) {
		c.r.ctxDeferred = append(c.r.ctxDeferred, settle)
		return
	}
	settle()
}

func (c *checker) noteMutBorrowSites(args []syntax.NodeID, declParams []EntityID) {
	for i, a := range args {
		others := slices.Delete(slices.Clone(args), i, i+1)
		if root, field := c.mutBorrowRoot(a); root == 0 {
			c.noteHeldArgSite(a, others)
		} else {
			roots, lent := c.exclusiveRoots(root, field)
			c.exclusiveSites = append(c.exclusiveSites, exclusiveSite{base: a, args: others, root: root, field: field, roots: roots, lent: lent})
			if !c.capturedHere(root) {
				c.outsideCalls = append(c.outsideCalls, outsideMutCall{base: a, root: root, field: field, roots: roots, lent: lent, byRef: true})
			}
		}
		c.noteArgStores(args, i, declParams)
	}
}

// A by-value argument that can carry a `&mut` lends that borrow to the call while it runs.
func (c *checker) noteHeldArgSite(a syntax.NodeID, others []syntax.NodeID) {
	expr := c.t.Unparen(a)
	if t := c.vars.resolve(c.info.Types[expr]); c.r.Types.Kind(t) == KRef && !c.r.Types.RefMut(t) || !c.holdsLentRef(t) {
		return
	}
	var start []lentRef
	for _, from := range c.leafSources(expr, 0, true, 0) {
		if from.root != 0 && (c.r.Entities[from.root].Kind == EntLocal || c.r.Entities[from.root].Kind == EntParam) {
			start = append(start, from)
		}
	}
	if len(start) == 0 {
		return
	}
	roots, lent := c.rootsFrom(start)
	c.exclusiveSites = append(c.exclusiveSites, exclusiveSite{base: a, args: others, root: start[0].root, field: start[0].field, roots: roots, lent: lent})
}

func (c *checker) mutBorrowRoot(a syntax.NodeID) (EntityID, EntityID) {
	a = c.t.Unparen(a)
	switch c.t.Kind(a) {
	case syntax.BorrowExpr:
		if c.t.View(a).AsBorrow().Flags()&syntax.FlagMut != 0 {
			return c.placeRoot(c.t.View(a).AsBorrow().Operand().ID())
		}
	case syntax.Ident:
		tt := c.r.Types
		if t := c.vars.resolve(c.info.Types[a]); tt.Kind(t) == KRef && tt.Node(t).Flags&flagMut != 0 {
			return c.info.Uses[a], 0
		}
	}
	return 0, 0
}

func (c *checker) noteLoopAlias(pat, head syntax.NodeID) {
	head = c.t.Unparen(head)
	if c.t.Kind(pat) != syntax.PatBind || c.t.Kind(head) != syntax.BorrowExpr || c.t.View(head).AsBorrow().Flags()&syntax.FlagMut == 0 {
		c.notePartBindings(pat, head)
		return
	}
	if root := c.rootOf(c.t.View(head).AsBorrow().Operand().ID()); root != 0 {
		c.setLentFrom(c.info.Defs[pat], lentRef{root: root})
	}
}

func (c *checker) setLentFrom(ent EntityID, from lentRef) {
	if c.lentFrom == nil {
		c.lentFrom = map[EntityID]lentRef{}
	}
	c.lentFrom[ent] = from
}
