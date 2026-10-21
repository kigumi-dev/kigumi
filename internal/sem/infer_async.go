package sem

import "kigumi/internal/syntax"

// taskEntity is std/task's `Task`, the second operand `await` accepts.
func (r *Result) taskEntity() EntityID {
	pkg, ok := r.pathIndex["std/task"]
	if !ok {
		return 0
	}
	return r.Scopes[r.Packages[pkg].Scope].Names["Task"].Ent
}

func (c *checker) synthAwait(n syntax.NodeID) TypeID {
	c.dropUnstableFacts()
	op := c.t.View(n).AsAwait().Operand().ID()
	c.r.markTaskFuture(c.f, op, futureAwaited)
	t := c.vars.resolve(c.synth(op))
	if e := c.r.Entities[c.fn]; e.Kind != EntFn || c.r.Fn(c.fn).Declared&EffAsync == 0 {
		c.errAt(n, cAwaitOutsideAsync)
	}
	if c.cleanup > 0 {
		c.errAt(n, cCleanupAwait)
	}
	c.edge(EffectEdge{Kind: EdgeSuspend, Node: n})
	if t == TyPoison {
		return TyPoison
	}
	if node := c.r.Types.Node(t); node.Kind == KNamed && len(node.Args) == 1 &&
		(node.Ent == c.r.Types.futureEnt || node.Ent == c.r.taskEntity()) {
		return node.Args[0]
	}
	c.errAt(n, cAwaitNotFuture, c.r.TypeString(t))
	return TyPoison
}

// Matches the lazy Future a direct call to the async fn builds.
func (c *checker) asyncValueFn(sig TypeID) TypeID {
	tt := c.r.Types
	n := tt.Node(sig)
	ret := tt.Named(tt.futureEnt, []TypeID{n.Elem})
	return tt.Fn(n.Args, ret, Effects(n.Flags), n.Flags&fnVariadic != 0)
}
