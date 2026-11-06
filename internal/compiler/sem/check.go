package sem

import (
	"kigumi/internal/compiler/syntax"
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
	// wantAliasTarget/File/Node name the declared type-annotation node for a mismatch reported on
	// wantAliasTarget; the target check keeps a stale hint from an open outer check() out of a nested mismatch.
	wantAliasTarget syntax.NodeID
	wantAliasFile   FileID
	wantAliasNode   syntax.NodeID
	cleanup         int
	cleanupNode     syntax.NodeID
	cleanupLog      cleanupLog
	reserved        map[EntityID][]reservation
	held            heldMuts
	exitCheck       exitMoveState
	edges           *[]EffectEdge
	// locals is the current edge scope's EntLocal list for dropsAtScopeExit.
	locals []EntityID
	// consumed marks a binding or a part of one moved out before scope exit, skipped by dropsAtScopeExit.
	consumed consumeLog
	// branch counts enclosing code that may not run (arms, guards, short-circuit operands), where a consuming use doesn't mark consumed.
	branch        int
	vars          *varStore
	touched       []syntax.NodeID
	allocPending  []pendingAlloc
	protoSites    []protoSite
	inlinePending []pendingInline
	deflt         defaultFrame
	// piped maps a call node to the `|>` left operand it takes first, and a `name: _` argument to the one standing for it.
	piped map[syntax.NodeID]syntax.NodeID
	// argLambda marks a lambda written directly as a call argument; it may capture borrows, the callee not expected to keep them.
	argLambda map[syntax.NodeID]bool
	// argPlace marks a `&mut` written directly as a call argument, the only place build_call_fixup.go writes a scalar index/field back.
	argPlace map[syntax.NodeID]bool
	// argAsyncPlace is argPlace for an async call, whose place is never boxed since the callee runs at await.
	argAsyncPlace map[syntax.NodeID]bool
	// patBorrow is set while checking a pattern whose scrutinee is borrowed:
	// bindings then borrow their payloads instead of moving them.
	patBorrow bool
	// patBorrowMut is patBorrow's mutability, meaningful only when patBorrow
	// is set (a `for` head can borrow mutably, unlike match).
	patBorrowMut bool
	patMoved     patMoveLog
	contracts    []Effects
	lambdas      []*lambdaCtx
	exits        exitLeaves
	// captureSites indexes every Capture entry of a local across open and closed closures, so a write seen after a
	// closure finishes can still promote that closure's entry.
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
	atomicPlace  map[syntax.NodeID]bool
	unsatIface   map[unsatKey]bool
	// paramAlias maps an immutable local bound to a fn-typed parameter (`let g = f`) back to it for callback polymorphism.
	paramAlias map[EntityID]EntityID
	// letInit: a local's `let` initializer, so an fn value can be traced through alias chains.
	letInit map[EntityID]syntax.NodeID
	// sliceViews maps a local that a `let` bound to a slice by reference to that slice.
	sliceViews map[EntityID]syntax.NodeID
	// valueExprs maps a method value expression to its own-generic instantiation, for the proof at calls through it.
	valueExprs map[syntax.NodeID]*valueInstData
	// holderVals, holderAlias, holderOther record, per local, the method values it is bound to, the locals it aliases,
	// and whether it is ever assigned a non-method value, resolved flow-insensitively for the holder escape walk.
	holderVals      map[EntityID][]*valueInstData
	holderAlias     map[EntityID][]EntityID
	holderOther     map[EntityID]bool
	holderAliasNode map[syntax.NodeID]bool
	holderRes       map[EntityID]*holderRes
	// callArgs and walkOwner feed the holder walk: a value call's placed arguments and the closure it walks inside.
	callArgs  map[syntax.NodeID][]syntax.NodeID
	walkOwner EntityID
	// valueIfaceChecks defers a value instantiation's interface-bound refusal to the end of the body, so a later call
	// through the value counts.
	valueIfaceChecks []valueIfaceCheck
	// paramReassign: the values a param is wholly reassigned to, which a returned param may hold instead of its argument.
	paramReassign map[EntityID][]syntax.NodeID
	// escapeAlias: a non-escaping parameter a local may hold; any binding or branch taints it and `mut` does not clear it.
	escapeAlias map[EntityID]EntityID
	// taintedCall: callValue nodes whose result may hold a non-escaping parameter (opaque callee).
	taintedCall map[syntax.NodeID]EntityID
	once        onceFlow
	// fieldFact stashes a resolved `pure?` field-call's FactPureField between callMethod and callValue adding its edge.
	fieldFact      map[syntax.NodeID]NarrowFact
	methodValues   methodValueState
	wantIndexPlace bool
	indexAsPlace   map[syntax.NodeID]bool
	// ctxStack is the lexical stack of `context (...) { }` bindings open at
	// the current point; the innermost frame with a key wins.
	ctxStack []ctxFrame
	// pendingFieldEdges are field writes' root-bridge edges, held back for
	// flushFieldWrites; see pendingFieldEdge.
	pendingFieldEdges []pendingFieldEdge
	exclusiveSites    []exclusiveSite
	outsideCalls      []outsideMutCall
	mutCaptures       map[EntityID][]mutWriteSite
	// exclusiveArgs are the argument expressions of every exclusive call, whose closures cannot outlive the call.
	exclusiveArgs []syntax.NodeID
	exprState
	// lentFrom maps a `for` or pattern binding over a `&mut` head to the head's root place.
	lentFrom map[EntityID]lentRef
	// mutAliasInit: the initializer of a `let mut` borrow, which cannot be rebound to another place.
	mutAliasInit map[EntityID]syntax.NodeID
}

// checkBodies (pass 7) type-checks every function, test and implicit main
// in entity order, which is package, file and declaration order.
func (r *Result) checkBodies() {
	for _, id := range r.implicitMain {
		r.checkImplicitMain(id)
	}
	for id := 1; id < len(r.Entities); id++ {
		e := &r.Entities[id]
		if e.File == 0 || e.Flags&EfPoison != 0 || r.BodySkipped(EntityID(id)) {
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
	c.checkDefaultVisibility(id)
	c.settleExclusive()
	c.registerContainerFlows(info.Body)
	c.recordResultLeaves(info.Body)
	c.settleInlineAt(true)
	c.flushFieldWrites()
	if r.DirectResultType(id, c.retType) == 0 {
		c.crossDirect(info.Body, c.retType)
	}
	c.dropsAtScopeExit(info.Params)
	c.selfExitDrop(id)
	r.Fn(id).Edges = edges
}

// checkBodyBlock checks a function body against the return type: the block's
// tail value must coerce to it, and a missing tail needs Void or Never.
func (c *checker) checkBodyBlock(body syntax.NodeID) {
	c.exitCheck.tail = c.bodyTail(body)
	c.check(body, c.retType)
	c.rejectResultLeaves(body)
	c.checkExitMoves()
	c.checkEscapeTailReturn(body)
	c.finish()
	c.escapeHolderUses(body)
	c.flushValueIfaceChecks()
}
