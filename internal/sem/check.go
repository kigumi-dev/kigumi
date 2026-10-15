package sem

import (
	"kigumi/internal/syntax"
)

type checker struct {
	r          *Result
	f          FileID
	t          *syntax.Tree
	info       *FileInfo
	pkg        PackageID
	fn         EntityID
	retType    TypeID
	scope      ScopeID
	loop       int
	brokeOut   bool
	breaks     *loopBreaks
	allocDepth int
	stmtPos    syntax.NodeID
	// wantAliasTarget/File/Node name the declared type-annotation node
	// for a mismatch reported on wantAliasTarget; the target check
	// keeps a stale hint from a still-open outer check() leaking into an
	// unrelated mismatch that a nested check() reports first.
	wantAliasTarget syntax.NodeID
	wantAliasFile   FileID
	wantAliasNode   syntax.NodeID
	cleanup         int
	edges           *[]EffectEdge
	// locals is the current edge scope's EntLocal list for dropsAtScopeExit.
	locals []EntityID
	// consumed marks a binding moved out before scope exit, skipped by dropsAtScopeExit.
	consumed map[EntityID]bool
	// branch counts nested if/match/loop bodies, where a consuming use doesn't mark consumed.
	branch        int
	vars          *varStore
	touched       []syntax.NodeID
	allocPending  []pendingAlloc
	protoSites    []protoSite
	inlinePending []pendingInline
	// piped maps a call node to the `|>` left operand it takes first.
	piped map[syntax.NodeID]syntax.NodeID
	// argLambda marks a lambda written directly as a call argument; it may capture borrows, the callee not expected to keep them.
	argLambda map[syntax.NodeID]bool
	// argPlace marks a `&mut` written directly as a call argument: only there
	// does build_call_fixup.go give a scalar index/field a base to write back to.
	argPlace map[syntax.NodeID]bool
	// argAsyncPlace is argPlace for an async call, where build_call_fixup.go
	// never boxes the place since the callee runs later, at await.
	argAsyncPlace map[syntax.NodeID]bool
	// patBorrow is set while checking a pattern whose scrutinee is borrowed:
	// bindings then borrow their payloads instead of moving them.
	patBorrow bool
	// patBorrowMut is patBorrow's mutability, meaningful only when patBorrow
	// is set (a `for` head can borrow mutably, unlike match).
	patBorrowMut bool
	contracts    []Effects
	lambdas      []*lambdaCtx
	retLeaves    []syntax.NodeID
	// captureSites indexes every Capture entry recorded for a local, across
	// closures open and closed, so a write seen after a closure finishes can
	// still promote that closure's entry.
	captureSites map[EntityID][]captureSite
	facts        []NarrowFact
	inCond       int
	viaCopy      bool
	unsafe       int
	naked        bool
	closures     []EntityID
	constraints  []constraintCheck
	witnessTasks []witnessTask
	ffiValues    []ffiValueCheck
	unsatIface   map[unsatKey]bool
	// paramAlias maps an immutable local bound to a fn-typed parameter
	// (`let g = f`) back to that parameter for callback polymorphism.
	paramAlias map[EntityID]EntityID
	// letInit: a local's `let` initializer, so an fn value can be traced through alias chains.
	letInit map[EntityID]syntax.NodeID
	// paramReassign: the values a param is wholly reassigned to, which a returned param may hold instead of its argument.
	paramReassign map[EntityID][]syntax.NodeID
	// escapeAlias: a non-escaping parameter a local may hold; any binding or branch taints it and `mut` does not clear it.
	escapeAlias map[EntityID]EntityID
	// taintedCall: callValue nodes whose result may hold a non-escaping parameter (opaque callee).
	taintedCall map[syntax.NodeID]EntityID
	// fieldFact stashes a resolved `pure?` field-call's FactPureField
	// between callMethod finding it and callValue adding
	// the matching effect edge.
	fieldFact map[syntax.NodeID]NarrowFact
	// wantIndexPlace/indexAsPlace: see synthPlace.
	wantIndexPlace bool
	indexAsPlace   map[syntax.NodeID]bool
	// ctxStack is the lexical stack of `context (...) { }` bindings open at
	// the current point; the innermost frame with a key wins.
	ctxStack []ctxFrame
	// pendingFieldEdges are field writes' root-bridge edges, held back for
	// flushFieldWrites; see pendingFieldEdge.
	pendingFieldEdges []pendingFieldEdge
}

// checkBodies (pass 7) type-checks every function, test and implicit main
// in entity order, which is package, file and declaration order.
func (r *Result) checkBodies() {
	for _, id := range r.implicitMain {
		r.checkImplicitMain(id)
	}
	for id := 1; id < len(r.Entities); id++ {
		e := &r.Entities[id]
		if e.File == 0 || e.Flags&EfPoison != 0 {
			continue
		}
		if r.Cancelled() {
			return
		}
		switch e.Kind {
		case EntFn:
			info := r.Fn(EntityID(id))
			if info.Body != 0 && r.Entities[e.Parent].Kind != EntInterface {
				r.checkFnBody(EntityID(id))
			}
		case EntTest:
			r.checkTestBody(EntityID(id))
		}
	}
	r.bodiesChecked = true
}

func (r *Result) newChecker(f FileID, fn EntityID, scope ScopeID, ret TypeID, edges *[]EffectEdge) *checker {
	return &checker{r: r, f: f, t: r.tree(f), info: &r.Files[f], pkg: r.packageOf(f), fn: fn, retType: ret, paramAlias: map[EntityID]EntityID{},
		escapeAlias: map[EntityID]EntityID{}, taintedCall: map[syntax.NodeID]EntityID{},
		captureSites: map[EntityID][]captureSite{}, scope: scope, edges: edges, vars: newVarStore(r)}
}

func (r *Result) checkFnBody(id EntityID) {
	e := &r.Entities[id]
	info := r.Fn(id)
	var edges []EffectEdge
	c := r.newChecker(e.File, id, r.declScope(id), r.Types.Node(info.Sig).Elem, &edges)
	r.Bodies = append(r.Bodies, BodyRef{Fn: id, File: e.File, Node: info.Body})
	if info.Naked {
		c.naked = true
		if stmts := c.t.List(info.Body); stmts.Len() != 1 || stmts.At(0).Kind() != syntax.ExprStmt || stmts.At(0).AsExprStmt().Expr().Kind() != syntax.AsmExpr {
			r.errAt(e.File, info.Body, cNakedBody)
		}
	}
	c.checkBodyBlock(info.Body)
	c.registerContainerFlows(info.Body)
	c.recordResultLeaves(info.Body)
	c.settleInlineAt(true)
	c.flushFieldWrites()
	if r.DirectResultType(id, c.retType) == 0 {
		c.crossDirect(info.Body, c.retType)
	}
	c.dropsAtScopeExit(info.Params)
	r.Fn(id).Edges = edges
}

// checkBodyBlock checks a function body against the return type: the block's
// tail value must coerce to it, and a missing tail needs Unit or Never.
func (c *checker) checkBodyBlock(body syntax.NodeID) {
	got := c.check(body, c.retType)
	_ = got
	c.checkEscapeTailReturn(body)
	c.finish()
}

func (c *checker) pushScope(kind ScopeKind, node syntax.NodeID) ScopeID {
	s := c.r.newScope(kind, c.scope, c.f, node)
	c.r.Scopes[s].Fn = c.fn
	c.info.Scopes[node] = s
	c.scope = s
	return s
}

func (c *checker) popScope(saved ScopeID) { c.scope = saved }

func (c *checker) setType(n syntax.NodeID, t TypeID) TypeID {
	c.info.Types[n] = t
	c.touched = append(c.touched, n)
	return t
}
